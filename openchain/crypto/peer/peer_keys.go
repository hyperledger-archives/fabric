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

package peer

import (
	"crypto/ecdsa"
	"github.com/openblockchain/obc-peer/openchain/crypto/utils"
	"io/ioutil"
	"os"
)

func (peer *peerImpl) createKeyStorage() error {
	// Create directory
	return os.MkdirAll(peer.conf.getKeysPath(), 0755)
}

func (peer *peerImpl) isRegistered() bool {
	missing, _ := utils.FileMissing(peer.conf.getKeysPath(), peer.conf.getEnrollmentIDFilename())

	return !missing
}

func (peer *peerImpl) retrieveEnrollmentData(userId, pwd string) error {
	key, enrollCertRaw, err := peer.getEnrollmentCertificateFromECA(userId, pwd)
	if err != nil {
		peer.log.Error("Failed getting enrollment certificate %s", err)

		return err
	}
	peer.log.Info("Register:cert %s", utils.EncodeBase64(enrollCertRaw))
	//	validatorLogger.Info("Register:key %s", utils.EncodeBase64(key))

	// Verify

	cert, err := utils.DERToX509Certificate(enrollCertRaw)
	if err != nil {
		peer.log.Error("Failed parsing enrollment certificate: %s", err.Error())

		return err
	}

	pk := cert.PublicKey.(*ecdsa.PublicKey)
	err = utils.VerifySignCapability(key, pk)
	if err != nil {
		peer.log.Error("Failed checking enrollment certificate against enrollment key: %s", err.Error())

		return err
	}

	// TODO: store it in an encrypted form

	// Store enrollment  key
	peer.log.Info("Storing enrollment key and certificate for user [%s]...", userId)

	rawKey, err := utils.PrivateKeyToPEM("", key)
	if err != nil {
		peer.log.Error("Failed converting enrollment key to PEM: %s", err)
		return err
	}

	err = ioutil.WriteFile(peer.conf.getEnrollmentKeyPath(), rawKey, 0700)
	if err != nil {
		peer.log.Error("Failed storing enrollment key: %s", err)
		return err
	}

	// Store enrollment cert
	err = ioutil.WriteFile(peer.conf.getEnrollmentCertPath(), utils.DERCertToPEM(enrollCertRaw), 0700)
	if err != nil {
		peer.log.Error("Failed storing enrollment certificate: %s", err)
		return err
	}

	// Store enrollment id
	err = ioutil.WriteFile(peer.conf.getEnrollmentIDPath(), []byte(userId), 0700)
	if err != nil {
		peer.log.Error("Failed storing enrollment certificate: %s", err)
		return err
	}

	return nil
}

func (peer *peerImpl) loadEnrollmentKey(pwd []byte) error {
	peer.log.Info("Loading enrollment key at %s...", peer.conf.getEnrollmentKeyPath())

	rawEnrollPrivKey, err := ioutil.ReadFile(peer.conf.getEnrollmentKeyPath())
	if err != nil {
		peer.log.Error("Failed loading enrollment private key: %s", err.Error())

		return err
	}

	enrollPrivKey, err := utils.PEMtoPrivateKey(rawEnrollPrivKey, pwd)
	if err != nil {
		peer.log.Error("Failed parsing enrollment private key: %s", err.Error())

		return err
	}
	peer.enrollPrivKey = enrollPrivKey.(*ecdsa.PrivateKey)

	return nil
}

func (peer *peerImpl) loadEnrollmentCertificate() error {
	peer.log.Info("Loading enrollment certificate at %s...", peer.conf.getEnrollmentCertPath())

	pemEnrollCert, err := ioutil.ReadFile(peer.conf.getEnrollmentCertPath())
	if err != nil {
		peer.log.Error("Failed loading enrollment certificate: %s", err.Error())

		return err
	}

	enrollCert, rawEnrollCert, err := utils.PEMtoCertificateAndDER(pemEnrollCert)
	if err != nil {
		peer.log.Error("Failed parsing enrollment certificate: %s", err.Error())

		return err
	}
	peer.enrollCert = enrollCert

	pk := peer.enrollCert.PublicKey.(*ecdsa.PublicKey)
	err = utils.VerifySignCapability(peer.enrollPrivKey, pk)
	if err != nil {
		peer.log.Error("Failed checking enrollment certificate against enrollment key: %s", err.Error())

		return err
	}

	// Set validator ID
	peer.id = utils.Hash(rawEnrollCert)
	peer.log.Info("Setting id to [%s]", utils.EncodeBase64(peer.id))

	return nil
}

func (peer *peerImpl) loadEnrollmentID() error {
	peer.log.Info("Loading enrollment id at %s...", peer.conf.getEnrollmentIDPath())

	enrollID, err := ioutil.ReadFile(peer.conf.getEnrollmentIDPath())
	if err != nil {
		peer.log.Error("Failed loading enrollment id: %s", err.Error())

		return err
	}

	// Set enrollment ID
	peer.enrollId = string(enrollID)
	peer.log.Info("Setting enrollment id to [%s]", peer.enrollId)

	return nil
}
