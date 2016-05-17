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

package abac

import (
	"crypto/x509"
	"encoding/asn1"
	"errors"
	"strings"
	"strconv"
	"bytes"
		
	"github.com/hyperledger/fabric/core/crypto/primitives"
	"github.com/hyperledger/fabric/core/crypto/utils"
	pb "github.com/hyperledger/fabric/core/crypto/abac/proto"

	"github.com/golang/protobuf/proto"
)

var (
	// TCertEncAttributesBase is the base ASN1 object identifier for attributes. 
	// When generating an extension to include the attribute an index will be 
	// appended to this Object Identifier.
	TCertEncAttributesBase = asn1.ObjectIdentifier{1, 2, 3, 4, 5, 6}

	// TCertAttributesHeaders is the ASN1 object identifier of attributes header.
	TCertAttributesHeaders = asn1.ObjectIdentifier{1, 2, 3, 4, 5, 6, 9}
	
	Padding = []byte{255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255}
	
	//Header 
	HeaderPrefix = "00HEAD"
	
	//Name used to derivate the k.
	HeaderAttributeName = "attributeHeader"
)

//Parse a string and return a map with the attributes.
func ParseAttributesHeader(header string) (map[string]int, error) { 
	if !strings.HasPrefix(header, HeaderPrefix) { 
		return nil, errors.New("Invalid header")
	} 
	headerBody := strings.Replace(header, HeaderPrefix, "", 1)
	tokens :=  strings.Split(headerBody, "#")
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

//Reads the attribute with name "attributeName" return the value and a boolean indicating if the returned value is encrypted or not.
func ReadTCertAttribute(tcert *x509.Certificate, attributeName string, headerKey []byte) ([]byte, bool, error) {
	var err error
	var header_raw []byte
	encrypted := false
	if header_raw, err = utils.GetCriticalExtension(tcert, TCertAttributesHeaders); err != nil {
		return nil, encrypted, err
	}
	header_str := string(header_raw)	
	var header map[string]int
	header, err = ParseAttributesHeader(header_str)
	if err != nil {
		if headerKey == nil { 
			return nil, false, errors.New("Is not possible read an attribute encrypted without the headerKey")
		}
		header_raw, err = DecryptAttributeValue(headerKey, header_raw)

		if err != nil { 
			return nil, encrypted, err		
		}
		header_str = string(header_raw)	
		header, err = ParseAttributesHeader(header_str)
		if err != nil { 
			return nil, encrypted, err		
		}
		encrypted = true
	}
	
	position := header[attributeName]
	if position == 0 {
		return nil, encrypted, errors.New("Failed attribute '"+attributeName+"' doesn't exists in the TCert.")
	}

    oid := asn1.ObjectIdentifier{1, 2, 3, 4, 5, 6, 9 + position}
    
    var value []byte
    if value, err = utils.GetCriticalExtension(tcert, oid); err != nil {
		return nil, encrypted, err
	}
    return value, encrypted, nil
}

//Encrypts "attributeValue" using "attributeKey"
func EncryptAttributeValue(attributeKey []byte, attributeValue []byte) ([]byte, error) { 
	value := append(attributeValue, Padding...)
	return primitives.CBCPKCS7Encrypt(attributeKey, value)
}

//Returns the attributeKey derived from the preK0 to the attributeName.
func getAttributeKey(preK0 []byte, attributeName string) []byte { 
	return primitives.HMACTruncated(preK0, []byte(attributeName), 32)
}

//Encrypts "attributeValue" using a key derived from preK0.
func EncryptAttributeValuePK0(preK0 []byte, attributeName string, attributeValue []byte) ([]byte, error) { 
	attributeKey := getAttributeKey(preK0, attributeName)
	return EncryptAttributeValue(attributeKey, attributeValue)
} 

//Decrypts "encryptedValue" using "attributeKey" and return the decrypted value.
func DecryptAttributeValue(attributeKey []byte, encryptedValue []byte) ([]byte, error) {
	value, err := primitives.CBCPKCS7Decrypt(attributeKey, encryptedValue)
	if err != nil {
		return nil, err
	}
	lenPadding := len(Padding)
	lenValue := len(value)
	if lenValue < lenPadding { 
		return nil, errors.New("Error invalid value. Decryption verification failed.")
	}
	lenWithoutPadding := lenValue - lenPadding
	if bytes.Compare(Padding[0:lenPadding], value[lenWithoutPadding:lenValue]) != 0 {
		return nil, errors.New("Error generating decryption key for value. Decryption verification failed.")
	}
	value = value[0:lenWithoutPadding]
	return value, nil
}

//Derives K for the attribute "attributeName", checks the value padding and returns both key and decrypted value
func getKAndValueForAttribute(attributeName string, preK0 []byte, cert *x509.Certificate) ([]byte, []byte, error) {
	headerKey := getAttributeKey(preK0, HeaderAttributeName)
	value, encrypted, err := ReadTCertAttribute(cert, attributeName, headerKey)
	if err != nil {
		return nil, nil, err
	}
	
	attributeKey := getAttributeKey(preK0, attributeName)
	if encrypted { 
		value, err = DecryptAttributeValue(attributeKey, value)
		if err != nil {
			return nil, nil, err
		}	
	}
	return attributeKey, value, nil
}

//Derives the K for the attribute "attributeName" and returns the key
func  GetKForAttribute(attributeName string, preK0 []byte, cert *x509.Certificate) ([]byte, error) {
	key, _ , err := getKAndValueForAttribute(attributeName, preK0, cert)
	return key, err
}

//Derives the K for the attribute "attributeName" and returns the value
func  GetValueForAttribute(attributeName string, preK0 []byte, cert *x509.Certificate) ([]byte, error) {
	_, value , err := getKAndValueForAttribute(attributeName, preK0, cert)
	return value, err
}

func createABACHeaderEntry(preK0 []byte) (*pb.ABACMetadataEntry, error) {
	attKey := getAttributeKey(preK0, HeaderAttributeName)
	return &pb.ABACMetadataEntry{HeaderAttributeName, attKey}, nil
}

func createABACMetadataEntry(attributeName string, preK0 []byte) (*pb.ABACMetadataEntry, error) {
	attKey := getAttributeKey(preK0, attributeName)
	return &pb.ABACMetadataEntry{attributeName, attKey}, nil
}

//Create an ABACMetadata object from certificate "cert", metadata and the attributes keys.
func CreateABACMetadataObjectFromCert(cert *x509.Certificate, metadata []byte, preK0 []byte, attributeKeys []string) (*pb.ABACMetadata, error) { 
	entries := make([]*pb.ABACMetadataEntry,0)
	for _, key := range(attributeKeys) { 
		if len(key) == 0 { 
			continue
		}
		entry, err := createABACMetadataEntry(key, preK0)
		if err == nil { 
			entries = append(entries, entry)
		}
	}
	headerEntry, err := createABACHeaderEntry(preK0)
	if err == nil { 
		entries = append(entries, headerEntry)
	}
	return &pb.ABACMetadata{metadata, entries}, nil
} 

//Create the ABACMetadata from the original metadata and certificate "cert".
func CreateABACMetadataFromCert(cert *x509.Certificate, metadata []byte, preK0 []byte, attributeKeys []string) ([]byte, error) { 
	abac_metadata, err := CreateABACMetadataObjectFromCert(cert, metadata, preK0, attributeKeys)
	if err != nil { 
		return nil, err
	}
	return proto.Marshal(abac_metadata)
}

//Create the ABACMetadata from the original metadata
func CreateABACMetadata(raw []byte, metadata []byte, preK0 []byte, attributeKeys []string) ([]byte, error) { 
	cert, err := utils.DERToX509Certificate(raw)
	if err != nil { 
		return nil, err
	}
	
	return CreateABACMetadataFromCert(cert, metadata, preK0, attributeKeys) 
}

//GetABACMetadata object from the original metadata "metadata".
func GetABACMetadata(metadata []byte) (*pb.ABACMetadata, error) { 
	abacMetadata := &pb.ABACMetadata{}
	err := proto.Unmarshal(metadata, abacMetadata)
	return abacMetadata, err
}

//Build a header attribute from a map of attribute names and positions.
func BuildAttributesHeader(attributesHeader map[string]int) []byte{
	var header []byte
	var headerString string
	for k,v := range attributesHeader	{
		v_str := strconv.Itoa(v)
		headerString = headerString + k + "->" + v_str + "#"
	}
	header = []byte(HeaderPrefix+headerString)
	return header
}