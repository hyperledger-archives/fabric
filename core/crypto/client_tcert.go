/*
Licensed to the Apache Software Foundation (ASF) under one
or more contributor license agreements.  See the NOTICE file
distributed with this work for additional information
regarding copyright ownership.  The ASF licenses this file
to you under the Apache License, Version 2.0 (the
"License"); you may not use this file except in compliance
with the License.  You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing,
software distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
KIND, either express or implied.  See the License for the
specific language governing permissions and limitations
under the License.
*/

package crypto

import (
	"crypto/x509"
	"errors"
	"bytes"
	"strings"
	"strconv"
	"encoding/asn1"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"github.com/hyperledger/fabric/core/crypto/utils"
	"github.com/hyperledger/fabric/core/crypto/conf"
)

var (
	Padding = []byte{255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255}
)

type tCert interface {
	GetCertificate() *x509.Certificate
	
	GetKForAttribute(attributeName string) ([]byte, error)  
	
	GetPreK0() ([]byte)
	
	Sign(msg []byte) ([]byte, error)

	Verify(signature, msg []byte) error

}

type tCertImpl struct {
	client *clientImpl
	cert   *x509.Certificate
	sk     interface{}
	preK0 []byte
	
}

func (tCert *tCertImpl) GetCertificate() *x509.Certificate {
	return tCert.cert
}

func (tCert *tCertImpl) GetPreK0() []byte {
	return tCert.preK0
}


func (tCert *tCertImpl) Sign(msg []byte) ([]byte, error) {
	if tCert.sk == nil {
		return nil, utils.ErrNilArgument
	}

	return tCert.client.sign(tCert.sk, msg)
}

func (tCert *tCertImpl) Verify(signature, msg []byte) (err error) {
	ok, err := tCert.client.verify(tCert.cert.PublicKey, msg, signature)
	if err != nil {
		return
	}
	if !ok {
		return utils.ErrInvalidSignature
	}
	return
}

func (tCert *tCertImpl) GetKForAttribute(attributeName string) ([]byte, error) {
	if tCert.preK0 == nil {
		return nil, utils.ErrNilArgument
	}
	
	mac := hmac.New(conf.GetDefaultHash(), tCert.preK0)
	mac.Write([]byte(attributeName))
	attributeKey := mac.Sum(nil)[:32]
	
	attribute, err := tCert.readTCertAttribute(attributeName)
	if err != nil {
		return nil, err
	}
	
	value, err := tCert.cbcDecrypt(attributeKey, attribute)
	if err != nil {
		return nil, err
	}
	
	lenPadding := len(Padding)
	lenValue := len(value)
	if bytes.Compare(Padding[0:lenPadding], value[lenValue - lenPadding:lenValue]) == 0 {
		return attributeKey, nil
	}

	return nil, errors.New("Error generating decryption key for attribute "+ attributeName + ". Decryption verification failed.")
}

func (tCert *tCertImpl) parseAttributesHeader(header string) (map[string]int, error) { 
	tokens :=  strings.Split(header, "#")
	result := make(map[string]int)
	
	for _, token := range tokens {
		pair:= strings.Split(token, "->")
		
		if len(pair) == 2 {
			key := pair[0]
			valueStr := pair[1]
			value, err := strconv.Atoi(valueStr)
			if err != nil { 
				return nil, err
			}
			result[key] = value
		}
	}
	
	return result, nil
}

func (tCert *tCertImpl) readTCertAttribute(attributeName string) ([]byte, error) {
	var err error
	var header_raw []byte
	if header_raw, err = utils.GetCriticalExtension(tCert.GetCertificate(), utils.TCertAttributesHeaders); err != nil {
		return nil, err
	}

	header_str := string(header_raw)	
	var header map[string]int
	header, err = tCert.parseAttributesHeader(header_str)
	
	if err != nil {
		return nil, err
	}
	
	position := header[attributeName]
	
	if position == 0 {
		return nil, errors.New("Failed attribute doesn't exists in the TCert.")
	}

    oid := asn1.ObjectIdentifier{1, 2, 3, 4, 5, 6, 9 + position}
    
    var value []byte
    if value, err = utils.GetCriticalExtension(tCert.GetCertificate(), oid); err != nil {
		return nil, err
	}
    return value, nil
}

func (tCert *tCertImpl) cbcDecrypt(key, src []byte) ([]byte, error) {
	blk, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	if len(src) < aes.BlockSize {
		return nil, errors.New("ciphertext too short")
	}
	iv := src[:aes.BlockSize]
	src = src[aes.BlockSize:]

	if len(src)%aes.BlockSize != 0 {
		return nil, errors.New("ciphertext length is not a multiple of the block size")
	}

	mode := cipher.NewCBCDecrypter(blk, iv)
	mode.CryptBlocks(src, src)

	return tCert.pkcs5Unpad(src), nil
}

func (tCert *tCertImpl) pkcs5Unpad(src []byte) []byte {
	len := len(src)
	unpad := int(src[len-1])
	return src[:(len - unpad)]
}
