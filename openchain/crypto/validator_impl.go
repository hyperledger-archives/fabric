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

	"github.com/openblockchain/obc-peer/openchain/crypto/ecies"
	"github.com/openblockchain/obc-peer/openchain/crypto/ecies/generic"
	"github.com/openblockchain/obc-peer/openchain/crypto/utils"
	obc "github.com/openblockchain/obc-peer/protos"
	"fmt"
)

// Public Struct

type validatorImpl struct {
	peer *peerImpl

	isInitialized bool

	enrollCerts map[string]*x509.Certificate

	// Chain
	chainPrivateKey ecies.PrivateKey
}

func (validator *validatorImpl) GetType() Entity_Type {
	return validator.peer.node.eType
}

func (validator *validatorImpl) GetName() string {
	return validator.peer.GetName()
}

// GetID returns this validator's identifier
func (validator *validatorImpl) GetID() []byte {
	return validator.peer.GetID()
}

// GetEnrollmentID returns this validator's enroolment id
func (validator *validatorImpl) GetEnrollmentID() string {
	return validator.peer.GetEnrollmentID()
}

// TransactionPreValidation verifies that the transaction is
// well formed with the respect to the security layer
// prescriptions (i.e. signature verification).
func (validator *validatorImpl) TransactionPreValidation(tx *obc.Transaction) (*obc.Transaction, error) {
	if !validator.isInitialized {
		return nil, utils.ErrNotInitialized
	}

	return validator.peer.TransactionPreValidation(tx)
}

// TransactionPreValidation verifies that the transaction is
// well formed with the respect to the security layer
// prescriptions (i.e. signature verification). If this is the case,
// the method prepares the transaction to be executed.
func (validator *validatorImpl) TransactionPreExecution(tx *obc.Transaction) (*obc.Transaction, error) {
	if !validator.isInitialized {
		return nil, utils.ErrNotInitialized
	}

	//	validator.peer.node.debug("Pre executing [%s].", tx.String())
	validator.peer.node.debug("Tx confdential level [%s].", tx.ConfidentialityLevel.String())

	if validityPeriodVerificationEnabled() {
		tx, err := validator.verifyValidityPeriod(tx)
		if err != nil {
			validator.peer.node.error("TransactionPreExecution: error verifying certificate validity period %s:", err)
			return tx, err
		}
	}

	switch tx.ConfidentialityLevel {
	case obc.ConfidentialityLevel_PUBLIC:
		// Nothing to do here!

		return tx, nil
	case obc.ConfidentialityLevel_CONFIDENTIAL:
		validator.peer.node.debug("Clone and Decrypt.")

		// Clone the transaction and decrypt it
		newTx, err := validator.deepCloneAndDecryptTx(tx)
		if err != nil {
			validator.peer.node.error("Failed decrypting [%s].", err.Error())

			return nil, err
		}

		return newTx, nil
	default:
		return nil, utils.ErrInvalidConfidentialityLevel
	}
}

// Sign signs msg with this validator's signing key and outputs
// the signature if no error occurred.
func (validator *validatorImpl) Sign(msg []byte) ([]byte, error) {
	return validator.peer.node.signWithEnrollmentKey(msg)
}

// Verify checks that signature if a valid signature of message under vkID's verification key.
// If the verification succeeded, Verify returns nil meaning no error occurred.
// If vkID is nil, then the signature is verified against this validator's verification key.
func (validator *validatorImpl) Verify(vkID, signature, message []byte) error {
	if len(vkID) == 0 {
		return fmt.Errorf("Invalid peer id. It is empty.")
	}
	if len(signature) == 0 {
		return fmt.Errorf("Invalid signature. It is empty.")
	}
	if len(message) == 0 {
		return fmt.Errorf("Invalid message. It is empty.")
	}

	cert, err := validator.getEnrollmentCert(vkID)
	if err != nil {
		validator.peer.node.error("Failed getting enrollment cert for [% x]: [%s]", vkID, err)

		return err
	}

	vk := cert.PublicKey.(*ecdsa.PublicKey)

	ok, err := validator.verify(vk, message, signature)
	if err != nil {
		validator.peer.node.error("Failed verifying signature for [% x]: [%s]", vkID, err)

		return err
	}

	if !ok {
		validator.peer.node.error("Failed invalid signature for [% x]", vkID)

		return utils.ErrInvalidSignature
	}

	return nil
}

// Private Methods

func (validator *validatorImpl) register(id string, pwd []byte, enrollID, enrollPWD string) error {
	if validator.isInitialized {
		validator.peer.node.error("Registering...done! Initialization already performed", enrollID)

		return utils.ErrAlreadyInitialized
	}

	// Register node
	peer := new(peerImpl)
	if err := peer.register(Entity_Validator, id, pwd, enrollID, enrollPWD); err != nil {
		log.Error("Failed registering [%s]: [%s]", enrollID, err)
		return err
	}

	validator.peer = peer

	return nil
}

func (validator *validatorImpl) init(name string, pwd []byte) error {
	if validator.isInitialized {
		validator.peer.node.error("Already initializaed.")

		return utils.ErrAlreadyInitialized
	}

	// Register node
	peer := new(peerImpl)
	if err := peer.init(Entity_Validator, name, pwd); err != nil {
		return err
	}
	validator.peer = peer

	// Initialize keystore
	validator.peer.node.debug("Init keystore...")
	err := validator.initKeyStore()
	if err != nil {
		if err != utils.ErrKeyStoreAlreadyInitialized {
			validator.peer.node.error("Keystore already initialized.")
		} else {
			validator.peer.node.error("Failed initiliazing keystore [%s].", err.Error())

			return err
		}
	}
	validator.peer.node.debug("Init keystore...done.")

	// Init crypto engine
	err = validator.initCryptoEngine()
	if err != nil {
		validator.peer.node.error("Failed initiliazing crypto engine [%s].", err.Error())
		return err
	}

	// initialized
	validator.isInitialized = true

	return nil
}

func (validator *validatorImpl) initCryptoEngine() (err error) {
	validator.enrollCerts = make(map[string]*x509.Certificate)

	// Init chain publicKey
	validator.chainPrivateKey, err = generic.NewPrivateKeyFromECDSA(
		validator.peer.node.enrollChainKey.(*ecdsa.PrivateKey),
	)
	if err != nil {
		return
	}

	return
}

func (validator *validatorImpl) close() error {
	if validator.peer != nil {
		return validator.peer.close()
	}

	return nil
}
