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
	"errors"
	"reflect"

	"encoding/asn1"
	"github.com/openblockchain/obc-peer/openchain/crypto/ecies/generic"
	"github.com/openblockchain/obc-peer/openchain/crypto/utils"
	obc "github.com/openblockchain/obc-peer/protos"
)

func (validator *validatorImpl) GetStateEncryptor(deployTx, executeTx *obc.Transaction) (StateEncryptor, error) {
	switch executeTx.ConfidentialityProtocolVersion {
	case "1.1":
		return validator.getStateEncryptor1_1(deployTx, executeTx)
	case "1.2":
		return validator.getStateEncryptor1_2(deployTx, executeTx)
	}

	return nil, utils.ErrInvalidConfidentialityLevel
}

func (validator *validatorImpl) getStateEncryptor1_1(deployTx, executeTx *obc.Transaction) (StateEncryptor, error) {
	// Check nonce
	if deployTx.Nonce == nil || len(deployTx.Nonce) == 0 {
		return nil, errors.New("Invalid deploy nonce.")
	}
	if executeTx.Nonce == nil || len(executeTx.Nonce) == 0 {
		return nil, errors.New("Invalid invoke nonce.")
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
		return nil, utils.ErrDifferentChaincodeID
	}
	// Check the confidentiality protocol version
	if deployTx.ConfidentialityProtocolVersion != executeTx.ConfidentialityProtocolVersion {
		return nil, utils.ErrDifferrentConfidentialityProtocolVersion
	}

	validator.log.Debug("Parsing transaction. Type [%s]. Confidentiality Protocol Version [%d]", executeTx.Type.String(), executeTx.ConfidentialityProtocolVersion)

	// client.enrollChainKey is an AES key represented as byte array
	enrollChainKey := validator.enrollChainKey.([]byte)

	if executeTx.Type == obc.Transaction_CHAINCODE_QUERY {
		validator.log.Debug("Parsing Query transaction...")

		// Compute deployTxKey key from the deploy transaction. This is used to decrypt the actual state
		// of the chaincode
		deployTxKey := utils.HMAC(enrollChainKey, deployTx.Nonce)

		// Compute the key used to encrypt the result of the query
		queryKey := utils.HMACTruncated(enrollChainKey, append([]byte{6}, executeTx.Nonce...), utils.AESKeyLength)

		// Init the state encryptor
		se := queryStateEncryptor{}
		err := se.init(validator.log, queryKey, deployTxKey)
		if err != nil {
			return nil, err
		}

		return &se, nil
	}

	// Compute deployTxKey key from the deploy transaction
	deployTxKey := utils.HMAC(enrollChainKey, deployTx.Nonce)

	// Mask executeTx.Nonce
	executeTxNonce := utils.HMACTruncated(deployTxKey, utils.Hash(executeTx.Nonce), utils.NonceSize)

	// Compute stateKey to encrypt the states and nonceStateKey to generates IVs. This
	// allows validators to reach consesus
	stateKey := utils.HMACTruncated(deployTxKey, append([]byte{3}, executeTxNonce...), utils.AESKeyLength)
	nonceStateKey := utils.HMAC(deployTxKey, append([]byte{4}, executeTxNonce...))

	// Init the state encryptor
	se := stateEncryptorImpl{}
	err := se.init(validator.log, stateKey, nonceStateKey, deployTxKey, executeTxNonce)
	if err != nil {
		return nil, err
	}

	return &se, nil
}

func (validator *validatorImpl) getStateEncryptor1_2(deployTx, executeTx *obc.Transaction) (StateEncryptor, error) {
	// Check nonce
	if deployTx.Nonce == nil || len(deployTx.Nonce) == 0 {
		return nil, errors.New("Invalid deploy nonce.")
	}
	if executeTx.Nonce == nil || len(executeTx.Nonce) == 0 {
		return nil, errors.New("Invalid invoke nonce.")
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
		return nil, utils.ErrDifferentChaincodeID
	}
	// Check the confidentiality protocol version
	if deployTx.ConfidentialityProtocolVersion != executeTx.ConfidentialityProtocolVersion {
		return nil, utils.ErrDifferrentConfidentialityProtocolVersion
	}

	validator.log.Debug("Parsing transaction. Type [%s]. Confidentiality Protocol Version [%s]", executeTx.Type.String(), executeTx.ConfidentialityProtocolVersion)

	deployStateKey, err := validator.getStateKeyFromTransaction(deployTx)

	if executeTx.Type == obc.Transaction_CHAINCODE_QUERY {
		validator.log.Debug("Parsing Query transaction...")

		executeStateKey, err := validator.getStateKeyFromTransaction(executeTx)

		// Compute deployTxKey key from the deploy transaction. This is used to decrypt the actual state
		// of the chaincode
		deployTxKey := utils.HMAC(deployStateKey, deployTx.Nonce)

		// Compute the key used to encrypt the result of the query
		//queryKey := utils.HMACTruncated(executeStateKey, append([]byte{6}, executeTx.Nonce...), utils.AESKeyLength)

		// Init the state encryptor
		se := queryStateEncryptor{}
		err = se.init(validator.log, executeStateKey, deployTxKey)
		if err != nil {
			return nil, err
		}

		return &se, nil
	}

	// Compute deployTxKey key from the deploy transaction
	deployTxKey := utils.HMAC(deployStateKey, deployTx.Nonce)

	// Mask executeTx.Nonce
	executeTxNonce := utils.HMACTruncated(deployTxKey, utils.Hash(executeTx.Nonce), utils.NonceSize)

	// Compute stateKey to encrypt the states and nonceStateKey to generates IVs. This
	// allows validators to reach consesus
	stateKey := utils.HMACTruncated(deployTxKey, append([]byte{3}, executeTxNonce...), utils.AESKeyLength)
	nonceStateKey := utils.HMAC(deployTxKey, append([]byte{4}, executeTxNonce...))

	// Init the state encryptor
	se := stateEncryptorImpl{}
	err = se.init(validator.log, stateKey, nonceStateKey, deployTxKey, executeTxNonce)
	if err != nil {
		return nil, err
	}

	return &se, nil
}

func (validator *validatorImpl) getStateKeyFromTransaction(tx *obc.Transaction) ([]byte, error) {
	es, err := generic.NewEncryptionSchemeFromPrivateKey(validator.chainPrivateKey)
	if err != nil {
		validator.log.Error("Failed init decryption engine [%s].", err.Error())
		return nil, err
	}

	validator.log.Debug("Decrypting message to validators [%s].", utils.EncodeBase64(tx.Key))

	msgToValidatorsRaw, err := es.Process(tx.Key)
	if err != nil {
		validator.log.Error("Failed decrypting transaction key [%s].", err.Error())
		return nil, err
	}

	msgToValidators := new(chainCodeValidatorMessage1_2)
	_, err = asn1.Unmarshal(msgToValidatorsRaw, msgToValidators)
	if err != nil {
		validator.log.Error("Failed unmarshalling message to validators [%s].", err.Error())
		return nil, err
	}

	return msgToValidators.StateKey, nil
}
