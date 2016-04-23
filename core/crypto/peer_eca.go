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
	"fmt"
	"github.com/hyperledger/fabric/core/crypto/utils"
	membersrvc "github.com/hyperledger/fabric/membersrvc/protos"
	"golang.org/x/net/context"
	"strconv"
)

func (peer *peerImpl) getEnrollmentCert(id []byte) (*x509.Certificate, error) {
	if len(id) == 0 {
		return nil, fmt.Errorf("Invalid peer id. It is empty.")
	}

	sid := utils.EncodeBase64(id)

	peer.debug("Getting enrollment certificate for [%s]", sid)

	if cert := peer.enrollCerts[sid]; cert != nil {
		peer.debug("Enrollment certificate for [%s] already in memory.", sid)
		return cert, nil
	}

	// Retrieve from the DB or from the ECA in case
	peer.debug("Retrieve Enrollment certificate for [%s]...", sid)
	rawCert, err := peer.ks.GetSignEnrollmentCert(id, peer.getEnrollmentCertByHashFromECA)
	if err != nil {
		peer.error("Failed getting enrollment certificate for [%s]: [%s]", sid, err)

		return nil, err
	}

	cert, err := utils.DERToX509Certificate(rawCert)
	if err != nil {
		peer.error("Failed parsing enrollment certificate for [%s]: [% x],[% x]", sid, rawCert, err)

		return nil, err
	}

	peer.enrollCerts[sid] = cert

	return cert, nil
}

func (peer *peerImpl) getEnrollmentCertByHashFromECA(id []byte) ([]byte, []byte, error) {
	// Prepare the request
	peer.debug("Reading certificate for hash [% x]", id)

	req := &membersrvc.Hash{Hash: id}
	responce, err := peer.callECAReadCertificateByHash(context.Background(), req)
	if err != nil {
		peer.error("Failed requesting enrollment certificate [%s].", err.Error())

		return nil, nil, err
	}

	peer.debug("Certificate for hash [% x] = [% x][% x]", id, responce.Sign, responce.Enc)

	// Verify responce.Sign
	x509Cert, err := utils.DERToX509Certificate(responce.Sign)
	if err != nil {
		peer.error("Failed parsing signing enrollment certificate for encrypting: [%s]", err)

		return nil, nil, err
	}

	// Check role
	roleRaw, err := utils.GetCriticalExtension(x509Cert, ECertSubjectRole)
	if err != nil {
		peer.error("Failed parsing ECertSubjectRole in enrollment certificate for signing: [%s]", err)

		return nil, nil, err
	}

	role, err := strconv.ParseInt(string(roleRaw), 10, len(roleRaw)*8)
	if err != nil {
		peer.error("Failed parsing ECertSubjectRole in enrollment certificate for signing: [%s]", err)

		return nil, nil, err
	}

	if membersrvc.Role(role) != membersrvc.Role_VALIDATOR && membersrvc.Role(role) != membersrvc.Role_PEER {
		peer.error("Invalid ECertSubjectRole in enrollment certificate for signing. Not a validator or peer: [%s]", err)

		return nil, nil, err
	}

	return responce.Sign, responce.Enc, nil
}
