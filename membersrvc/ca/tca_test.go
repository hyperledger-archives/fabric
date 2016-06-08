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
	"crypto/x509"
	"fmt"
	"google/protobuf"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"golang.org/x/net/context"

	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric/core/crypto"
	"github.com/hyperledger/fabric/core/crypto/primitives"
	"github.com/hyperledger/fabric/membersrvc/protos"
)

func TestNewTCA(t *testing.T) {
	tca, err := initTCA()
	if err != nil {
		t.Fatal(err)
	}

	if tca.hmacKey == nil || len(tca.hmacKey) == 0 {
		t.Fatal("Could not read hmacKey from TCA")
	}

	if tca.rootPreKey == nil || len(tca.rootPreKey) == 0 {
		t.Fatal("Could not read rootPreKey from TCA")
	}

	if tca.preKeys == nil || len(tca.preKeys) == 0 {
		t.Fatal("Could not read preKeys from TCA")
	}
}

func TestCreateCertificateSet(t *testing.T) {
	tca, err := initTCA()
	if err != nil {
		t.Fatal(err)
	}

	ecert, priv, err := loadECertAndEnrollmentPrivateKey("test_user0")
	if err != nil {
		t.Fatal(err)
	}

	certificateSetRequest, err := buildCertificateSetRequest(ecert.Subject.CommonName, priv, 1)
	if err != nil {
		t.Fatal(err)
	}

	tcap := &TCAP{tca}
	response, err := tcap.CreateCertificateSet(context.Background(), certificateSetRequest)
	if err != nil {
		t.Fatal(err)
	}

	tcerts := response.GetCerts()

	if len(tcerts.Certs) != 1 {
		t.Fatal(fmt.Errorf("Invalid tcert size. Expected: %v, Actual: %v", 1, len(tcerts.Certs)))
	}
}

func loadECertAndEnrollmentPrivateKey(enrollmentID string) (*x509.Certificate, []byte, error) {
	enrollmentPrivateKey, err := ioutil.ReadFile("./test_resources/key_" + enrollmentID + ".dump")
	if err != nil {
		return nil, nil, err
	}

	ecertRaw, err := ioutil.ReadFile("./test_resources/ecert_" + enrollmentID + ".dump")
	if err != nil {
		return nil, nil, err
	}

	ecert, err := x509.ParseCertificate(ecertRaw)
	if err != nil {
		return nil, nil, err
	}

	return ecert, enrollmentPrivateKey, nil
}

func initTCA() (*TCA, error) {
	//init the crypto layer
	if err := crypto.Init(); err != nil {
		return nil, fmt.Errorf("Failed initializing the crypto layer [%v]", err)
	}

	//initialize logging to avoid panics in the current code
	LogInit(os.Stdout, os.Stdout, os.Stdout, os.Stderr, os.Stdout)

	eca := NewECA()
	if eca == nil {
		return nil, fmt.Errorf("Could not create a new ECA")
	}

	tca := NewTCA(eca)
	if tca == nil {
		return nil, fmt.Errorf("Could not create a new TCA")
	}

	return tca, nil
}

func buildCertificateSetRequest(enrollID string, enrollmentPrivKey []byte, num int) (*protos.TCertCreateSetReq, error) {
	now := time.Now()
	timestamp := google_protobuf.Timestamp{Seconds: int64(now.Second()), Nanos: int32(now.Nanosecond())}

	var attributes []*protos.TCertAttribute

	req := &protos.TCertCreateSetReq{
		Ts:         &timestamp,
		Id:         &protos.Identity{Id: enrollID},
		Num:        uint32(num),
		Attributes: attributes,
		Sig:        nil,
	}

	rawReq, err := proto.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("Failed marshaling request [%v].", err)
	}

	r, s, err := primitives.ECDSASignDirect(enrollmentPrivKey, rawReq)
	if err != nil {
		return nil, fmt.Errorf("Failed creating signature for [%v]: [%v].", rawReq, err)
	}

	R, _ := r.MarshalText()
	S, _ := s.MarshalText()

	req.Sig = &protos.Signature{Type: protos.CryptoType_ECDSA, R: R, S: S}
	return req, nil
}
