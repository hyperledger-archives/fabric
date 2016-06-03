/*
Copyright IBM Corp. 2016 All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

		 http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package ac

import (
	"bytes"
	"crypto/x509"
	"errors"

	"github.com/hyperledger/fabric/core/crypto/attributes"
	attributespb "github.com/hyperledger/fabric/core/crypto/attributes/proto"
	"github.com/hyperledger/fabric/core/crypto/utils"
)

//Attribute defines a key, value pair to be verified.
type Attribute struct {
	Name  string
	Value []byte
}

// chaincodeHolder is the struct that hold the certificate and the metadata. An implementation is ChaincodeStub
type chaincodeHolder interface {
	// GetCallerCertificate returns caller certificate
	GetCallerCertificate() ([]byte, error)

	// GetCallerMetadata returns caller metadata
	GetCallerMetadata() ([]byte, error)
}

//ABACHandler is an entity can be used to both verify and read attributes.
//		The hanlder can retrieve the attributes, and the propertly keys to decrypt the values from the chaincodeHolder
//		The functions declared can be used to access the attributes stored in the transaction certificates from the application layer. Can be used directly from the ChaincodeStub API but
//		 if you need multiple access create a hanlder is better:
// 	Multiple accesses
// 		If multiple calls to the functions above are required, a best practice is to create an ABACHandler instead of calling the functions multiple times, this practice will avoid creating a new abacHandler for each of these calls thus eliminating an unnecessary overhead.
//    Example:
//
//		abacHandler, err := ac.NewABACHandlerImpl(stub)
//		if err != nil {
//			return false, err
//		}
//		abacHandler.VerifyAttribute(attributeName, attributeValue)
//		... you can make other verifications and/or read attribute values by using the abacHandler
type ABACHandler interface {

	//VerifyAttributes does the same as VerifyAttribute but it checks for a list of attributes and their respective values instead of a single attribute/value pair
	// Example:
	//    containsAttrs, error:= handler.VerifyAttributes(&ac.Attribute{"position",  "Software Engineer"}, &ac.Attribute{"company", "ACompany"})
	VerifyAttributes(attrs ...*Attribute) (bool, error)

	//VerifyAttribute is used to verify if the transaction certificate has an attribute with name *attributeName* and value *attributeValue* which are the input parameters received by this function.
	//Example:
	//    containsAttr, error := handler.VerifyAttribute("position", "Software Engineer")
	VerifyAttribute(attributeName string, attributeValue []byte) (bool, error)

	//GetValue is used to read an specific attribute from the transaction certificate, *attributeName* is passed as input parameter to this function.
	// Example:
	//  attrValue,error:=handler.GetValue("position")
	GetValue(attributeName string) ([]byte, error)
}

//ABACHandlerImpl is an implementation of ABACHandler interface.
type ABACHandlerImpl struct {
	cert      *x509.Certificate
	cache     map[string][]byte
	keys      map[string][]byte
	header    map[string]int
	encrypted bool
}

//NewABACHandlerImpl creates a new ABACHandlerImpl from a pb.ChaincodeSecurityContext object.
func NewABACHandlerImpl(holder chaincodeHolder) (*ABACHandlerImpl, error) {
	// Getting certificate
	certRaw, err := holder.GetCallerCertificate()
	if err != nil {
		return nil, err
	}
	var tcert *x509.Certificate
	tcert, err = utils.DERToX509Certificate(certRaw)
	if err != nil {
		return nil, err
	}

	//Getting Attributes Metadata from security context.
	var attrsMetadata *attributespb.AttributesMetadata
	var rawMetadata []byte
	rawMetadata, err = holder.GetCallerMetadata()
	if err != nil {
		return nil, err
	}

	attrsMetadata, err = attributes.GetAttributesMetadata(rawMetadata)

	if err != nil {
		return nil, err
	}

	keys := make(map[string][]byte)
	for _, entry := range attrsMetadata.Entries {
		keys[entry.AttributeName] = entry.AttributeKey
	}

	cache := make(map[string][]byte)
	return &ABACHandlerImpl{tcert, cache, keys, nil, false}, nil
}

func (abacHandler *ABACHandlerImpl) readHeader() (map[string]int, bool, error) {
	if abacHandler.header != nil {
		return abacHandler.header, abacHandler.encrypted, nil
	}
	header, encrypted, err := attributes.ReadAttributeHeader(abacHandler.cert, abacHandler.keys[attributes.HeaderAttributeName])
	if err != nil {
		return nil, false, err
	}
	abacHandler.header = header
	abacHandler.encrypted = encrypted
	return header, encrypted, nil
}

//GetValue is used to read an specific attribute from the transaction certificate, *attributeName* is passed as input parameter to this function.
//	Example:
//  	attrValue,error:=handler.GetValue("position")
func (abacHandler *ABACHandlerImpl) GetValue(attributeName string) ([]byte, error) {
	if abacHandler.cache[attributeName] != nil {
		return abacHandler.cache[attributeName], nil
	}
	header, encrypted, err := abacHandler.readHeader()
	if err != nil {
		return nil, err
	}
	value, err := attributes.ReadTCertAttributeByPosition(abacHandler.cert, header[attributeName])
	if err != nil {
		return nil, errors.New("error reading attribute value '" + err.Error() + "'")
	}
	if abacHandler.keys[attributeName] == nil {
		return nil, errors.New("There isn't a key")
	}
	if encrypted {
		value, err = attributes.DecryptAttributeValue(abacHandler.keys[attributeName], value)
		if err != nil {
			return nil, errors.New("error decrypting value '" + err.Error() + "'")
		}
	}
	abacHandler.cache[attributeName] = value
	return value, nil
}

//VerifyAttribute is used to verify if the transaction certificate has an attribute with name *attributeName* and value *attributeValue* which are the input parameters received by this function.
//	Example:
//  	containsAttr, error := handler.VerifyAttribute("position", "Software Engineer")
func (abacHandler *ABACHandlerImpl) VerifyAttribute(attributeName string, attributeValue []byte) (bool, error) {
	valueHash, err := abacHandler.GetValue(attributeName)
	if err != nil {
		return false, err
	}
	return bytes.Compare(valueHash, attributeValue) == 0, nil
}

//VerifyAttributes does the same as VerifyAttribute but it checks for a list of attributes and their respective values instead of a single attribute/value pair
//	Example:
//  	containsAttrs, error:= handler.VerifyAttributes(&ac.Attribute{"position",  "Software Engineer"}, &ac.Attribute{"company", "ACompany"})
func (abacHandler *ABACHandlerImpl) VerifyAttributes(attrs ...*Attribute) (bool, error) {
	for _, attribute := range attrs {
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
