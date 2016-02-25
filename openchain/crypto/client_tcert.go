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
	"github.com/openblockchain/obc-peer/openchain/crypto/utils"
)

type tCert interface {
	GetCertificate() *x509.Certificate

	Sign(msg []byte) ([]byte, error)

	Verify(signature, msg []byte) error
}

type tCertImpl struct {
	client *clientImpl
	cert   *x509.Certificate
	sk     interface{}
}

func (tCert *tCertImpl) GetCertificate() *x509.Certificate {
	return tCert.cert
}

func (tCert *tCertImpl) Sign(msg []byte) ([]byte, error) {
	if tCert.sk == nil {
		return nil, utils.ErrNilArgument
	}

	return tCert.client.sign(tCert.sk, msg)
}

func (tCert *tCertImpl) Verify(signature, msg []byte) (err error) {
	ok, err := tCert.client.verify(tCert.cert.PublicKey, msg, signature)
	if err != nil {
		return
	}
	if !ok {
		return utils.ErrInvalidSignature
	}
	return
}
