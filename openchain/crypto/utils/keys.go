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

package utils

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
)

// PrivateKeyToDER marshals a private key to der
func PrivateKeyToDER(privateKey *ecdsa.PrivateKey) ([]byte, error) {
	return x509.MarshalECPrivateKey(privateKey)
}

func PrivateKeyToPEM(privateKey interface{}) ([]byte, error) {
	switch x := privateKey.(type) {
	case *ecdsa.PrivateKey:
		raw, err := x509.MarshalECPrivateKey(x)

		if err != nil {
			return nil, err
		}

		return pem.EncodeToMemory(
			&pem.Block{
				Type:  "ECDSA PRIVATE KEY",
				Bytes: raw,
			},
		), nil
	default:
		return nil, nil
	}
}

func PrivateKeyToEncryptedPEM(privateKey interface{}, pwd []byte) ([]byte, error) {
	switch x := privateKey.(type) {
	case *ecdsa.PrivateKey:
		raw, err := x509.MarshalECPrivateKey(x)

		if err != nil {
			return nil, err
		}

		block, err := x509.EncryptPEMBlock(
			rand.Reader,
			"ECDSA PRIVATE KEY",
			raw,
			pwd,
			x509.PEMCipherAES256)

		if err != nil {
			return nil, err
		}

		return pem.EncodeToMemory(block), nil

	default:
		return nil, nil
	}
}

// DERToPrivateKey unmarshals a der to private key
func DERToPrivateKey(der []byte) (interface{}, error) {
	// Try RSA and then ECDSA
	rsakey, err1 := x509.ParsePKCS1PrivateKey(der)
	if err1 == nil {
		return rsakey, nil
	}
	//	else {
	//		fmt.Println("RSA failed" + err1.Error())
	//	}

	ecdsakey, err2 := x509.ParseECPrivateKey(der)
	if err2 == nil {
		return ecdsakey, nil
	}
	//else {
	//		fmt.Println("EC failed" + err2.Error())
	//	}

	return nil, errors.New("Key not recognized.")
}

// PEMtoPrivateKey unmarshals a pem to private key
func PEMtoPrivateKey(raw []byte, pwd []byte) (interface{}, error) {
	block, _ := pem.Decode(raw)

	if x509.IsEncryptedPEMBlock(block) {
		if pwd == nil {
			return nil, errors.New("Encrypted Key. Need a password!!!")
		}

		decrypted, err := x509.DecryptPEMBlock(block, pwd)
		if err != nil {
			return nil, errors.New("Failed decryption!!!")
		}

		key, err := DERToPrivateKey(decrypted)
		if err != nil {
			return nil, err
		}
		return key, err
	}

	cert, err := DERToPrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	return cert, err
}

func PEMtoAES(raw []byte, pwd []byte) ([]byte, error) {
	block, _ := pem.Decode(raw)

	if x509.IsEncryptedPEMBlock(block) {
		if pwd == nil {
			return nil, errors.New("Encrypted Key. Need a password!!!")
		}

		decrypted, err := x509.DecryptPEMBlock(block, pwd)
		if err != nil {
			return nil, err
		}
		return decrypted, nil
	}

	return block.Bytes, nil
}

func AEStoPEM(raw []byte) []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "AES PRIVATE KEY", Bytes: raw})
}

func AEStoEncryptedPEM(raw []byte, pwd []byte) ([]byte, error) {
	block, err := x509.EncryptPEMBlock(
		rand.Reader,
		"AES PRIVATE KEY",
		raw,
		pwd,
		x509.PEMCipherAES256)

	if err != nil {
		return nil, err
	}

	return pem.EncodeToMemory(block), nil
}

/*
func PublicKeyToDER(publicKey interface{}) ([]byte, error) {
	return x509.MarshalPKIXPublicKey(publicKey)
}

func DERToPublicKey(derBytes []byte) (pub interface{}, err error) {
	key, err := x509.ParsePKIXPublicKey(derBytes)

	return key, err
}
*/

// PublicKeyToPEM marshals a public key to the pem forma
func PublicKeyToPEM(algo string, publicKey interface{}) ([]byte, error) {
	PubASN1, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return nil, err
	}

	return pem.EncodeToMemory(
		&pem.Block{
			Type:  algo + " PUBLIC KEY",
			Bytes: PubASN1,
		},
	), nil
}
