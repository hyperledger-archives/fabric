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

package ca

import (
	"io/ioutil"
	"math/big"
	"testing"
	"time"

	"crypto/x509"
	"google/protobuf"

	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric/core/crypto/primitives"
	"github.com/hyperledger/fabric/core/crypto/utils"
	pb "github.com/hyperledger/fabric/membersrvc/protos"
	"golang.org/x/net/context"
)

func loadECert() (*x509.Certificate, error) {
	ecertRaw, err := ioutil.ReadFile("./test_resources/ecert.dump")
	if err != nil {
		return nil, err
	}

	ecert, err := x509.ParseCertificate(ecertRaw)
	if err != nil {
		return nil, err
	}

	return ecert, nil
}

func TestFetchAttributes(t *testing.T) {

	cert, err := loadECert()

	if err != nil {
		t.Fatalf("Error loading ECert: %v", err)
	}
	sock, acaP, err := GetACAClient()
	if err != nil {
		t.Fatalf("Error executing test: %v", err)
	}
	defer sock.Close()

	req := &pb.ACAFetchAttrReq{
		Ts:        &google_protobuf.Timestamp{Seconds: time.Now().Unix(), Nanos: 0},
		ECert:     &pb.Cert{cert.Raw},
		Signature: nil}

	var rawReq []byte
	rawReq, err = proto.Marshal(req)
	if err != nil {
		t.Fatalf("Error executing test: %v", err)
	}

	var r, s *big.Int

	r, s, err = primitives.ECDSASignDirect(eca.priv, rawReq)

	if err != nil {
		t.Fatalf("Error executing test: %v", err)
	}

	R, _ := r.MarshalText()
	S, _ := s.MarshalText()

	req.Signature = &pb.Signature{Type: pb.CryptoType_ECDSA, R: R, S: S}

	resp, err := acaP.FetchAttributes(context.Background(), req)
	if err != nil {
		t.Fatalf("Error executing test: %v", err)
	}

	if resp.Status == pb.ACAFetchAttrResp_FAILURE {
		t.Fatalf("Error executing test: %v", "Error fetching attributes.")
	}
}

func TestFetchAttributes_MissingSignature(t *testing.T) {

	cert, err := loadECert()

	if err != nil {
		t.Fatalf("Error loading ECert: %v", err)
	}
	sock, acaP, err := GetACAClient()
	if err != nil {
		t.Fatalf("Error executing test: %v", err)
	}
	defer sock.Close()

	req := &pb.ACAFetchAttrReq{
		Ts:        &google_protobuf.Timestamp{Seconds: time.Now().Unix(), Nanos: 0},
		ECert:     &pb.Cert{cert.Raw},
		Signature: nil}

	resp, err := acaP.FetchAttributes(context.Background(), req)

	if err != nil {
		t.Fatalf("Error executing test: %v", err)
	}

	if resp.Status == pb.ACAFetchAttrResp_SUCCESS {
		t.Fatalf("Fetching attributes without a signature should fail")
	}
}

func TestRequestAttributes(t *testing.T) {

	cert, err := loadECert()
	if err != nil {
		t.Fatalf("Error loading ECert: %v", err)
	}
	ecert := cert.Raw

	sock, acaP, err := GetACAClient()
	if err != nil {
		t.Fatalf("Error executing test: %v", err)
	}
	defer sock.Close()
	attributesHash := make([]*pb.TCertAttributeHash, 0)

	attributes := make([]*pb.TCertAttribute, 0)
	attributes = append(attributes, &pb.TCertAttribute{"company", "ACompany"})
	attributes = append(attributes, &pb.TCertAttribute{"position", "Software Engineer"})
	attributes = append(attributes, &pb.TCertAttribute{"identity-number", "1234"})

	for _, att := range attributes {
		attributeHash := pb.TCertAttributeHash{att.AttributeName, primitives.Hash([]byte(att.AttributeValue))}
		attributesHash = append(attributesHash, &attributeHash)
	}

	req := &pb.ACAAttrReq{
		Ts:         &google_protobuf.Timestamp{Seconds: time.Now().Unix(), Nanos: 0},
		Id:         &pb.Identity{"diego"},
		ECert:      &pb.Cert{ecert},
		Attributes: attributesHash,
		Signature:  nil}

	var rawReq []byte
	rawReq, err = proto.Marshal(req)
	if err != nil {
		t.Fatalf("Error executing test: %v", err)
	}

	var r, s *big.Int

	r, s, err = primitives.ECDSASignDirect(tca.priv, rawReq)

	if err != nil {
		t.Fatalf("Error executing test: %v", err)
	}

	R, _ := r.MarshalText()
	S, _ := s.MarshalText()

	req.Signature = &pb.Signature{Type: pb.CryptoType_ECDSA, R: R, S: S}

	resp, err := acaP.RequestAttributes(context.Background(), req)
	if err != nil {
		t.Fatalf("Error executing test: %v", err)
	}

	if resp.Status == pb.ACAAttrResp_FAILURE {
		t.Fatalf("Error executing test: %v", err)
	}

	aCert, err := utils.DERToX509Certificate(resp.Cert.Cert)
	if err != nil {
		t.Fatalf("Error executing test: %v", err)
	}

	valueMap := make(map[string]string)
	for _, eachExtension := range aCert.Extensions {
		if IsAttributeOID(eachExtension.Id) {
			var attribute pb.ACAAttribute
			proto.Unmarshal(eachExtension.Value, &attribute)
			valueMap[attribute.AttributeName] = string(attribute.AttributeValue)
		}
	}

	if valueMap["company"] != "ACompany" {
		t.Fatal("Test failed 'company' attribute don't found.")
	}

	if valueMap["position"] != "Software Engineer" {
		t.Fatal("Test failed 'position' attribute don't found.")
	}
}

