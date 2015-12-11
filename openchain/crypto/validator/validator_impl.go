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
	"crypto/x509"
	"github.com/golang/protobuf/proto"
	"github.com/op/go-logging"
	"github.com/openblockchain/obc-peer/openchain/crypto/utils"
	obc "github.com/openblockchain/obc-peer/protos"
)

// Public Struct

type validatorImpl struct {
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
	enrollID      string
	enrollCert    *x509.Certificate
	enrollPrivKey interface{}
	enrollCerts   map[string]*x509.Certificate
}

// Public Methods

// Register is used to register this validator to the membership service.
// The information received from the membership service are stored
// locally and used for initialization.
// This method is supposed to be called only once when the validator
// is first deployed.
func (validator *validatorImpl) Register(id string, pwd []byte, enrollID, enrollPWD string) error {
	if validator.isInitialized {
		log.Error("Registering [%s]...done! Initialization already performed", enrollID)

		return nil
	}

	// Init Conf
	if err := validator.initConfiguration(id); err != nil {
		log.Error("Failed ini configuration [%s]: %s", enrollID, err)

		return err
	}

	// Start registration
	validator.log.Info("Registering [%s]...", enrollID)

	if validator.isRegistered() {
		validator.log.Error("Registering [%s]...done! Registration already performed", enrollID)
	} else {
		if err := validator.createKeyStorage(); err != nil {
			validator.log.Error("Failed creating key storage: %s", err)

			return err
		}

		if err := validator.retrieveECACertsChain(enrollID); err != nil {
			validator.log.Error("Failed retrieveing ECA certs chain: %s", err)

			return err
		}

		if err := validator.retrieveTCACertsChain(enrollID); err != nil {
			validator.log.Error("Failed retrieveing ECA certs chain: %s", err)

			return err
		}

		if err := validator.retrieveEnrollmentData(enrollID, enrollPWD); err != nil {
			validator.log.Error("Failed retrieveing enrollment data: %s", err)

			return err
		}
	}

	validator.log.Info("Registering [%s]...done!", enrollID)

	return nil
}

// Init initializes this validator by loading
// the required certificates and keys which are created at registration time.
// This method must be called at the very beginning to able to use
// the api. If the validator is not initialized,
// all the methods will report an error (ErrModuleNotInitialized).
func (validator *validatorImpl) Init(id string, pwd []byte) error {
	if validator.isInitialized {
		validator.log.Error("Already initializaed.")

		return nil
	}

	// Init Conf
	if err := validator.initConfiguration(id); err != nil {
		return err
	}

	if !validator.isRegistered() {
		return utils.ErrRegistrationRequired
	}

	// Initialize keystore
	validator.log.Info("Init keystore...")
	// TODO: password support
	err := validator.initKeyStore()
	if err != nil {
		if err != utils.ErrKeyStoreAlreadyInitialized {
			validator.log.Error("Keystore already initialized.")
		} else {
			validator.log.Error("Failed initiliazing keystore %s", err)

			return err
		}
	}
	validator.log.Info("Init keystore...done.")

	// Init crypto engine
	err = validator.initCryptoEngine()
	if err != nil {
		validator.log.Error("Failed initiliazing crypto engine %s", err)
		return err
	}

	validator.log.Info("Initialization...done.")

	// Initialisation complete
	validator.isInitialized = true

	return nil
}

func (validator *validatorImpl) GetName() string {
	return validator.conf.id
}

// GetID returns this validator's identifier
func (validator *validatorImpl) GetID() []byte {
	// Clone id to avoid exposure of internal data structure
	clone := make([]byte, len(validator.id))
	copy(clone, validator.id)

	return clone
}

// GetEnrollmentID returns this validator's enroolment id
func (validator *validatorImpl) GetEnrollmentID() string {
	return validator.enrollID
}

