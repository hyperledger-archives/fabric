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

package validator

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/x509"
	obcca "github.com/openblockchain/obc-peer/protos"
	protobuf "google/protobuf"
	"time"

	"errors"
	"github.com/golang/protobuf/proto"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"io/ioutil"
	"github.com/openblockchain/obc-peer/openchain/crypto/utils"
)

func (validator *Validator) retrieveECACertsChain(userId string) error {
	// Retrieve ECA certificate and verify it
	ecaCertRaw, err := validator.getECACertificate()
	if err != nil {
		log.Error("Failed getting ECA certificate %s", err)

		return err
	}
	log.Info("Register:ECAcert %s", utils.EncodeBase64(ecaCertRaw))

	// TODO: Test ECA cert againt root CA
	_, err = utils.DERToX509Certificate(ecaCertRaw)
	if err != nil {
		log.Error("Failed parsing ECA certificate %s", err)

		return err
	}

	// Store ECA cert
	log.Info("Storing ECA certificate for validator [%s]...", userId)

	err = ioutil.WriteFile(getECACertsChainPath(), utils.DERCertToPEM(ecaCertRaw), 0700)
	if err != nil {
		log.Error("Failed storing eca certificate: %s", err)
		return err
	}

	return nil
}

func (validator *Validator) loadECACertsChain() error {
	log.Info("Loading ECA certificates chain at %s...", getECACertsChainPath())

	chain, err := ioutil.ReadFile(getECACertsChainPath())
	if err != nil {
		log.Error("Failed loading ECA certificates chain : %s", err.Error())

		return err
	}

	ok := validator.rootsCertPool.AppendCertsFromPEM(chain)
	if !ok {
		log.Error("Failed appending ECA certificates chain.")

		return errors.New("Failed appending ECA certificates chain.")
	}

	return nil
}

func (validator *Validator) callECACreateCertificate(ctx context.Context, in *obcca.ECertCreateReq, opts ...grpc.CallOption) (*obcca.Cert, error) {
	sockP, err := grpc.Dial(getECAPAddr(), grpc.WithInsecure())
	if err != nil {
		log.Error("Failed dailing in: %s", err)

		return nil, err
	}
	defer sockP.Close()

	ecaP := obcca.NewECAPClient(sockP)

	cert, err := ecaP.CreateCertificate(context.Background(), in)
	if err != nil {
		log.Error("Failed requesting enrollment certificate: %s", err)

		return nil, err
	}

	return cert, nil
}

func (validator *Validator) callECAReadCertificate(ctx context.Context, in *obcca.ECertReadReq, opts ...grpc.CallOption) (*obcca.Cert, error) {
	sockP, err := grpc.Dial(getECAPAddr(), grpc.WithInsecure())
	if err != nil {
		log.Error("Failed eca dialing in : %s", err)

		return nil, err
	}
	defer sockP.Close()

	ecaP := obcca.NewECAPClient(sockP)

	cert, err := ecaP.ReadCertificate(context.Background(), in)
	if err != nil {
		log.Error("Failed requesting read certificate: %s", err)

		return nil, err
	}

	return cert, nil
}

func (validator *Validator) getEnrollmentCertificateFromECA(id, pw string) (interface{}, []byte, error) {
	priv, err := utils.NewECDSAKey()

	if err != nil {
		log.Error("Failed generating key: %s", err)

		return nil, nil, err
	}

	// Prepare the request
	pubraw, _ := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	req := &obcca.ECertCreateReq{&protobuf.Timestamp{Seconds: time.Now().Unix(), Nanos: 0},
		&obcca.Identity{Id: id},
		&obcca.Password{Pw: pw},
		&obcca.PublicKey{Type: obcca.CryptoType_ECDSA, Key: pubraw}, nil}
	rawreq, _ := proto.Marshal(req)
	r, s, err := ecdsa.Sign(rand.Reader, priv, utils.Hash(rawreq))
	if err != nil {
		panic(err)
	}
	R, _ := r.MarshalText()
	S, _ := s.MarshalText()
	req.Sig = &obcca.Signature{obcca.CryptoType_ECDSA, R, S}

	pbCert, err := validator.callECACreateCertificate(context.Background(), req)
	if err != nil {
		log.Error("Failed requesting enrollment certificate: %s", err)

		return nil, nil, err
	}

	// Verify pbCert.Cert
	log.Info("Enrollment certificate hash: %s", utils.EncodeBase64(utils.Hash(pbCert.Cert)))

	return priv, pbCert.Cert, nil
}

func (validator *Validator) getECACertificate() ([]byte, error) {
	// Prepare the request
	req := &obcca.ECertReadReq{&obcca.Identity{Id: "eca-root"}, nil}
	pbCert, err := validator.callECAReadCertificate(context.Background(), req)
	if err != nil {
		log.Error("Failed requesting enrollment certificate: %s", err)

		return nil, err
	}

	// TODO Verify pbCert.Cert

	return pbCert.Cert, nil
}

func (validator *Validator) getEnrollmentCert(id []byte) (*x509.Certificate, error) {
	sid := utils.EncodeBase64(id)

	if cert := validator.enrollCerts[sid]; cert != nil {
		return cert, nil
	}

	// Retrieve from the DB or from the ECA in case
	rawCert, err := getDBHandle().GetEnrollmentCert(id, validator.getEnrollmentCertByHashFromECA)
	if err != nil {
		log.Error("Failed getting enrollment certificate for [%s]: %s", sid, err)
	}

	cert, err := utils.DERToX509Certificate(rawCert)
	if err != nil {
		log.Error("Failed parsing enrollment certificate for [%s]: %s", sid, utils.EncodeBase64(rawCert))
	}

	validator.enrollCerts[sid] = cert

	return cert, nil
}

func (validator *Validator) getEnrollmentCertByHashFromECA(id []byte) ([]byte, error) {
	// Prepare the request
	log.Info("Reading certificate for hash " + utils.EncodeBase64(id))

	req := &obcca.ECertReadReq{&obcca.Identity{Id: ""}, id}
	pbCert, err := validator.callECAReadCertificate(context.Background(), req)
	if err != nil {
		log.Error("Failed requesting enrollment certificate: %s", err)

		return nil, err
	}

	// TODO Verify pbCert.Cert
	return pbCert.Cert, nil
}
