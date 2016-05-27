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

package abac

import (
	"crypto/x509"
	"encoding/pem"
	"io/ioutil"
	"testing"

	"github.com/hyperledger/fabric/core/crypto/primitives"
)

func TestEncryptDecryptAttributeValuePK0(t *testing.T) {
	expected := "ACompany"

	preK0 := []byte{
		91, 206, 163, 104, 247, 74, 149, 209, 91, 137, 215, 236,
		84, 135, 9, 70, 160, 138, 89, 163, 240, 223, 83, 164, 58,
		208, 199, 23, 221, 123, 53, 220, 15, 41, 28, 111, 166,
		28, 29, 187, 97, 229, 117, 117, 49, 192, 134, 31, 151}

	if err := primitives.InitSecurityLevel("SHA2", 256); err != nil {
		t.Errorf("Failed setting security level: %v", err)
	}

	encryptedAttribute, err := EncryptAttributeValuePK0(preK0, "company", []byte(expected))
	if err != nil {
		t.Error(err)
	}

	attributeKey := getAttributeKey(preK0, "company")

	attribute, err := DecryptAttributeValue(attributeKey, encryptedAttribute)
	if err != nil {
		t.Error(err)
	}

	if string(attribute) != expected {
		t.Errorf("Failed decrypting attribute. Expected: %v, Actual: %v", expected, attribute)
	}
}

func TestGetKAndValueForAttribute(t *testing.T) {
	expected := "ACompany"

	if err := primitives.InitSecurityLevel("SHA2", 256); err != nil {
		t.Errorf("Failed setting security level: %v", err)
	}

	tcert, prek0, err := loadTCertAndPreK0()
	if err != nil {
		t.Error(err)
	}

	_, attribute, err := getKAndValueForAttribute("company", prek0, tcert)
	if err != nil {
		t.Error(err)
	}

	if string(attribute) != expected {
		t.Errorf("Failed retrieving attribute value from TCert. Expected: %v, Actual: %v", expected, string(attribute))
	}
}

func TestGetValueForAttribute(t *testing.T) {
	expected := "ACompany"

	if err := primitives.InitSecurityLevel("SHA2", 256); err != nil {
		t.Errorf("Failed setting security level: %v", err)
	}

	tcert, prek0, err := loadTCertAndPreK0()
	if err != nil {
		t.Error(err)
	}

	value, err := GetValueForAttribute("company", prek0, tcert)
	if err != nil {
		t.Error(err)
	}

	if string(value) != expected {
		t.Errorf("Failed retrieving attribute value from TCert. Expected: %v, Actual: %v", expected, string(value))
	}
}

func TestParseEmptyAttributesHeader(t *testing.T) {
	_, err := ParseAttributesHeader("")
	if err == nil {
		t.Error("Empty header should produce a parsing error")
	}
}

func TestBuildAndParseAttributesHeader(t *testing.T) {
	attributes := make(map[string]int)
	attributes["company"] = 1
	attributes["position"] = 2

	header := string(BuildAttributesHeader(attributes)[:])

	components, err := ParseAttributesHeader(header)
	if err != nil {
		t.Error(err)
	}

	if components["company"] != 1 {
		t.Errorf("Error parsing header. Expected %v with value %v, found %v instead", "company", 1, components["company"])
	}

	if components["position"] != 2 {
		t.Errorf("Error parsing header. Expected %v with value %v, found %v instead", "position", 2, components["position"])
	}
}

func loadTCertAndPreK0() (*x509.Certificate, []byte, error) {
	preKey0, err := ioutil.ReadFile("./test_resources/prek0.dump")
	if err != nil {
		return nil, nil, err
	}

	if err != nil {
		return nil, nil, err
	}

	tcertRaw, err := ioutil.ReadFile("./test_resources/tcert.dump")
	if err != nil {
		return nil, nil, err
	}

	tcertDecoded, _ := pem.Decode(tcertRaw)

	tcert, err := x509.ParseCertificate(tcertDecoded.Bytes)
	if err != nil {
		return nil, nil, err
	}

	return tcert, preKey0, nil
}
