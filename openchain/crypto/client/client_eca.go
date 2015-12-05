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

package client

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/x509"
	obcca "github.com/openblockchain/obc-peer/obcca/protos"
	protobuf "google/protobuf"
	"time"

	"errors"
	"github.com/golang/protobuf/proto"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"io/ioutil"
	"github.com/openblockchain/obc-peer/openchain/crypto/utils"
)

func (client *Client) retrieveECACertsChain(userId string) error {
	// Retrieve ECA certificate and verify it
	ecaCertRaw, err := client.getECACertificate()
	if err != nil {
		log.Error("Failed getting ECA certificate %s", err)

		return err
	}
	log.Info("Storing ECAcert [%s]", utils.EncodeBase64(ecaCertRaw))

	// TODO: Test ECA cert againt root CA
	_, err = utils.DERToX509Certificate(ecaCertRaw)
	if err != nil {
		log.Error("Failed parsing ECA certificate %s", err)

		return err
	}

	// Store ECA cert
	log.Info("Storing ECA certificate for [%s]...", userId)

	err = ioutil.WriteFile(getECACertsChainPath(), utils.DERCertToPEM(ecaCertRaw), 0700)
	if err != nil {
		log.Error("Failed storing eca certificate: %s", err)
		return err
	}

	return nil
}

func (client *Client) loadECACertsChain() error {
	log.Info("Loading ECA certificates chain at %s...", getECACertsChainPath())

	chain, err := ioutil.ReadFile(getECACertsChainPath())
	if err != nil {
		log.Error("Failed loading ECA certificates chain : %s", err.Error())

		return err
	}

	ok := client.rootsCertPool.AppendCertsFromPEM(chain)
	if !ok {
		log.Error("Failed appending ECA certificates chain.")

		return errors.New("Failed appending ECA certificates chain.")
	}

	return nil
}

func (client *Client) callECACreateCertificate(ctx context.Context, in *obcca.ECertCreateReq, opts ...grpc.CallOption) (*obcca.Cert, error) {
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

func (client *Client) callECAReadCertificate(ctx context.Context, in *obcca.ECertReadReq, opts ...grpc.CallOption) (*obcca.Cert, error) {
	sockP, err := grpc.Dial(getECAPAddr(), grpc.WithInsecure())
	if err != nil {
		log.Error("Failed dailing key: %s", err)

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

func (client *Client) getEnrollmentCertificateFromECA(id, pw string) (interface{}, []byte, error) {
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
		&obcca.PublicKey{
			Type: obcca.CryptoType_ECDSA,
			Key:  pubraw,
		}, nil}
	rawreq, _ := proto.Marshal(req)
	r, s, err := ecdsa.Sign(rand.Reader, priv, utils.Hash(rawreq))
	if err != nil {
		panic(err)
	}
	R, _ := r.MarshalText()
	S, _ := s.MarshalText()
	req.Sig = &obcca.Signature{obcca.CryptoType_ECDSA, R, S}

	pbCert, err := client.callECACreateCertificate(context.Background(), req)
	if err != nil {
		log.Error("Failed requesting enrollment certificate: %s", err)

		return nil, nil, err
	}

	log.Info("Verifing enrollment certificate...")

	enrollCert, err := utils.DERToX509Certificate(pbCert.Cert)
	certPK := enrollCert.PublicKey.(*ecdsa.PublicKey)
	utils.VerifySignCapability(priv, certPK)

	log.Info("Verifing enrollment certificate...done!")

	// Verify pbCert.Cert
	return priv, pbCert.Cert, nil
}

func (client *Client) getECACertificate() ([]byte, error) {
	// Prepare the request
	req := &obcca.ECertReadReq{&obcca.Identity{Id: "eca-root"}, nil}
	pbCert, err := client.callECAReadCertificate(context.Background(), req)
	if err != nil {
		log.Error("Failed requesting eca certificate: %s", err)

		return nil, err
	}

	// TODO Verify pbCert.Cert

	return pbCert.Cert, nil
}
