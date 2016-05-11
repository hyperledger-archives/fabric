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

package ac

import (
	"errors"
	"bytes" 
	"fmt"
	
	"crypto/x509"

	abacpb "github.com/hyperledger/fabric/core/crypto/abac/proto"

	"github.com/hyperledger/fabric/core/chaincode/shim"
	"github.com/hyperledger/fabric/core/crypto/utils"
	"github.com/hyperledger/fabric/core/crypto/abac"
	
)

type Attribute struct { 
	Name 			string
	Value    		[]byte
}

// ABACVerifier verifies attributes
type ABACVerifier interface {

	// VerifyAttributes verifies passed attributes within the message..
	VerifyAttributes(attrs...*Attribute) (bool, error)
}

type ABACVerifierImpl struct {
	cert        *x509.Certificate
	signature   map[string][]byte
	keys		map[string][]byte
}

//Creates a new ABACVerifierImpl from a pb.ChaincodeSecurityContext object.
func NewABACVerifierImpl(stub *shim.ChaincodeStub) (*ABACVerifierImpl, error) { 
	// Getting certificate
	cert_raw, err := stub.GetCallerCertificate()
	if err != nil { 
		return nil, err
	}
	var tcert *x509.Certificate
	tcert, err = utils.DERToX509Certificate(cert_raw)
	if err != nil {
		return nil, err
	}

	//Getting ABAC Metadata from security context.
	var abacMetadata *abacpb.ABACMetadata
	var raw_metadata []byte
	raw_metadata, err = stub.GetCallerMetadata()
	if err != nil {
		return nil, err
	}
	abacMetadata, err = abac.GetABACMetadata(raw_metadata) 
	if err != nil { 
		return nil, err
	}
	
	fmt.Printf("Metadata created attributes %v .\n", len(abacMetadata.Entries))
	
	keys := make(map[string][]byte)
	for _,entry := range(abacMetadata.Entries) { 
		keys[entry.AttributeName] = entry.AttributeKey
	}
	
	signatures := make(map[string][]byte)
	for attributeName,_ := range(keys) { 
		value, err := abac.ReadTCertAttribute(tcert, attributeName)
		if err != nil { 
			return nil, err
		}
		signatures[attributeName] = value
	}
	
	return &ABACVerifierImpl{tcert, signatures, keys}, nil
}

func (abacVerifier *ABACVerifierImpl) getValueHash(attributeName string) ([]byte, error) { 
		return abacVerifier.signature[attributeName], nil	
}

func (abacVerifier *ABACVerifierImpl) getCalculateValueHash(attribute *Attribute) ([]byte, error) { 
	key := abacVerifier.keys[attribute.Name]
	if key == nil { 
		return nil, errors.New("The key to the attribute '"+attribute.Name+"' could not be found.")
	}
	return abac.EncryptAttributeValue(key, attribute.Value)
}

//Verfies the attribute "attribute".
func (abacVerifier *ABACVerifierImpl) VerifyAttribute(attribute *Attribute) (bool, error) { 
	valueHash, err := abacVerifier.getValueHash(attribute.Name)	
	if err != nil { 
		return false, err
	}
	calculateValue, err := abacVerifier.getCalculateValueHash(attribute)
	if err != nil { 
		return false, err
	}
	fmt.Printf("Expected %v calculated %v \n", valueHash, calculateValue)

	return bytes.Compare(valueHash, calculateValue) == 0, nil
}

//Verifies all the attributes included in attrs.
func (abacVerifier *ABACVerifierImpl) VerifyAttributes(attrs...*Attribute) (bool, error) { 
	fmt.Println("Verifing attributes...")
	for _, attribute := range(attrs) { 
		val, err := abacVerifier.VerifyAttribute(attribute)
		fmt.Printf("Verfied attribute %v \n", val)
		if err != nil { 
			return false, nil
		}
		if !val { 
			return val, nil
		}
	}
	return true, nil
}