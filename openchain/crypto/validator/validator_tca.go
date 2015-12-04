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
	obcca "github.com/openblockchain/obc-peer/obcca/protos"

	"errors"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google/protobuf"
	"io/ioutil"
	"github.com/openblockchain/obc-peer/openchain/crypto/utils"
	"time"
)

func (validator *Validator) retrieveTCACertsChain(userId string) error {
	// Retrieve TCA certificate and verify it
	tcaCertRaw, err := validator.getTCACertificate()
	if err != nil {
		log.Error("Failed getting TCA certificate %s", err)

		return err
	}
	log.Info("Register:TCAcert %s", utils.EncodeBase64(tcaCertRaw))

	// TODO: Test TCA cert againt root CA
	_, err = utils.DERToX509Certificate(tcaCertRaw)
	if err != nil {
		log.Error("Failed parsing TCA certificate %s", err)

		return err
	}

	// Store TCA cert
	log.Info("Storing TCA certificate for validator [%s]...", userId)

	err = ioutil.WriteFile(getTCACertsChainPath(), utils.DERCertToPEM(tcaCertRaw), 0700)
	if err != nil {
		log.Error("Failed storing tca certificate: %s", err)
		return err
	}

	return nil
}

func (validator *Validator) loadTCACertsChain() error {
	// Load TCA certs chain
	log.Info("Loading TCA certificates chain at %s...", getTCACertsChainPath())

	chain, err := ioutil.ReadFile(getTCACertsChainPath())
	if err != nil {
		log.Error("Failed loading TCA certificates chain : %s", err.Error())

		return err
	}

	ok := validator.rootsCertPool.AppendCertsFromPEM(chain)
	if !ok {
		log.Error("Failed appending TCA certificates chain.")

		return errors.New("Failed appending TCA certificates chain.")
	}

	return nil
}

func (validator *Validator) callTCAReadCertificate(ctx context.Context, in *obcca.TCertReadReq, opts ...grpc.CallOption) (*obcca.Cert, error) {
	sockP, err := grpc.Dial(getTCAPAddr(), grpc.WithInsecure())
	if err != nil {
		log.Error("Failed tca dial in: %s", err)

		return nil, err
	}
	defer sockP.Close()

	tcaP := obcca.NewTCAPClient(sockP)

	cert, err := tcaP.ReadCertificate(context.Background(), in)
	if err != nil {
		log.Error("Failed requesting tca read certificate: %s", err)

		return nil, err
	}

	return cert, nil
}

func (validator *Validator) getTCACertificate() ([]byte, error) {
	log.Info("getTCACertificate...")

	// Prepare the request
	now := time.Now()
	timestamp := google_protobuf.Timestamp{int64(now.Second()), int32(now.Nanosecond())}
	req := &obcca.TCertReadReq{&timestamp, &obcca.Identity{Id: "tca-root"}, nil}
	pbCert, err := validator.callTCAReadCertificate(context.Background(), req)
	if err != nil {
		log.Error("Failed requesting tca certificate: %s", err)

		return nil, err
	}

	// TODO Verify pbCert.Cert

	log.Info("getTCACertificate...done!")

	return pbCert.Cert, nil
}
