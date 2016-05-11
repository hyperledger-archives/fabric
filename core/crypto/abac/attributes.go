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
	"fmt"
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
)

//Parse a string and return a map with the attributes.
func ParseAttributesHeader(header string) (map[string]int, error) { 
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

//Reads the attribute with name "attributeName" and return the value.
func ReadTCertAttribute(tcert *x509.Certificate, attributeName string) ([]byte, error) {
	var err error
	var header_raw []byte
	if header_raw, err = utils.GetCriticalExtension(tcert, TCertAttributesHeaders); err != nil {
		return nil, err
	}

	header_str := string(header_raw)	
	var header map[string]int
	header, err = ParseAttributesHeader(header_str)
	
	if err != nil {
		return nil, err
	}
	
	position := header[attributeName]
	
	if position == 0 {
		return nil, errors.New("Failed attribute '"+attributeName+"' doesn't exists in the TCert.")
	}

    oid := asn1.ObjectIdentifier{1, 2, 3, 4, 5, 6, 9 + position}
    
    var value []byte
    if value, err = utils.GetCriticalExtension(tcert, oid); err != nil {
		return nil, err
	}
    return value, nil
}

func pkcs5Unpad(src []byte) []byte {
	len := len(src)
	unpad := int(src[len-1])
	return src[:(len - unpad)]
}

func EncryptAttributeValue(attributeKey []byte, attributeValue []byte) ([]byte, error) { 
	value := append(attributeValue, Padding...)
	return primitives.CBCEncrypt(attributeKey, value)
}

//Derives the K for the attribute "attributeName". 
func  GetKForAttribute(attributeName string, preK0 []byte, cert *x509.Certificate) ([]byte, error) {
	attributeKey := primitives.HMACTruncated(preK0, []byte(attributeName), 32)
	
	attribute, err := ReadTCertAttribute(cert, attributeName)
	if err != nil {
		return nil, err
	}
	
	value, err := primitives.CBCDecrypt(attributeKey, attribute)
	if err != nil {
		return nil, err
	}
	
	value = pkcs5Unpad(value)
	
	lenPadding := len(Padding)
	lenValue := len(value)
	if bytes.Compare(Padding[0:lenPadding], value[lenValue - lenPadding:lenValue]) == 0 {
		return attributeKey, nil
	}

	return nil, errors.New("Error generating decryption key for attribute "+ attributeName + ". Decryption verification failed.")
}

func createABACMetadataEntry(cert *x509.Certificate, attributeName string, preK0 []byte) (*pb.ABACMetadataEntry, error) {
	attKey, err := GetKForAttribute(attributeName, preK0, cert)
	if err != nil { 
		return nil, err
	}
	
	return &pb.ABACMetadataEntry{attributeName, attKey}, nil
}

//Create an ABACMetadata object from certificate "cert", metadata and the attributes keys.
func CreateABACMetadataObjectFromCert(cert *x509.Certificate, metadata []byte, preK0 []byte, attributeKeys []string) (*pb.ABACMetadata, error) { 
	entries := make([]*pb.ABACMetadataEntry,0)
	for _, key := range(attributeKeys) { 
		fmt.Printf("Attribute %v \n", key)
		entry, err := createABACMetadataEntry(cert, key, preK0)
		if err != nil { 
			return nil, err
		}
		entries = append(entries, entry)
	}
	return &pb.ABACMetadata{metadata, entries}, nil
} 

//Create the ABACMetadata from the original metadata and certificate "cert".
func CreateABACMetadataFromCert(cert *x509.Certificate, metadata []byte, preK0 []byte, attributeKeys []string) ([]byte, error) { 
	abac_metadata, err := CreateABACMetadataObjectFromCert(cert, metadata, preK0, attributeKeys)
	if err != nil { 
		return nil, err
	}
	fmt.Printf("CreateABACMetadataFromCert() entries %v \n", len(abac_metadata.Entries))

	
	return proto.Marshal(abac_metadata)
}

//Create the ABACMetadata from the original metadata
func CreateABACMetadata(raw []byte, metadata []byte, preK0 []byte, attributeKeys []string) ([]byte, error) { 
	fmt.Printf("CreateABACMetadata() %v \n", len(attributeKeys))
	cert, err := utils.DERToX509Certificate(raw)
	if err != nil { 
		return nil, err
	}
	
	return CreateABACMetadataFromCert(cert, metadata, preK0, attributeKeys) 
}

func GetABACMetadata(metadata []byte) (*pb.ABACMetadata, error) { 
	abacMetadata := &pb.ABACMetadata{}
	err := proto.Unmarshal(metadata, abacMetadata)
	return abacMetadata, err
}