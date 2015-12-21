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
	"crypto/x509"
	obcca "github.com/openblockchain/obc-peer/obc-ca/protos"
	"github.com/openblockchain/obc-peer/openchain/crypto/utils"
	"golang.org/x/net/context"
)

func (validator *validatorImpl) getEnrollmentCert(id []byte) (*x509.Certificate, error) {
	sid := utils.EncodeBase64(id)

	if cert := validator.enrollCerts[sid]; cert != nil {
		return cert, nil
	}

	// Retrieve from the DB or from the ECA in case
	rawCert, err := validator.peer.node.ks.GetEnrollmentCert(id, validator.getEnrollmentCertByHashFromECA)
	if err != nil {
		validator.peer.node.log.Error("Failed getting enrollment certificate for ", sid, err)
	}

	cert, err := utils.DERToX509Certificate(rawCert)
	if err != nil {
		validator.peer.node.log.Error("Failed parsing enrollment certificate for ", sid, utils.EncodeBase64(rawCert))
	}

	validator.enrollCerts[sid] = cert

	return cert, nil
}

func (validator *validatorImpl) getEnrollmentCertByHashFromECA(id []byte) ([]byte, error) {
	// Prepare the request
	validator.peer.node.log.Debug("Reading certificate for hash " + utils.EncodeBase64(id))

	req := &obcca.ECertReadReq{Id: &obcca.Identity{Id: ""}, Hash: id}
	pbCert, err := validator.peer.node.callECAReadCertificate(context.Background(), req)
	if err != nil {
		validator.peer.node.log.Error("Failed requesting enrollment certificate [%s].", err.Error())

		return nil, err
	}

	// TODO Verify pbCert.Cert
	return pbCert.Cert, nil
}
