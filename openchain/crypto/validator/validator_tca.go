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
	"github.com/openblockchain/obc-peer/openchain/crypto/utils"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google/protobuf"
	"io/ioutil"
	"time"
)

func (validator *validatorImpl) retrieveTCACertsChain(userID string) error {
	// Retrieve TCA certificate and verify it
	tcaCertRaw, err := validator.getTCACertificate()
	if err != nil {
		validator.log.Error("Failed getting TCA certificate %s", err)

		return err
	}
	validator.log.Info("Register:TCAcert %s", utils.EncodeBase64(tcaCertRaw))

	// TODO: Test TCA cert againt root CA
	_, err = utils.DERToX509Certificate(tcaCertRaw)
	if err != nil {
		validator.log.Error("Failed parsing TCA certificate %s", err)

		return err
	}

	// Store TCA cert
	validator.log.Info("Storing TCA certificate for validator [%s]...", userID)

	err = ioutil.WriteFile(validator.conf.getTCACertsChainPath(), utils.DERCertToPEM(tcaCertRaw), 0700)
	if err != nil {
		validator.log.Error("Failed storing tca certificate: %s", err)
		return err
	}

	return nil
}

func (validator *validatorImpl) loadTCACertsChain() error {
	// Load TCA certs chain
	validator.log.Info("Loading TCA certificates chain at %s...", validator.conf.getTCACertsChainPath())

	chain, err := ioutil.ReadFile(validator.conf.getTCACertsChainPath())
	if err != nil {
		validator.log.Error("Failed loading TCA certificates chain : %s", err.Error())

		return err
	}

	ok := validator.rootsCertPool.AppendCertsFromPEM(chain)
	if !ok {
		validator.log.Error("Failed appending TCA certificates chain.")

		return errors.New("Failed appending TCA certificates chain.")
	}

	return nil
}

func (validator *validatorImpl) callTCAReadCertificate(ctx context.Context, in *obcca.TCertReadReq, opts ...grpc.CallOption) (*obcca.Cert, error) {
	sockP, err := grpc.Dial(validator.conf.getTCAPAddr(), grpc.WithInsecure())
	if err != nil {
		validator.log.Error("Failed tca dial in: %s", err)

		return nil, err
	}
	defer sockP.Close()

	tcaP := obcca.NewTCAPClient(sockP)

	cert, err := tcaP.ReadCertificate(context.Background(), in)
	if err != nil {
		validator.log.Error("Failed requesting tca read certificate: %s", err)

		return nil, err
	}

	return cert, nil
}

func (validator *validatorImpl) getTCACertificate() ([]byte, error) {
	validator.log.Info("getTCACertificate...")

	// Prepare the request
	now := time.Now()
	timestamp := google_protobuf.Timestamp{int64(now.Second()), int32(now.Nanosecond())}
	req := &obcca.TCertReadReq{&timestamp, &obcca.Identity{Id: "tca-root"}, nil}
	pbCert, err := validator.callTCAReadCertificate(context.Background(), req)
	if err != nil {
		validator.log.Error("Failed requesting tca certificate: %s", err)

		return nil, err
	}

	// TODO Verify pbCert.Cert

	validator.log.Info("getTCACertificate...done!")

	return pbCert.Cert, nil
}
