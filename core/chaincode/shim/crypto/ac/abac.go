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
	"crypto/x509"

	pb "github.com/hyperledger/fabric/protos"

	"github.com/hyperledger/fabric/core/crypto/primitives"
	"github.com/hyperledger/fabric/core/crypto/utils"
	"github.com/hyperledger/fabric/core/crypto/abac"


)

type Attribute struct { 
	name 			string
	valueHash		[]byte
}

// ABACVerifier verifies attributes
type ABACVerifier interface {

	// VerifyAttributes verifies passed attributes within the message..
	VerifyAttributes(securityContext *ChaincodeSecurityContext, attrs...Attribute) (bool, error)
}

type ABACVerifierImpl struct {
	cert        *x509.Certificate
	preKey0		[]byte
	signatures	[][]byte
	keys		*map[string][]byte
	cache	    *map[string][]byte
}

//Reads the preK0 from the security context.
func GetPreK0(securityContext *pb.ChaincodeSecurityContext) ([]byte, error) {
	
}

//Creates a certificate object from the security context.
func GetCertificate(securityContext *pb.ChaincodeSecurityContext) (*x509.Certificate, error) {
	return utils.DERToX509Certificate(securityContext.CallerCert)
}

//Gets the attribute signatures from the security context.
func GetAttributeSignatures(securityContext *pb.ChaincodeSecurityContext) ([][]byte, error) {

}

//Creates a new ABACVerifierImpl from a pb.ChaincodeSecurityContext object.
func NewABACVerifierImpl(securityContext *pb.ChaincodeSecurityContext) (*ABACVerifierImpl, error) { 
	preK0, err := GetPreKey0(securityContext)
	if err != nil { 
		return nil, err
	}
	
	var tcert *x509.Certificate
	tcert, err = GetCertificate(securityContext)
	if err != nil {
		return nil, err
	}

	
	var signatures [][]byte
	signatures, err = GetAttributeSignatures(securityContext)
	if err!= nil { 
		return nil, err
	}
	
	keys := &make(map[string][]byte)
	cache := &make(map[string][]byte)
	return &ABACVerifierImpl{tcert, preK0, signatures, keys, cache}
}

func (abacVerifier *ABACVerifierImpl) VerifyAttributes(securityContext *ChaincodeSecurityContext, attrs...Attribute) (bool, error) { 
	
	abac
}