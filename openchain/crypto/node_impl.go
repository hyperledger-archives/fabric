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
	"github.com/openblockchain/obc-peer/openchain/crypto/utils"
)

// Public Struct

type nodeImpl struct {
	isInitialized bool

	// Configuration
	conf *configuration

	/*
		// Logging
		log *logging.Logger
	*/

	// keyStore
	ks *keyStore

	// Certs Pool
	rootsCertPool *x509.CertPool
	tlsCertPool   *x509.CertPool
	ecaCertPool   *x509.CertPool
	tcaCertPool   *x509.CertPool

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
		node.error("Registering [%s]...done! Initialization already performed", enrollID)

		return utils.ErrAlreadyInitialized
	}

	// Init Conf
	if err := node.initConfiguration(prefix, name); err != nil {
		log.Error("Failed initiliazing configuration [%s] [%s].", enrollID, err)

		return err
	}

	// Start registration
	node.debug("Registering [%s]...", enrollID)

	if node.isRegistered() {
		node.error("Registering [%s]...done! Registration already performed", enrollID)

		return utils.ErrAlreadyRegistered
	}

	// Initialize keystore
	node.debug("Init keystore...")
	err := node.initKeyStore(pwd)
	if err != nil {
		if err != utils.ErrKeyStoreAlreadyInitialized {
			node.error("Keystore already initialized.")
		} else {
			node.error("Failed initiliazing keystore [%s].", err.Error())

			return err
		}
	}
	node.debug("Init keystore...done.")

	// Register crypto engine
	err = node.registerCryptoEngine(enrollID, enrollPWD)
	if err != nil {
		node.error("Failed registering crypto engine [%s].", err.Error())
		return err
	}

	node.debug("Registering [%s]...done!", enrollID)

	return nil
}

func (node *nodeImpl) init(prefix, name string, pwd []byte) error {
	if node.isInitialized {
		node.error("Already initializaed.")

		return utils.ErrAlreadyInitialized
	}

	// Init Conf
	if err := node.initConfiguration(prefix, name); err != nil {
		return err
	}

	if !node.isRegistered() {
		node.error("Not registered yet.")

		return utils.ErrRegistrationRequired
	}

	// Initialize keystore
	node.debug("Init keystore...")
	err := node.initKeyStore(pwd)
	if err != nil {
		if err != utils.ErrKeyStoreAlreadyInitialized {
			node.error("Keystore already initialized.")
		} else {
			node.error("Failed initiliazing keystore [%s].", err.Error())

			return err
		}
	}
	node.debug("Init keystore...done.")

	// Init crypto engine
	err = node.initCryptoEngine()
	if err != nil {
		node.error("Failed initiliazing crypto engine [%s].", err.Error())
		return err
	}

	// Initialisation complete
	node.isInitialized = true

	node.debug("Initialization...done.")

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
