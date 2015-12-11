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
	"crypto/x509"
	"github.com/golang/protobuf/proto"
	"github.com/op/go-logging"
	"github.com/openblockchain/obc-peer/openchain/crypto/utils"
	obc "github.com/openblockchain/obc-peer/protos"
)

type clientImpl struct {
	isInitialized bool

	// Configuration
	conf *configuration

	// Logging
	log *logging.Logger

	// keyStore
	ks *keyStore

	// Identifier
	id []byte

	// Certs
	rootsCertPool *x509.CertPool

	// Enrollment Certificate and private key
	enrollID      string
	enrollCert    *x509.Certificate
	enrollPrivKey *ecdsa.PrivateKey
}

// Public Methods

// Register is used to register this client to the membership service.
// The information received from the membership service are stored
// locally and used for initialization.
// This method is supposed to be called only once when the client
// is first deployed.
func (client *clientImpl) Register(id string, pwd []byte, enrollID, enrollPWD string) error {
	if client.isInitialized {
		log.Error("Registering [%s]...done! Initialization already performed", enrollID)

		return nil
	}

	// Init Conf
	if err := client.initConfiguration(id); err != nil {
		return err
	}

	// Start registration
	client.log.Info("Registering [%s]...", enrollID)

	if client.isRegistered() {
		client.log.Error("Registering [%s]...done! Registration already performed", enrollID)

	} else {
		if err := client.createKeyStorage(); err != nil {
			client.log.Error("Failed creating key storage: %s", err)

			return err
		}

		if err := client.retrieveECACertsChain(enrollID); err != nil {
			client.log.Error("Failed retrieveing ECA certs chain: %s", err)

			return err
		}

		if err := client.retrieveTCACertsChain(enrollID); err != nil {
			client.log.Error("Failed retrieveing ECA certs chain: %s", err)

			return err
		}

		if err := client.retrieveEnrollmentData(enrollID, enrollPWD); err != nil {
			client.log.Error("Failed retrieveing enrollment data: %s", err)

			return err
		}
	}

	client.log.Info("Registering [%s]...done!", enrollID)

	return nil
}

func (client *clientImpl) GetName() string {
	return client.conf.id
}

// Init initializes this client by loading
// the required certificates and keys which are created at registration time.
// This method must be called at the very beginning to able to use
// the api. If the client is not initialized,
// all the methods will report an error (ErrModuleNotInitialized).
func (client *clientImpl) Init(id string, pwd []byte) error {

	if client.isInitialized {
		client.log.Error("Already initializaed.")

		return nil
	}

	// Init Conf
	if err := client.initConfiguration(id); err != nil {
		return err
	}

	if !client.isRegistered() {
		return utils.ErrRegistrationRequired
	}

	client.log.Info("Initialization...")

	// Initialize keystore
	client.log.Info("Init keystore...")
	// TODO: password support
	err := client.initKeyStore()
	if err != nil {
		if err != utils.ErrKeyStoreAlreadyInitialized {
			client.log.Error("Keystore already initialized.")
		} else {
			client.log.Error("Failed initiliazing keystore %s", err)

			return err
		}
	}
	client.log.Info("Init keystore...done.")

	// Init crypto engine
	err = client.initCryptoEngine()
	if err != nil {
		client.log.Error("Failed initiliazing crypto engine %s", err)
		return err
	}

	// initialized
	client.isInitialized = true

	client.log.Info("Initialization...done.")

	return nil
}

// NewChaincodeDeployTransaction is used to deploy chaincode.
func (client *clientImpl) NewChaincodeDeployTransaction(chainletDeploymentSpec *obc.ChaincodeDeploymentSpec, uuid string) (*obc.Transaction, error) {
	// Verify that the client is initialized
	if !client.isInitialized {
		return nil, utils.ErrNotInitialized
	}

	// Create a new transaction
	tx, err := obc.NewChaincodeDeployTransaction(chainletDeploymentSpec, uuid)
	if err != nil {
		client.log.Error("Failed creating new transaction %s:", err)
		return nil, err
	}

	// TODO: implement like this.
	// getNextTCert returns only a TCert
	// Then, invoke signWithTCert to sign the signature

	// Get next available (not yet used) transaction certificate
	// with the relative signing key.
	rawTCert, signKey, err := client.getNextTCert()
	if err != nil {
		client.log.Error("Failed getting next transaction certificate %s:", err)
		return nil, err
	}

	// Append the certificate to the transaction
	client.log.Info("Appending certificate %s", utils.EncodeBase64(rawTCert))
	tx.Cert = rawTCert

	// Sign the transaction and append the signature
	// 1. Marshall tx to bytes
	rawTx, err := proto.Marshal(tx)
	if err != nil {
		client.log.Error("Failed marshaling tx %s:", err)
		return nil, err
	}

	// 2. Sign rawTx and check signature
	client.log.Info("Signing tx %s", utils.EncodeBase64(rawTx))
	rawSignature, err := client.sign(signKey, rawTx)
	if err != nil {
		client.log.Error("Failed creating signature %s:", err)
		return nil, err
	}

	// 3. Append the signature
	tx.Signature = rawSignature

	client.log.Info("Appending signature %s", utils.EncodeBase64(rawSignature))

	return tx, nil
}

