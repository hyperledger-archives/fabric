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
	obc "github.com/openblockchain/obc-peer/protos"
"errors"
	"reflect"
)

// Public Struct

type validatorImpl struct {
	peer *peerImpl

	isInitialized bool

	enrollCerts map[string]*x509.Certificate
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

	switch tx.ConfidentialityLevel {
	case obc.Transaction_CHAINCODE_PUBLIC:
		// TODO: Nothing to do here?

		break
	case obc.Transaction_CHAINCODE_CONFIDENTIAL:
		// Decrypt payload
		err := validator.decryptTx(tx)
		if err != nil {
			validator.peer.node.log.Error("Failed decrypting: %s", err)

			return nil, err
		}
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
		validator.peer.node.log.Error("Failed getting enrollment cert for [%s]: %s", utils.EncodeBase64(vkID), err)
	}

	vk := cert.PublicKey.(*ecdsa.PublicKey)

	ok, err := validator.verify(vk, message, signature)
	if err != nil {
		validator.peer.node.log.Error("Failed verifying signature for [%s]: %s", utils.EncodeBase64(vkID), err)
	}

	if !ok {
		validator.peer.node.log.Error("Failed invalid signature for [%s]", utils.EncodeBase64(vkID))

		return utils.ErrInvalidSignature
	}

	return nil
}

func (validator *validatorImpl) GetStateEncryptor(deployTx, executeTx *obc.Transaction) (StateEncryptor, error) {
	// Check nonce
	if deployTx.Nonce == nil || len(deployTx.Nonce) == 0 {
		return nil, errors.New("Failed getting state ES. Invalid deploy nonce.")
	}
	if executeTx.Nonce == nil || len(executeTx.Nonce) == 0 {
		return nil, errors.New("Failed getting state ES. Invalid invoke nonce.")
	}
	// Check ChaincodeID
	if deployTx.ChaincodeID == nil {
		return nil, errors.New("Invalid deploy chaincodeID.")
	}
	if executeTx.ChaincodeID == nil {
		return nil, errors.New("Invalid execute chaincodeID.")
	}
	// Check that deployTx and executeTx refers to the same chaincode
	if !reflect.DeepEqual(deployTx.ChaincodeID, executeTx.ChaincodeID) {
		return nil, utils.ErrDirrentChaincodeID
	}

	validator.peer.node.log.Info("Parsing transaction. Type [%s].", executeTx.Type)

	if executeTx.Type == obc.Transaction_CHAINCODE_QUERY {
		validator.peer.node.log.Info("Parsing Query transaction...")

		// Compute deployTxKey key from the deploy transaction. This is used to decrypt the actual state
		// of the chaincode
		deployTxKey := utils.HMAC(validator.peer.node.enrollChainKey, deployTx.Nonce)

		// Compute the key used to encrypt the result of the query
		queryKey := utils.HMACTruncated(validator.peer.node.enrollChainKey, append([]byte{6}, executeTx.Nonce...), utils.AESKeyLength)

		// Init the state encryptor
		se := queryStateEncryptor{}
		err := se.init(validator.peer.node.log, queryKey, deployTxKey)
		if err != nil {
			return nil, err
		}

		return &se, nil
	} else {
		// Compute deployTxKey key from the deploy transaction
		deployTxKey := utils.HMAC(validator.peer.node.enrollChainKey, deployTx.Nonce)

		// Mask executeTx.Nonce
		executeTxNonce := utils.HMACTruncated(deployTxKey, utils.Hash(executeTx.Nonce), utils.NonceSize)

		// Compute stateKey to encrypt the states and nonceStateKey to generates IVs. This
		// allows validators to reach consesus
		stateKey := utils.HMACTruncated(deployTxKey, append([]byte{3}, executeTxNonce...), utils.AESKeyLength)
		nonceStateKey := utils.HMAC(deployTxKey, append([]byte{4}, executeTxNonce...))

		// Init the state encryptor
		se := stateEncryptorImpl{}
		err := se.init(validator.peer.node.log, stateKey, nonceStateKey, deployTxKey, executeTxNonce)
		if err != nil {
			return nil, err
		}

		return &se, nil
	}
}

// Private Methods

func (validator *validatorImpl) register(id string, pwd []byte, enrollID, enrollPWD string) error {
	if validator.isInitialized {
		validator.peer.node.log.Error("Registering [%s]...done! Initialization already performed", enrollID)

		return utils.ErrAlreadyInitialized
	}

	// Register node
	peer := new(peerImpl)
	if err := peer.register("validator", id, pwd, enrollID, enrollPWD); err != nil {
		log.Error("Failed registering [%s]: %s", enrollID, err)
		return err
	}

	validator.peer = peer

	return nil
}

func (validator *validatorImpl) init(name string, pwd []byte) error {
	if validator.isInitialized {
		validator.peer.node.log.Error("Already initializaed.")

		return utils.ErrAlreadyInitialized
	}

	// Register node
	peer := new(peerImpl)
	if err := peer.init("validator", name, pwd); err != nil {
		return err
	}
	validator.peer = peer

	// Initialize keystore
	validator.peer.node.log.Info("Init keystore...")
	err := validator.initKeyStore()
	if err != nil {
		if err != utils.ErrKeyStoreAlreadyInitialized {
			validator.peer.node.log.Error("Keystore already initialized.")
		} else {
			validator.peer.node.log.Error("Failed initiliazing keystore %s", err)

			return err
		}
	}
	validator.peer.node.log.Info("Init keystore...done.")

	// Init crypto engine
	err = validator.initCryptoEngine()
	if err != nil {
		validator.peer.node.log.Error("Failed initiliazing crypto engine %s", err)
		return err
	}

	// initialized
	validator.isInitialized = true

	peer.node.log.Info("Initialization...done.")

	return nil
}

func (validator *validatorImpl) initCryptoEngine() error {
	validator.enrollCerts = make(map[string]*x509.Certificate)
	return nil
}

func (validator *validatorImpl) close() error {
	if validator.peer != nil {
		return validator.peer.close()
	} else {
		return nil
	}
}
