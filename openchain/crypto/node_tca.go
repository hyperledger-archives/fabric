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
	obcca "github.com/openblockchain/obc-peer/obc-ca/protos"

	"errors"
	"github.com/openblockchain/obc-peer/openchain/crypto/utils"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google/protobuf"
	"io/ioutil"
	"time"
)

func (node *nodeImpl) retrieveTCACertsChain(userID string) error {
	// Retrieve TCA certificate and verify it
	tcaCertRaw, err := node.getTCACertificate()
	if err != nil {
		node.log.Error("Failed getting TCA certificate [%s].", err.Error())

		return err
	}
	node.log.Debug("TCA certificate [%s]", utils.EncodeBase64(tcaCertRaw))

	// TODO: Test TCA cert againt root CA
	_, err = utils.DERToX509Certificate(tcaCertRaw)
	if err != nil {
		node.log.Error("Failed parsing TCA certificate [%s].", err.Error())

		return err
	}

	// Store TCA cert
	node.log.Debug("Storing TCA certificate for validator [%s]...", userID)

	err = ioutil.WriteFile(node.conf.getTCACertsChainPath(), utils.DERCertToPEM(tcaCertRaw), 0700)
	if err != nil {
		node.log.Error("Failed storing tca certificate [%s].", err.Error())
		return err
	}

	return nil
}

func (node *nodeImpl) loadTCACertsChain() error {
	// Load TCA certs chain
	node.log.Debug("Loading TCA certificates chain at [%s]...", node.conf.getTCACertsChainPath())

	chain, err := ioutil.ReadFile(node.conf.getTCACertsChainPath())
	if err != nil {
		node.log.Error("Failed loading TCA certificates chain [%s].", err.Error())

		return err
	}

	ok := node.rootsCertPool.AppendCertsFromPEM(chain)
	if !ok {
		node.log.Error("Failed appending TCA certificates chain.")

		return errors.New("Failed appending TCA certificates chain.")
	}

	return nil
}

func (node *nodeImpl) callTCAReadCertificate(ctx context.Context, in *obcca.TCertReadReq, opts ...grpc.CallOption) (*obcca.Cert, error) {
	sockP, err := grpc.Dial(node.conf.getTCAPAddr(), grpc.WithInsecure())
	if err != nil {
		node.log.Error("Failed tca dial in [%s].", err.Error())

		return nil, err
	}
	defer sockP.Close()

	tcaP := obcca.NewTCAPClient(sockP)

	cert, err := tcaP.ReadCertificate(context.Background(), in)
	if err != nil {
		node.log.Error("Failed requesting tca read certificate [%s].", err.Error())

		return nil, err
	}

	return cert, nil
}

func (node *nodeImpl) getTCACertificate() ([]byte, error) {
	// Prepare the request
	now := time.Now()
	timestamp := google_protobuf.Timestamp{int64(now.Second()), int32(now.Nanosecond())}
	req := &obcca.TCertReadReq{Ts: &timestamp, Id: &obcca.Identity{Id: "tca-root"}, Sig: nil}
	pbCert, err := node.callTCAReadCertificate(context.Background(), req)
	if err != nil {
		node.log.Error("Failed requesting tca certificate [%s].", err.Error())

		return nil, err
	}

	// TODO Verify pbCert.Cert

	return pbCert.Cert, nil
}
