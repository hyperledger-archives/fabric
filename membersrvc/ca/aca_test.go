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
	"errors"
	"io/ioutil"
	"math/big"
	"strings"
	"testing"
	"time"

	"crypto/x509"
	"google/protobuf"

	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric/core/crypto/primitives"
	pb "github.com/hyperledger/fabric/membersrvc/protos"
	"golang.org/x/net/context"
)

var identity = "diego"

func loadECert(identityID string) (*x509.Certificate, error) {
	ecertRaw, err := ioutil.ReadFile("./test_resources/ecert.dump")
	if err != nil {
		return nil, err
	}

	ecert, err := x509.ParseCertificate(ecertRaw)

	if err != nil {
		return nil, err
	}

	var certificateID = strings.Split(ecert.Subject.CommonName, "\\")[0]

	if identityID != certificateID {
		return nil, errors.New("Incorrect ecert user.")
	}

	return ecert, nil
}

func TestFetchAttributes(t *testing.T) {

	cert, err := loadECert(identity)

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
		ECert:     &pb.Cert{Cert: cert.Raw},
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

	cert, err := loadECert(identity)

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
		ECert:     &pb.Cert{Cert: cert.Raw},
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

	cert, err := loadECert(identity)
	if err != nil {
		t.Fatalf("Error loading ECert: %v", err)
	}
	ecert := cert.Raw

	sock, acaP, err := GetACAClient()
	if err != nil {
		t.Fatalf("Error executing test: %v", err)
	}
	defer sock.Close()

	var attributes = make([]*pb.TCertAttribute, 0)
	attributes = append(attributes, &pb.TCertAttribute{AttributeName: "company"})
	attributes = append(attributes, &pb.TCertAttribute{AttributeName: "position"})
	attributes = append(attributes, &pb.TCertAttribute{AttributeName: "identity-number"})

	req := &pb.ACAAttrReq{
		Ts:         &google_protobuf.Timestamp{Seconds: time.Now().Unix(), Nanos: 0},
		Id:         &pb.Identity{Id: identity},
		ECert:      &pb.Cert{Cert: ecert},
		Attributes: attributes,
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

	aCert, err := primitives.DERToX509Certificate(resp.Cert.Cert)
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

	cert, err := loadECert(identity)
	if err != nil {
		t.Fatalf("Error loading ECert: %v", err)
	}
	ecert := cert.Raw

	sock, acaP, err := GetACAClient()
	if err != nil {
		t.Fatalf("Error executing test: %v", err)
	}
	defer sock.Close()

	var attributes = make([]*pb.TCertAttribute, 0)
	attributes = append(attributes, &pb.TCertAttribute{AttributeName: "account"})

	req := &pb.ACAAttrReq{
		Ts:         &google_protobuf.Timestamp{Seconds: time.Now().Unix(), Nanos: 0},
		Id:         &pb.Identity{Id: identity},
		ECert:      &pb.Cert{Cert: ecert},
		Attributes: attributes,
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
		t.Fatal("Test failed 'account' attribute shouldn't be found.")
	}

}

func TestRequestAttributes_MissingSignature(t *testing.T) {

	cert, err := loadECert(identity)
	if err != nil {
		t.Fatalf("Error loading ECert: %v", err)
	}
	ecert := cert.Raw

	sock, acaP, err := GetACAClient()
	if err != nil {
		t.Fatalf("Error executing test: %v", err)
	}
	defer sock.Close()

	var attributes = make([]*pb.TCertAttribute, 0)
	attributes = append(attributes, &pb.TCertAttribute{AttributeName: "company"})
	attributes = append(attributes, &pb.TCertAttribute{AttributeName: "position"})
	attributes = append(attributes, &pb.TCertAttribute{AttributeName: "identity-number"})

	req := &pb.ACAAttrReq{
		Ts:         &google_protobuf.Timestamp{Seconds: time.Now().Unix(), Nanos: 0},
		Id:         &pb.Identity{Id: identity},
		ECert:      &pb.Cert{Cert: ecert},
		Attributes: attributes,
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

	cert, err := loadECert(identity)
	if err != nil {
		t.Fatalf("Error loading ECert: %v", err)
	}
	ecert := cert.Raw

	sock, acaP, err := GetACAClient()
	if err != nil {
		t.Fatalf("Error executing test: %v", err)
	}
	defer sock.Close()

	var attributes = make([]*pb.TCertAttribute, 0)
	attributes = append(attributes, &pb.TCertAttribute{AttributeName: "company"})
	attributes = append(attributes, &pb.TCertAttribute{AttributeName: "company"})

	req := &pb.ACAAttrReq{
		Ts:         &google_protobuf.Timestamp{Seconds: time.Now().Unix(), Nanos: 0},
		Id:         &pb.Identity{Id: identity},
		ECert:      &pb.Cert{Cert: ecert},
		Attributes: attributes,
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

func TestRequestAttributes_FullAttributes(t *testing.T) {

	cert, err := loadECert(identity)
	if err != nil {
		t.Fatalf("Error loading ECert: %v", err)
	}
	ecert := cert.Raw

	sock, acaP, err := GetACAClient()
	if err != nil {
		t.Fatalf("Error executing test: %v", err)
	}
	defer sock.Close()

	var attributes []*pb.TCertAttribute
	attributes = append(attributes, &pb.TCertAttribute{AttributeName: "company"})
	attributes = append(attributes, &pb.TCertAttribute{AttributeName: "business_unit"})

	req := &pb.ACAAttrReq{
		Ts:         &google_protobuf.Timestamp{Seconds: time.Now().Unix(), Nanos: 0},
		Id:         &pb.Identity{Id: identity},
		ECert:      &pb.Cert{Cert: ecert},
		Attributes: attributes,
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

	aCert, err := primitives.DERToX509Certificate(resp.Cert.Cert)
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
		t.Fatalf("The attribute should have coincided.")
	}

	if valueMap["business_unit"] != "Sales" {
		t.Fatalf("The attribute should have coincided.")
	}

	if resp.Status != pb.ACAAttrResp_FULL_SUCCESSFUL {
		t.Fatalf("All attributes in the query should have coincided.")
	}
}

func TestRequestAttributes_PartialAttributes(t *testing.T) {

	cert, err := loadECert(identity)
	if err != nil {
		t.Fatalf("Error loading ECert: %v", err)
	}
	ecert := cert.Raw

	sock, acaP, err := GetACAClient()
	if err != nil {
		t.Fatalf("Error executing test: %v", err)
	}
	defer sock.Close()

	var attributes []*pb.TCertAttribute
	attributes = append(attributes, &pb.TCertAttribute{AttributeName: "company"})
	attributes = append(attributes, &pb.TCertAttribute{AttributeName: "credit_card"})

	req := &pb.ACAAttrReq{
		Ts:         &google_protobuf.Timestamp{Seconds: time.Now().Unix(), Nanos: 0},
		Id:         &pb.Identity{Id: identity},
		ECert:      &pb.Cert{Cert: ecert},
		Attributes: attributes,
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

	aCert, err := primitives.DERToX509Certificate(resp.Cert.Cert)
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
		t.Fatalf("The attribute should have coincided.")
	}

	if valueMap["credit_card"] != "" {
		t.Fatalf("The Attribute should be blank.")
	}

	if resp.Status == pb.ACAAttrResp_NO_ATTRIBUTES_FOUND {
		t.Fatalf("At least one attribute must be conincided")
	}

	if resp.Status != pb.ACAAttrResp_PARTIAL_SUCCESSFUL {
		t.Fatalf("All attributes in the query should have coincided.")
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
