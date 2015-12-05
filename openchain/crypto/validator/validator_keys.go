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
	"crypto/ecdsa"
	"io/ioutil"
	"github.com/openblockchain/obc-peer/openchain/crypto/utils"
	"os"
)

func (validator *Validator) createKeyStorage() error {
	// Create directory
	return os.MkdirAll(getKeysPath(), 0755)
}

func (validator *Validator) retrieveEnrollmentData(userId, pwd string) error {
	key, enrollCertRaw, err := validator.getEnrollmentCertificateFromECA(userId, pwd)
	if err != nil {
		log.Error("Failed getting enrollment certificate %s", err)

		return err
	}
	log.Info("Register:cert %s", utils.EncodeBase64(enrollCertRaw))
	//	validatorLogger.Info("Register:key %s", utils.EncodeBase64(key))

	// Verify

	cert, err := utils.DERToX509Certificate(enrollCertRaw)
	if err != nil {
		log.Error("Failed parsing enrollment certificate: %s", err.Error())

		return err
	}

	pk := cert.PublicKey.(*ecdsa.PublicKey)
	err = utils.VerifySignCapability(key, pk)
	if err != nil {
		log.Error("Failed checking enrollment certificate against enrollment key: %s", err.Error())

		return err
	}

	// TODO: store it in an encrypted form

	// Store enrollment  key
	log.Info("Storing enrollment key and certificate for user [%s]...", userId)

	rawKey, err := utils.PrivateKeyToPEM("", key)
	if err != nil {
		log.Error("Failed converting enrollment key to PEM: %s", err)
		return err
	}

	err = ioutil.WriteFile(getEnrollmentKeyPath(), rawKey, 0700)
	if err != nil {
		log.Error("Failed storing enrollment key: %s", err)
		return err
	}

	// Store enrollment cert
	err = ioutil.WriteFile(getEnrollmentCertPath(), utils.DERCertToPEM(enrollCertRaw), 0700)
	if err != nil {
		log.Error("Failed storing enrollment certificate: %s", err)
		return err
	}

	// Store enrollment id
	err = ioutil.WriteFile(getEnrollmentIDPath(), []byte(userId), 0700)
	if err != nil {
		log.Error("Failed storing enrollment certificate: %s", err)
		return err
	}

	return nil
}

func (validator *Validator) loadEnrollmentKey(pwd []byte) error {
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
	validator.enrollPrivKey = enrollPrivKey.(*ecdsa.PrivateKey)

	return nil
}

func (validator *Validator) loadEnrollmentCertificate() error {
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
	validator.enrollCert = enrollCert

	pk := validator.enrollCert.PublicKey.(*ecdsa.PublicKey)
	err = utils.VerifySignCapability(validator.enrollPrivKey, pk)
	if err != nil {
		log.Error("Failed checking enrollment certificate against enrollment key: %s", err.Error())

		return err
	}

	// Set validator ID
	validator.id = utils.Hash(rawEnrollCert)
	log.Info("Setting id to [%s]", utils.EncodeBase64(validator.id))

	return nil
}

func (validator *Validator) loadEnrollmentID() error {
	log.Info("Loading enrollment id at %s...", getEnrollmentIDPath())

	enrollID, err := ioutil.ReadFile(getEnrollmentIDPath())
	if err != nil {
		log.Error("Failed loading enrollment id: %s", err.Error())

		return err
	}

	// Set enrollment ID
	validator.enrollId = string(enrollID)
	log.Info("Setting enrollment id to [%s]", validator.enrollId)

	return nil
}
