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
	"crypto/x509"
	"github.com/op/go-logging"
	"github.com/openblockchain/obc-peer/openchain/crypto/utils"
)

// Public Struct

type nodeImpl struct {
	isInitialized bool

	// Configuration
	conf *configuration

	// Logging
	log *logging.Logger

	// keyStore
	ks *keyStore

	// Certs
	rootsCertPool *x509.CertPool

	// 48-bytes identifier
	id []byte

	// Enrollment Certificate and private key
	enrollID       string
	enrollCert     *x509.Certificate
	enrollPrivKey  *ecdsa.PrivateKey
	enrollCertHash []byte

	// Enrollment Chain
	enrollChainKey []byte

	// TLS
	tlsCert *x509.Certificate
}

func (node *nodeImpl) GetName() string {
	return node.conf.name
}

func (node *nodeImpl) isRegistered() bool {
	missing, _ := utils.FileMissing(node.conf.getRawsPath(), node.conf.getEnrollmentIDFilename())

	return !missing
}

func (node *nodeImpl) register(prefix, name string, pwd []byte, enrollID, enrollPWD string) error {
	if node.isInitialized {
		node.log.Error("Registering [%s]...done! Initialization already performed", enrollID)

		return utils.ErrAlreadyInitialized
	}

	// Init Conf
	if err := node.initConfiguration(prefix, name); err != nil {
		log.Error("Failed initiliazing configuration [%s] [%s].", enrollID, err)

		return err
	}

	// Start registration
	node.log.Info("Registering [%s]...", enrollID)

	if node.isRegistered() {
		node.log.Error("Registering [%s]...done! Registration already performed", enrollID)

		return utils.ErrAlreadyRegistered
	}

	// Initialize keystore
	node.log.Info("Init keystore...")
	err := node.initKeyStore(pwd)
	if err != nil {
		if err != utils.ErrKeyStoreAlreadyInitialized {
			node.log.Error("Keystore already initialized.")
		} else {
			node.log.Error("Failed initiliazing keystore [%s].", err.Error())

			return err
		}
	}
	node.log.Info("Init keystore...done.")

	// Retrieve keys and certificates

	if err := node.retrieveECACertsChain(enrollID); err != nil {
		node.log.Error("Failed retrieveing ECA certs chain [%s].", err.Error())

		return err
	}

	if err := node.retrieveTCACertsChain(enrollID); err != nil {
		node.log.Error("Failed retrieveing ECA certs chain [%s].", err.Error())

		return err
	}

	if err := node.retrieveEnrollmentData(enrollID, enrollPWD); err != nil {
		node.log.Error("Failed retrieveing enrollment data [%s].", err.Error())

		return err
	}

	if err := node.retrieveTLSCertificate(enrollID, enrollPWD); err != nil {
		node.log.Error("Failed retrieveing enrollment data: %s", err)

		return err
	}

	node.log.Info("Registering [%s]...done!", enrollID)

	return nil
}

func (node *nodeImpl) init(prefix, name string, pwd []byte) error {
	if node.isInitialized {
		node.log.Error("Already initializaed.")

		return utils.ErrAlreadyInitialized
	}

	// Init Conf
	if err := node.initConfiguration(prefix, name); err != nil {
		return err
	}

	if !node.isRegistered() {
		node.log.Error("Not registered yet.")

		return utils.ErrRegistrationRequired
	}

	// Initialize keystore
	node.log.Info("Init keystore...")
	err := node.initKeyStore(pwd)
	if err != nil {
		if err != utils.ErrKeyStoreAlreadyInitialized {
			node.log.Error("Keystore already initialized.")
		} else {
			node.log.Error("Failed initiliazing keystore [%s].", err.Error())

			return err
		}
	}
	node.log.Info("Init keystore...done.")

	// Init crypto engine
	err = node.initCryptoEngine()
	if err != nil {
		node.log.Error("Failed initiliazing crypto engine [%s].", err.Error())
		return err
	}

	// Initialisation complete
	node.isInitialized = true

	node.log.Info("Initialization...done.")

	return nil
}

func (node *nodeImpl) close() error {
	// Close keystore
	var err error

	if node.ks != nil {
		err = node.ks.close()
	}

	return err
}