// NewChaincodeInvokeTransaction is used to invoke chaincode's functions.
func (client *clientImpl) NewChaincodeInvokeTransaction(chaincodeInvocation *obc.ChaincodeInvocationSpec, uuid string) (*obc.Transaction, error) {
	// Verify that the client is initialized
	if !client.isInitialized {
		return nil, utils.ErrNotInitialized
	}

	// Create a new transaction
	tx, err := obc.NewChaincodeExecute(chaincodeInvocation, uuid, obc.Transaction_CHAINCODE_EXECUTE)
	if err != nil {
		client.log.Error("Failed creating new transaction %s:", err)
		return nil, err
	}

	// TODO: implement like this.
	// getNextTCert returns only a TCert
	// Then, invoke signWithTCert to sign the signature

	// Get next available (not yet used) transaction certificate
	// with the relative signing key.
	rawTCert, signKey, err := client.getNextTCert()
	if err != nil {
		client.log.Error("Failed getting next transaction certificate %s:", err)
		return nil, err
	}

	// Append the certificate to the transaction
	client.log.Info("Appending certificate %s", utils.EncodeBase64(rawTCert))
	tx.Cert = rawTCert

	// Sign the transaction and append the signature
	// 1. Marshall tx to bytes
	rawTx, err := proto.Marshal(tx)
	if err != nil {
		client.log.Error("Failed marshaling tx %s:", err)
		return nil, err
	}

	// 2. Sign rawTx and check signature
	client.log.Info("Signing tx %s", utils.EncodeBase64(rawTx))
	rawSignature, err := client.sign(signKey, rawTx)
	if err != nil {
		client.log.Error("Failed creating signature %s:", err)
		return nil, err
	}

	// 3. Append the signature
	tx.Signature = rawSignature

	client.log.Info("Appending signature %s", utils.EncodeBase64(rawSignature))

	return tx, nil
}

// Private Methods

func (client *clientImpl) initCryptoEngine() error {
	client.log.Info("Initialing Crypto Engine...")

	client.rootsCertPool = x509.NewCertPool()

	// Load ECA certs chain
	if err := client.loadECACertsChain(); err != nil {
		return err
	}

	// Load TCA certs chain
	if err := client.loadTCACertsChain(); err != nil {
		return err
	}

	// Load enrollment secret key
	// TODO: finalize encrypted pem support
	if err := client.loadEnrollmentKey(nil); err != nil {
		return err
	}

	// Load enrollment certificate
	if err := client.loadEnrollmentCertificate(); err != nil {
		return err
	}

	// Load enrollment id
	if err := client.loadEnrollmentID(); err != nil {
		return err
	}

	client.log.Info("Initialing Crypto Engine...done!")
	return nil
}

func (client *clientImpl) Close() error {
	// Close keystore
	var err error

	if client.ks != nil {
		err = client.ks.Close()
	}

	return err
}

// CheckTransaction is used to verify that a transaction
// is well formed with the respect to the security layer
// prescriptions. To be used for internal verifications.
func (client *clientImpl) checkTransaction(tx *obc.Transaction) error {
	if !client.isInitialized {
		return utils.ErrNotInitialized
	}

	if tx.Cert == nil && tx.Signature == nil {
		return utils.ErrTransactionMissingCert
	}

	if tx.Cert != nil && tx.Signature != nil {
		// Verify the transaction
		// 1. Unmarshal cert
		cert, err := utils.DERToX509Certificate(tx.Cert)
		if err != nil {
			client.log.Error("Failed unmarshalling cert %s:", err)
			return err
		}
		// TODO: verify cert

		// 3. Marshall tx without signature
		signature := tx.Signature
		tx.Signature = nil
		rawTx, err := proto.Marshal(tx)
		if err != nil {
			client.log.Error("Failed marshaling tx %s:", err)
			return err
		}
		tx.Signature = signature

		// 2. Verify signature
		ver, err := client.verify(cert.PublicKey, rawTx, tx.Signature)
		if err != nil {
			client.log.Error("Failed marshaling tx %s:", err)
			return err
		}

		if ver {
			return nil
		}

		return utils.ErrInvalidTransactionSignature
	}

	return utils.ErrTransactionMissingCert
}
