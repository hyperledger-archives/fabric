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
	"github.com/openblockchain/obc-peer/openchain/crypto/utils"
	"io/ioutil"
	"os"
)

func (client *clientImpl) createKeyStorage() error {
	// Create directory
	return os.MkdirAll(client.conf.getKeysPath(), 0755)
}

func (client *clientImpl) isRegistered() bool {
	missing, _ := utils.FileMissing(client.conf.getKeysPath(), client.conf.getEnrollmentIDFilename())

	return !missing
}

func (client *clientImpl) retrieveEnrollmentData(userId, pwd string) error {
	key, enrollCertRaw, err := client.getEnrollmentCertificateFromECA(userId, pwd)
	if err != nil {
		client.log.Error("Failed getting enrollment certificate [id=%s] %s", userId, err)

		return err
	}
	client.log.Info("Register:cert %s", utils.EncodeBase64(enrollCertRaw))
	//	validatorLogger.Info("Register:key %s", utils.EncodeBase64(key))

	// Store enrollment  key
	client.log.Info("Storing enrollment key and certificate for user [%s]...", userId)

	rawKey, err := utils.PrivateKeyToPEM("", key)
	if err != nil {
		client.log.Error("Failed converting enrollment key to PEM [id=%s]: %s", userId, err)
		return err
	}

	err = ioutil.WriteFile(client.conf.getEnrollmentKeyPath(), rawKey, 0700)
	if err != nil {
		client.log.Error("Failed storing enrollment key [id=%s]: %s", userId, err)
		return err
	}

	// Store enrollment cert
	err = ioutil.WriteFile(client.conf.getEnrollmentCertPath(), utils.DERCertToPEM(enrollCertRaw), 0700)
	if err != nil {
		client.log.Error("Failed storing enrollment certificate [id=%s]: %s", userId, err)
		return err
	}

	// Store enrollment id
	err = ioutil.WriteFile(client.conf.getEnrollmentIDPath(), []byte(userId), 0700)
	if err != nil {
		client.log.Error("Failed storing enrollment certificate [id=%s]: %s", userId, err)
		return err
	}

	return nil
}

func (client *clientImpl) loadEnrollmentKey(pwd []byte) error {
	client.log.Info("Loading enrollment key at %s...", client.conf.getEnrollmentKeyPath())

	rawEnrollPrivKey, err := ioutil.ReadFile(client.conf.getEnrollmentKeyPath())
	if err != nil {
		client.log.Error("Failed loading enrollment private key: %s", err.Error())

		return err
	}

	enrollPrivKey, err := utils.PEMtoPrivateKey(rawEnrollPrivKey, pwd)
	if err != nil {
		client.log.Error("Failed parsing enrollment private key: %s", err.Error())

		return err
	}
	client.enrollPrivKey = enrollPrivKey.(*ecdsa.PrivateKey)

	return nil
}

func (client *clientImpl) loadEnrollmentCertificate() error {
	client.log.Info("Loading enrollment certificate at %s...", client.conf.getEnrollmentCertPath())

	pemEnrollCert, err := ioutil.ReadFile(client.conf.getEnrollmentCertPath())
	if err != nil {
		client.log.Error("Failed loading enrollment certificate: %s", err.Error())

		return err
	}

	enrollCert, rawEnrollCert, err := utils.PEMtoCertificateAndDER(pemEnrollCert)
	if err != nil {
		client.log.Error("Failed parsing enrollment certificate: %s", err.Error())

		return err
	}
	client.enrollCert = enrollCert

	// Set ID
	client.id = utils.Hash(rawEnrollCert)
	client.log.Info("Setting id to [%s]", utils.EncodeBase64(client.id))

	pk := client.enrollCert.PublicKey.(*ecdsa.PublicKey)
	err = utils.VerifySignCapability(client.enrollPrivKey, pk)
	if err != nil {
		client.log.Error("Failed checking enrollment certificate against enrollment key: %s", err.Error())

		return err
	}

	return nil
}

func (client *clientImpl) loadEnrollmentID() error {
	client.log.Info("Loading enrollment id at %s...", client.conf.getEnrollmentIDPath())

	enrollID, err := ioutil.ReadFile(client.conf.getEnrollmentIDPath())
	if err != nil {
		client.log.Error("Failed loading enrollment id: %s", err.Error())

		return err
	}

	// Set enrollment ID
	client.enrollId = string(enrollID)
	client.log.Info("Setting enrollment id to [%s]", client.enrollId)

	return nil
}