func TestRequestAttributes_AttributesMismatch(t *testing.T) {

	cert, err := loadECert()
	if err != nil {
		t.Fatalf("Error loading ECert: %v", err)
	}
	ecert := cert.Raw

	sock, acaP, err := GetACAClient()
	if err != nil {
		t.Fatalf("Error executing test: %v", err)
	}
	defer sock.Close()
	attributesHash := make([]*pb.TCertAttributeHash, 0)

	attributes := make([]*pb.TCertAttribute, 0)
	attributes = append(attributes, &pb.TCertAttribute{"company", "BCompany"})

	for _, att := range attributes {
		attributeHash := pb.TCertAttributeHash{att.AttributeName, primitives.Hash([]byte(att.AttributeValue))}
		attributesHash = append(attributesHash, &attributeHash)
	}

	req := &pb.ACAAttrReq{
		Ts:         &google_protobuf.Timestamp{Seconds: time.Now().Unix(), Nanos: 0},
		Id:         &pb.Identity{"diego"},
		ECert:      &pb.Cert{ecert},
		Attributes: attributesHash,
		Signature:  nil}

	var rawReq []byte
	rawReq, err = proto.Marshal(req)
	if err != nil {
		t.Fatalf("Error executing test: %v", err)
	}

	var r, s *big.Int

	r, s, err = primitives.ECDSASignDirect(tca.priv, rawReq)

	if err != nil {
		t.Fatalf("Error executing test: %v", err)
	}

	R, _ := r.MarshalText()
	S, _ := s.MarshalText()

	req.Signature = &pb.Signature{Type: pb.CryptoType_ECDSA, R: R, S: S}

	resp, err := acaP.RequestAttributes(context.Background(), req)
	if err != nil {
		t.Fatalf("Error executing test: %v", err)
	}

	if resp.Status == pb.ACAAttrResp_FAILURE {
		t.Fatalf("Error executing test: %v", err)
	}

	if resp.Status != pb.ACAAttrResp_NO_ATTRIBUTES_FOUND {
		t.Fatal("Test failed 'company' attribute shouldn't be found.")
	}

}

func TestRequestAttributes_MissingSignature(t *testing.T) {

	cert, err := loadECert()
	if err != nil {
		t.Fatalf("Error loading ECert: %v", err)
	}
	ecert := cert.Raw

	sock, acaP, err := GetACAClient()
	if err != nil {
		t.Fatalf("Error executing test: %v", err)
	}
	defer sock.Close()
	attributesHash := make([]*pb.TCertAttributeHash, 0)

	attributes := make([]*pb.TCertAttribute, 0)
	attributes = append(attributes, &pb.TCertAttribute{"company", "ACompany"})
	attributes = append(attributes, &pb.TCertAttribute{"position", "Software Engineer"})
	attributes = append(attributes, &pb.TCertAttribute{"identity-number", "1234"})

	for _, att := range attributes {
		attributeHash := pb.TCertAttributeHash{att.AttributeName, primitives.Hash([]byte(att.AttributeValue))}
		attributesHash = append(attributesHash, &attributeHash)
	}

	req := &pb.ACAAttrReq{
		Ts:         &google_protobuf.Timestamp{Seconds: time.Now().Unix(), Nanos: 0},
		Id:         &pb.Identity{"diego"},
		ECert:      &pb.Cert{ecert},
		Attributes: attributesHash,
		Signature:  nil}

	resp, err := acaP.RequestAttributes(context.Background(), req)
	if err != nil {
		t.Fatalf("Error executing test: %v", err)
	}

	if resp.Status != pb.ACAAttrResp_BAD_REQUEST {
		t.Fatalf("Requesting attributes without a signature should fail")
	}
}

func TestRequestAttributes_DuplicatedAttributes(t *testing.T) {

	cert, err := loadECert()
	if err != nil {
		t.Fatalf("Error loading ECert: %v", err)
	}
	ecert := cert.Raw

	sock, acaP, err := GetACAClient()
	if err != nil {
		t.Fatalf("Error executing test: %v", err)
	}
	defer sock.Close()
	attributesHash := make([]*pb.TCertAttributeHash, 0)

	attributes := make([]*pb.TCertAttribute, 0)
	attributes = append(attributes, &pb.TCertAttribute{"company", "ACompany"})
	attributes = append(attributes, &pb.TCertAttribute{"company", "BCompany"})

	for _, att := range attributes {
		attributeHash := pb.TCertAttributeHash{att.AttributeName, primitives.Hash([]byte(att.AttributeValue))}
		attributesHash = append(attributesHash, &attributeHash)
	}

	req := &pb.ACAAttrReq{
		Ts:         &google_protobuf.Timestamp{Seconds: time.Now().Unix(), Nanos: 0},
		Id:         &pb.Identity{"diego"},
		ECert:      &pb.Cert{ecert},
		Attributes: attributesHash,
		Signature:  nil}

	var rawReq []byte
	rawReq, err = proto.Marshal(req)
	if err != nil {
		t.Fatalf("Error executing test: %v", err)
	}

	var r, s *big.Int

	r, s, err = primitives.ECDSASignDirect(tca.priv, rawReq)

	if err != nil {
		t.Fatalf("Error executing test: %v", err)
	}

	R, _ := r.MarshalText()
	S, _ := s.MarshalText()

	req.Signature = &pb.Signature{Type: pb.CryptoType_ECDSA, R: R, S: S}

	resp, err := acaP.RequestAttributes(context.Background(), req)
	if err != nil {
		t.Fatalf("Error executing test: %v", err)
	}

	if resp.Status != pb.ACAAttrResp_BAD_REQUEST {
		t.Fatalf("Requesting attributes with multiple values should fail")
	}
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}