// TransactionPreValidation verifies that the transaction is
// well formed with the respect to the security layer
// prescriptions (i.e. signature verification).
func (validator *validatorImpl) TransactionPreValidation(tx *obc.Transaction) (*obc.Transaction, error) {
	if !validator.isInitialized {
		return nil, utils.ErrNotInitialized
	}

	if tx.Cert != nil && tx.Signature != nil {
		validator.log.Info("TransactionPreValidation: executing...")

		// Verify the transaction
		// 1. Unmarshal cert
		cert, err := utils.DERToX509Certificate(tx.Cert)
		if err != nil {
			validator.log.Error("TransactionPreExecution: failed unmarshalling cert %s:", err)
			return tx, err
		}
		// TODO: verify cert

		// 3. Marshall tx without signature
		signature := tx.Signature
		tx.Signature = nil
		rawTx, err := proto.Marshal(tx)
		if err != nil {
			validator.log.Error("TransactionPreExecution: failed marshaling tx %s:", err)
			return tx, err
		}
		tx.Signature = signature

		// 2. Verify signature
		ok, err := validator.verify(cert.PublicKey, rawTx, tx.Signature)
		if err != nil {
			validator.log.Error("TransactionPreExecution: failed marshaling tx %s:", err)
			return tx, err
		}

		if !ok {
			return tx, utils.ErrInvalidTransactionSignature
		}
	} else {
		if tx.Cert == nil {
			return tx, utils.ErrTransactionCertificate
		}

		if tx.Signature == nil {
			return tx, utils.ErrTransactionSignature
		}
	}

	return tx, nil
}

// TransactionPreValidation verifies that the transaction is
// well formed with the respect to the security layer
// prescriptions (i.e. signature verification). If this is the case,
// the method prepares the transaction to be executed.
func (validator *validatorImpl) TransactionPreExecution(tx *obc.Transaction) (*obc.Transaction, error) {
	if !validator.isInitialized {
		return nil, utils.ErrNotInitialized
	}

	return tx, nil
}

// Sign signs msg with this validator's signing key and outputs
// the signature if no error occurred.
func (validator *validatorImpl) Sign(msg []byte) ([]byte, error) {
	return validator.signWithEnrollmentKey(msg)
}

// Verify checks that signature if a valid signature of message under vkID's verification key.
// If the verification succeeded, Verify returns nil meaning no error occurred.
// If vkID is nil, then the signature is verified against this validator's verification key.
func (validator *validatorImpl) Verify(vkID, signature, message []byte) error {
	cert, err := validator.getEnrollmentCert(vkID)
	if err != nil {
		validator.log.Error("Failed getting enrollment cert for [%s]: %s", utils.EncodeBase64(vkID), err)
	}

	vk := cert.PublicKey.(*ecdsa.PublicKey)

	ok, err := validator.verify(vk, message, signature)
	if err != nil {
		validator.log.Error("Failed verifying signature for [%s]: %s", utils.EncodeBase64(vkID), err)
	}

	if !ok {
		validator.log.Error("Failed invalid signature for [%s]", utils.EncodeBase64(vkID))

		return utils.ErrInvalidSignature
	}

	return nil
}

func (validator *validatorImpl) Close() error {
	// Close keystore
	var err error

	if validator.ks != nil {
		err = validator.ks.Close()
	}

	return err
}

// Private Methods

func (validator *validatorImpl) initCryptoEngine() error {
	validator.log.Info("Initialing Crypto Engine...")

	validator.rootsCertPool = x509.NewCertPool()
	validator.enrollCerts = make(map[string]*x509.Certificate)

	// Load ECA certs chain
	if err := validator.loadECACertsChain(); err != nil {
		return err
	}

	// Load TCA certs chain
	if err := validator.loadTCACertsChain(); err != nil {
		return err
	}

	// Load enrollment secret key
	// TODO: finalize encrypted pem support
	if err := validator.loadEnrollmentKey(nil); err != nil {
		return err
	}

	// Load enrollment certificate and set validator ID
	if err := validator.loadEnrollmentCertificate(); err != nil {
		return err
	}

	// Load enrollment id
	if err := validator.loadEnrollmentID(); err != nil {
		return err
	}

	validator.log.Info("Initialing Crypto Engine...done!")

	return nil
}
