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
	"github.com/openblockchain/obc-peer/openchain/crypto/utils"
	"io/ioutil"
)

func (node *nodeImpl) isRegistered() bool {
	missing, _ := utils.FileMissing(node.conf.getKeysPath(), node.conf.getEnrollmentIDFilename())

	return !missing
}

func (node *nodeImpl) retrieveEnrollmentData(enrollID, enrollPWD string, ksPWD []byte) error {
	key, enrollCertRaw, enrollChainKey, err := node.getEnrollmentCertificateFromECA(enrollID, enrollPWD)
	if err != nil {
		node.log.Error("Failed getting enrollment certificate [id=%s]: [%s]", enrollID, err)

		return err
	}
	node.log.Debug("Enrollment certificate [%s].", utils.EncodeBase64(enrollCertRaw))

	node.log.Debug("Storing enrollment data for user [%s]...", enrollID)

	// Store enrollment id
	err = ioutil.WriteFile(node.conf.getEnrollmentIDPath(), []byte(enrollID), 0700)
	if err != nil {
		node.log.Error("Failed storing enrollment certificate [id=%s]: [%s]", enrollID, err)
		return err
	}

	// Store enrollment key
	if err := node.ks.storePrivateKey(node.conf.getEnrollmentKeyFilename(), key, ksPWD); err != nil {
		node.log.Error("Failed storing enrollment key [id=%s]: [%s]", enrollID, err)
		return err
	}

	// Store enrollment cert
	if err := node.ks.storeCert(node.conf.getEnrollmentCertFilename(), enrollCertRaw, ksPWD); err != nil {
		node.log.Error("Failed storing enrollment certificate [id=%s]: [%s]", enrollID, err)
		return err
	}

	// Store enrollment chain key
	if err := node.ks.storeKey(node.conf.getEnrollmentChainKeyFilename(), enrollChainKey, ksPWD); err != nil {
		node.log.Error("Failed storing enrollment chain key [id=%s]: [%s]", enrollID, err)
		return err
	}

	return nil
}

func (node *nodeImpl) loadEnrollmentKey(pwd []byte) error {
	node.log.Debug("Loading enrollment key at [%s]...", node.conf.getEnrollmentKeyPath())

	enrollPrivKey, err := node.ks.loadPrivateKey(node.conf.getEnrollmentKeyFilename(), pwd)
	if err != nil {
		node.log.Error("Failed loading enrollment private key [%s].", err.Error())

		return err
	}

	node.enrollPrivKey = enrollPrivKey.(*ecdsa.PrivateKey)

	return nil
}

func (node *nodeImpl) loadEnrollmentCertificate() error {
	node.log.Debug("Loading enrollment certificate at [%s]...", node.conf.getEnrollmentCertPath())

	cert, der, err := node.ks.loadCertX509AndDer(node.conf.getEnrollmentCertFilename(), nil)
	if err != nil {
		node.log.Error("Failed parsing enrollment certificate [%s].", err.Error())

		return err
	}
	node.enrollCert = cert

	// TODO: move this to retrieve
	pk := node.enrollCert.PublicKey.(*ecdsa.PublicKey)
	err = utils.VerifySignCapability(node.enrollPrivKey, pk)
	if err != nil {
		node.log.Error("Failed checking enrollment certificate against enrollment key [%s].", err.Error())

		return err
	}

	// Set node ID
	node.id = utils.Hash(der)
	node.log.Debug("Setting id to [%s].", utils.EncodeBase64(node.id))

	// Set eCertHash
	node.enrollCertHash = utils.Hash(der)
	node.log.Debug("Setting enrollCertHash to [%s].", utils.EncodeBase64(node.enrollCertHash))

	return nil
}

func (node *nodeImpl) loadEnrollmentID() error {
	node.log.Debug("Loading enrollment id at [%s]...", node.conf.getEnrollmentIDPath())

	enrollID, err := ioutil.ReadFile(node.conf.getEnrollmentIDPath())
	if err != nil {
		node.log.Error("Failed loading enrollment id [%s].", err.Error())

		return err
	}

	// Set enrollment ID
	node.enrollID = string(enrollID)
	node.log.Debug("Setting enrollment id to [%s].", node.enrollID)

	return nil
}

func (node *nodeImpl) retrieveTLSCertificate(id, affiliation string, ksPWD []byte) error {
	key, tlsCertRaw, err := node.getTLSCertificateFromTLSCA(id, affiliation)
	if err != nil {
		node.log.Error("Failed getting tls certificate [id=%s] %s", id, err)

		return err
	}
	node.log.Debug("TLS Cert [%s]", utils.EncodeBase64(tlsCertRaw))

	node.log.Info("Storing TLS key and certificate for user [%s]...", id)

	// Store tls key
	if err := node.ks.storePrivateKey(node.conf.getTLSKeyFilename(), key, ksPWD); err != nil {
		node.log.Error("Failed storing tls key [id=%s]: %s", id, err)
		return err
	}

	// Store tls cert
	if err := node.ks.storeCert(node.conf.getTLSCertFilename(), utils.DERCertToPEM(tlsCertRaw), ksPWD); err != nil {
		node.log.Error("Failed storing tls certificate [id=%s]: %s", id, err)
		return err
	}

	return nil
}

func (node *nodeImpl) loadEnrollmentChainKey(pwd []byte) error {
	node.log.Debug("Loading enrollment chain key at [%s]...", node.conf.getEnrollmentChainKeyPath())

	enrollChainKey, err := node.ks.loadKey(node.conf.getEnrollmentChainKeyFilename(), pwd)
	if err != nil {
		node.log.Error("Failed loading enrollment chain key [%s].", err.Error())

		return err
	}
	node.enrollChainKey = enrollChainKey

	return nil
}
