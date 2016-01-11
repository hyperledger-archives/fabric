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

package crypto

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/x509"
	obcca "github.com/openblockchain/obc-peer/obc-ca/protos"
	protobuf "google/protobuf"
	"time"

	"errors"
	"github.com/golang/protobuf/proto"
	"github.com/openblockchain/obc-peer/openchain/crypto/utils"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"io/ioutil"
)

func (node *nodeImpl) retrieveECACertsChain(userID string) error {
	// Retrieve ECA certificate and verify it
	ecaCertRaw, err := node.getECACertificate()
	if err != nil {
		node.log.Error("Failed getting ECA certificate [%s].", err.Error())

		return err
	}
	node.log.Debug("ECA certificate [%s].", utils.EncodeBase64(ecaCertRaw))

	// TODO: Test ECA cert againt root CA
	_, err = utils.DERToX509Certificate(ecaCertRaw)
	if err != nil {
		node.log.Error("Failed parsing ECA certificate [%s].", err.Error())

		return err
	}

	// Store ECA cert
	node.log.Debug("Storing ECA certificate for validator [%s]...", userID)

	err = ioutil.WriteFile(node.conf.getECACertsChainPath(), utils.DERCertToPEM(ecaCertRaw), 0700)
	if err != nil {
		node.log.Error("Failed storing eca certificate [%s].", err.Error())
		return err
	}

	return nil
}

func (node *nodeImpl) loadECACertsChain() error {
	node.log.Debug("Loading ECA certificates chain at [%s]...", node.conf.getECACertsChainPath())

	chain, err := ioutil.ReadFile(node.conf.getECACertsChainPath())
	if err != nil {
		node.log.Error("Failed loading ECA certificates chain [%s].", err.Error())

		return err
	}

	ok := node.rootsCertPool.AppendCertsFromPEM(chain)
	if !ok {
		node.log.Error("Failed appending ECA certificates chain.")

		return errors.New("Failed appending ECA certificates chain.")
	}

	return nil
}

func (node *nodeImpl) callECACreateCertificate(ctx context.Context, in *obcca.ECertCreateReq, opts ...grpc.CallOption) (*obcca.Cert, []byte, error) {
	sockP, err := grpc.Dial(node.conf.getECAPAddr(), grpc.WithInsecure())
	if err != nil {
		node.log.Error("Failed dailing in [%s].", err.Error())

		return nil, nil, err
	}
	defer sockP.Close()

	ecaP := obcca.NewECAPClient(sockP)

	cred, err := ecaP.CreateCertificate(context.Background(), in)
	if err != nil {
		node.log.Error("Failed requesting enrollment certificate [%s].", err.Error())

		return nil, nil, err
	}

	return cred.Cert, cred.Key, nil
}

func (node *nodeImpl) callECAReadCACertificate(ctx context.Context, in *obcca.ECertReadReq, opts ...grpc.CallOption) (*obcca.Cert, error) {
	sockP, err := grpc.Dial(node.conf.getECAPAddr(), grpc.WithInsecure())
	if err != nil {
		node.log.Error("Failed eca dialing in [%s].", err.Error())

		return nil, err
	}
	defer sockP.Close()

	ecaP := obcca.NewECAPClient(sockP)

	cert, err := ecaP.ReadCACertificate(context.Background(), &protobuf.Empty{})
	if err != nil {
		node.log.Error("Failed requesting read certificate [%s].", err.Error())

		return nil, err
	}

	return cert, nil
}

func (node *nodeImpl) callECAReadCertificate(ctx context.Context, in *obcca.ECertReadReq, opts ...grpc.CallOption) (*obcca.Cert, error) {
	sockP, err := grpc.Dial(node.conf.getECAPAddr(), grpc.WithInsecure())
	if err != nil {
		node.log.Error("Failed eca dialing in [%s].", err.Error())

		return nil, err
	}
	defer sockP.Close()

	ecaP := obcca.NewECAPClient(sockP)

	cert, err := ecaP.ReadCertificate(context.Background(), in)
	if err != nil {
		node.log.Error("Failed requesting read certificate [%s].", err.Error())

		return nil, err
	}

	return cert, nil
}

func (node *nodeImpl) getEnrollmentCertificateFromECA(id, pw string) (interface{}, []byte, []byte, error) {
	priv, err := utils.NewECDSAKey()

	if err != nil {
		node.log.Error("Failed generating key [%s].", err.Error())

		return nil, nil, nil, err
	}

	// Prepare the request
	pubraw, _ := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	req := &obcca.ECertCreateReq{
		Ts:  &protobuf.Timestamp{Seconds: time.Now().Unix(), Nanos: 0},
		Id:  &obcca.Identity{Id: id},
		Pw:  &obcca.Password{Pw: pw},
		Pub: &obcca.PublicKey{Type: obcca.CryptoType_ECDSA, Key: pubraw},
		Sig: nil}
	rawreq, _ := proto.Marshal(req)
	r, s, err := ecdsa.Sign(rand.Reader, priv, utils.Hash(rawreq))
	if err != nil {
		node.log.Error("Failed signing request [%s].", err.Error())

		return nil, nil, nil, err
	}
	R, _ := r.MarshalText()
	S, _ := s.MarshalText()
	req.Sig = &obcca.Signature{obcca.CryptoType_ECDSA, R, S}

	pbCert, key, err := node.callECACreateCertificate(context.Background(), req)
	if err != nil {
		node.log.Error("Failed requesting enrollment certificate [%s].", err.Error())

		return nil, nil, nil, err
	}

	node.log.Debug("Enrollment certificate hash [%s].", utils.EncodeBase64(utils.Hash(pbCert.Cert)))

	// Verify pbCert.Cert
	return priv, pbCert.Cert, key, nil
}

func (node *nodeImpl) getECACertificate() ([]byte, error) {
	// Prepare the request
	req := &obcca.ECertReadReq{&obcca.Identity{Id: "eca-root"}, nil}
	pbCert, err := node.callECAReadCACertificate(context.Background(), req)
	if err != nil {
		node.log.Error("Failed requesting enrollment certificate [%s].", err.Error())

		return nil, err
	}

	// TODO Verify pbCert.Cert

	return pbCert.Cert, nil
}
