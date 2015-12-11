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
	"os"
)

func (node *nodeImpl) createKeyStorage() error {
	// Create directory
	return os.MkdirAll(node.conf.getKeysPath(), 0755)
}

func (node *nodeImpl) isRegistered() bool {
	missing, _ := utils.FileMissing(node.conf.getKeysPath(), node.conf.getEnrollmentIDFilename())

	return !missing
}

func (node *nodeImpl) retrieveEnrollmentData(userID, pwd string) error {
	key, enrollCertRaw, err := node.getEnrollmentCertificateFromECA(userID, pwd)
	if err != nil {
		node.log.Error("Failed getting enrollment certificate %s", err)

		return err
	}
	node.log.Info("Register:cert %s", utils.EncodeBase64(enrollCertRaw))
	//	nodeLogger.Info("Register:key %s", utils.EncodeBase64(key))

	// Verify

	cert, err := utils.DERToX509Certificate(enrollCertRaw)
	if err != nil {
		node.log.Error("Failed parsing enrollment certificate: %s", err.Error())

		return err
	}

	pk := cert.PublicKey.(*ecdsa.PublicKey)
	err = utils.VerifySignCapability(key, pk)
	if err != nil {
		node.log.Error("Failed checking enrollment certificate against enrollment key: %s", err.Error())

		return err
	}

	// TODO: store it in an encrypted form

	// Store enrollment  key
	node.log.Info("Storing enrollment key and certificate for user [%s]...", userID)

	rawKey, err := utils.PrivateKeyToPEM("", key)
	if err != nil {
		node.log.Error("Failed converting enrollment key to PEM: %s", err)
		return err
	}

	err = ioutil.WriteFile(node.conf.getEnrollmentKeyPath(), rawKey, 0700)
	if err != nil {
		node.log.Error("Failed storing enrollment key: %s", err)
		return err
	}

	// Store enrollment cert
	err = ioutil.WriteFile(node.conf.getEnrollmentCertPath(), utils.DERCertToPEM(enrollCertRaw), 0700)
	if err != nil {
		node.log.Error("Failed storing enrollment certificate: %s", err)
		return err
	}

	// Store enrollment id
	err = ioutil.WriteFile(node.conf.getEnrollmentIDPath(), []byte(userID), 0700)
	if err != nil {
		node.log.Error("Failed storing enrollment certificate: %s", err)
		return err
	}

	return nil
}

func (node *nodeImpl) loadEnrollmentKey(pwd []byte) error {
	node.log.Info("Loading enrollment key at %s...", node.conf.getEnrollmentKeyPath())

	rawEnrollPrivKey, err := ioutil.ReadFile(node.conf.getEnrollmentKeyPath())
	if err != nil {
		node.log.Error("Failed loading enrollment private key: %s", err.Error())

		return err
	}

	enrollPrivKey, err := utils.PEMtoPrivateKey(rawEnrollPrivKey, pwd)
	if err != nil {
		node.log.Error("Failed parsing enrollment private key: %s", err.Error())

		return err
	}
	node.enrollPrivKey = enrollPrivKey.(*ecdsa.PrivateKey)

	return nil
}

func (node *nodeImpl) loadEnrollmentCertificate() error {
	node.log.Info("Loading enrollment certificate at %s...", node.conf.getEnrollmentCertPath())

	pemEnrollCert, err := ioutil.ReadFile(node.conf.getEnrollmentCertPath())
	if err != nil {
		node.log.Error("Failed loading enrollment certificate: %s", err.Error())

		return err
	}

	enrollCert, rawEnrollCert, err := utils.PEMtoCertificateAndDER(pemEnrollCert)
	if err != nil {
		node.log.Error("Failed parsing enrollment certificate: %s", err.Error())

		return err
	}
	node.enrollCert = enrollCert

	pk := node.enrollCert.PublicKey.(*ecdsa.PublicKey)
	err = utils.VerifySignCapability(node.enrollPrivKey, pk)
	if err != nil {
		node.log.Error("Failed checking enrollment certificate against enrollment key: %s", err.Error())

		return err
	}

	// Set node ID
	node.id = utils.Hash(rawEnrollCert)
	node.log.Info("Setting id to [%s]", utils.EncodeBase64(node.id))

	return nil
}

func (node *nodeImpl) loadEnrollmentID() error {
	node.log.Info("Loading enrollment id at %s...", node.conf.getEnrollmentIDPath())

	enrollID, err := ioutil.ReadFile(node.conf.getEnrollmentIDPath())
	if err != nil {
		node.log.Error("Failed loading enrollment id: %s", err.Error())

		return err
	}

	// Set enrollment ID
	node.enrollID = string(enrollID)
	node.log.Info("Setting enrollment id to [%s]", node.enrollID)

	return nil
}
