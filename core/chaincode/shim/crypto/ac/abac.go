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
	"crypto/x509"

	abacpb "github.com/hyperledger/fabric/core/crypto/abac/proto"
	"github.com/hyperledger/fabric/core/crypto/utils"
	"github.com/hyperledger/fabric/core/crypto/abac"
	
)

type Attribute struct { 
	Name 			string
	Value    		[]byte
}

// ChaincodeHolder is the struct that hold the certificate and the metadata. An implementation is ChaincodeStub
type ChaincodeHolder interface { 
	// GetCallerCertificate returns caller certificate
	GetCallerCertificate() ([]byte, error)
	
	// GetCallerMetadata returns caller metadata
	GetCallerMetadata() ([]byte, error)
	
}

// ABACHandler verifies attributes
type ABACHandler interface {

	// VerifyAttributes verifies passed attributes within the message..
	VerifyAttributes(attrs...*Attribute) (bool, error)
	
	// VerifyAttribute verifies if the attribute with name "attributeName" has the value "attributeValue"
	VerifyAttribute(attributeName string, attributeValue []byte) (bool, error)
	
	
	//Returns the value on the certificate of the attribute with name "attributeName".	
	GetValue(attributeName string) ([]byte, error) 
}

type ABACHandlerImpl struct {
	cert        *x509.Certificate
	cache   map[string][]byte
	keys		map[string][]byte
}

//Creates a new ABACHandlerImpl from a pb.ChaincodeSecurityContext object.
func NewABACHandlerImpl(holder ChaincodeHolder) (*ABACHandlerImpl, error) { 
	// Getting certificate
	cert_raw, err := holder.GetCallerCertificate()
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
	raw_metadata, err = holder.GetCallerMetadata()
	if err != nil {
		return nil, err
	}
	abacMetadata, err = abac.GetABACMetadata(raw_metadata) 
	if err != nil { 
		return nil, err
	}
	
	keys := make(map[string][]byte)
	for _,entry := range(abacMetadata.Entries) { 
		keys[entry.AttributeName] = entry.AttributeKey
	}
	
	cache := make(map[string][]byte)	
	return &ABACHandlerImpl{tcert, cache, keys}, nil
}

//Returns the value on the certificate of the attribute with name "attributeName".
func (abacHandler *ABACHandlerImpl) GetValue(attributeName string) ([]byte, error) { 
		if abacHandler.cache[attributeName] != nil { 
			return abacHandler.cache[attributeName], nil
		}
		
		value, err := abac.ReadTCertAttribute(abacHandler.cert, attributeName)
		if err != nil { 
			return nil, err
		}
		
		if abacHandler.keys[attributeName] == nil { 
			return nil, errors.New("There isn't a key")
		}
		
		return abac.DecryptAttributeValue(abacHandler.keys[attributeName], value)
}

// VerifyAttribute verifies if the attribute with name "attributeName" has the value "attributeValue"
func (abacHandler *ABACHandlerImpl) VerifyAttribute(attributeName string, attributeValue []byte) (bool, error){
	valueHash, err := abacHandler.GetValue(attributeName)	
	if err != nil { 
		return false, err
	}
	return bytes.Compare(valueHash, attributeValue) == 0, nil
}

//Verifies all the attributes included in attrs.
func (abacHandler *ABACHandlerImpl) VerifyAttributes(attrs...*Attribute) (bool, error) { 
	for _, attribute := range(attrs) { 
		val, err := abacHandler.VerifyAttribute(attribute.Name, attribute.Value)
		if err != nil { 
			return false, nil
		}
		if !val { 
			return val, nil
		}
	}
	return true, nil
}