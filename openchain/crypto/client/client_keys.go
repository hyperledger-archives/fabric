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

func (client *Client) createKeyStorage() error {
	// Create directory
	return os.MkdirAll(getKeysPath(), 0755)
}

func (client *Client) retrieveEnrollmentData(userId, pwd string) error {
	key, enrollCertRaw, enrollChainKey, err := client.getEnrollmentCertificateFromECA(userId, pwd)
	if err != nil {
		log.Error("Failed getting enrollment certificate [id=%s] %s", userId, err)

		return err
	}
	log.Info("Register:cert %s", utils.EncodeBase64(enrollCertRaw))
	//	validatorLogger.Info("Register:key %s", utils.EncodeBase64(key))

	// Store enrollment  key
	log.Info("Storing enrollment key and certificate for user [%s]...", userId)

	rawKey, err := utils.PrivateKeyToPEM(key)
	if err != nil {
		log.Error("Failed converting enrollment key to PEM [id=%s]: %s", userId, err)
		return err
	}

	err = ioutil.WriteFile(getEnrollmentKeyPath(), rawKey, 0700)
	if err != nil {
		log.Error("Failed storing enrollment key [id=%s]: %s", userId, err)
		return err
	}

	// Store enrollment cert
	err = ioutil.WriteFile(getEnrollmentCertPath(), utils.DERCertToPEM(enrollCertRaw), 0700)
	if err != nil {
		log.Error("Failed storing enrollment certificate [id=%s]: %s", userId, err)
		return err
	}

	// Store enrollment id
	err = ioutil.WriteFile(getEnrollmentIDPath(), []byte(userId), 0700)
	if err != nil {
		log.Error("Failed storing enrollment certificate [id=%s]: %s", userId, err)
		return err
	}

	// Store enrollment chain key
	log.Info("Storing chain key %s", utils.EncodeBase64(enrollChainKey))
	err = ioutil.WriteFile(getEnrollmentChainKeyPath(), utils.AEStoPEM(enrollChainKey), 0700)
	if err != nil {
		log.Error("Failed storing enrollment chain key [id=%s]: %s", userId, err)
		return err
	}

	return nil
}

func (client *Client) loadEnrollmentKey(pwd []byte) error {
	log.Info("Loading enrollment key at %s...", getEnrollmentKeyPath())

	rawEnrollPrivKey, err := ioutil.ReadFile(getEnrollmentKeyPath())
	if err != nil {
		log.Error("Failed loading enrollment private key: %s", err.Error())

		return err
	}

	enrollPrivKey, err := utils.PEMtoPrivateKey(rawEnrollPrivKey, pwd)
	if err != nil {
		log.Error("Failed parsing enrollment private key: %s", err.Error())

		return err
	}
	client.enrollPrivKey = enrollPrivKey.(*ecdsa.PrivateKey)

	return nil
}

func (client *Client) loadEnrollmentCertificate() error {
	log.Info("Loading enrollment certificate at %s...", getEnrollmentCertPath())

	pemEnrollCert, err := ioutil.ReadFile(getEnrollmentCertPath())
	if err != nil {
		log.Error("Failed loading enrollment certificate: %s", err.Error())

		return err
	}

	enrollCert, rawEnrollCert, err := utils.PEMtoCertificateAndDER(pemEnrollCert)
	if err != nil {
		log.Error("Failed parsing enrollment certificate: %s", err.Error())

		return err
	}
	client.enrollCert = enrollCert

	// Set ID
	client.id = utils.Hash(rawEnrollCert)
	log.Info("Setting id to [%s]", utils.EncodeBase64(client.id))

	pk := client.enrollCert.PublicKey.(*ecdsa.PublicKey)
	err = utils.VerifySignCapability(client.enrollPrivKey, pk)
	if err != nil {
		log.Error("Failed checking enrollment certificate against enrollment key: %s", err.Error())

		return err
	}

	return nil
}

func (client *Client) loadEnrollmentID() error {
	log.Info("Loading enrollment id at %s...", getEnrollmentIDPath())

	enrollID, err := ioutil.ReadFile(getEnrollmentIDPath())
	if err != nil {
		log.Error("Failed loading enrollment id: %s", err.Error())

		return err
	}

	// Set enrollment ID
	client.enrollId = string(enrollID)
	log.Info("Setting enrollment id to [%s]", client.enrollId)

	return nil
}

func (client *Client) loadEnrollmentChainKey(pwd []byte) error {
	log.Info("Loading enrollment chain key at %s...", getEnrollmentChainKeyPath())

	pem, err := ioutil.ReadFile(getEnrollmentChainKeyPath())
	if err != nil {
		log.Error("Failed loading enrollment chain key: %s", err.Error())

		return err
	}

	enrollChainKey, err := utils.PEMtoAES(pem, pwd)
	if err != nil {
		log.Error("Failed parsing enrollment chain  key: %s", err.Error())

		return err
	}
	client.enrollChainKey = enrollChainKey

	return nil

}
