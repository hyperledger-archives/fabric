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

	"crypto/aes"
	"crypto/cipher"
	"encoding/asn1"
	"encoding/binary"
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

	validator.debug("Parsing transaction. Type [%s]. Confidentiality Protocol Version [%d]", executeTx.Type.String(), executeTx.ConfidentialityProtocolVersion)

	// client.enrollChainKey is an AES key represented as byte array
	enrollChainKey := validator.enrollChainKey.([]byte)

	if executeTx.Type == obc.Transaction_CHAINCODE_QUERY {
		validator.debug("Parsing Query transaction...")

		// Compute deployTxKey key from the deploy transaction. This is used to decrypt the actual state
		// of the chaincode
		deployTxKey := utils.HMAC(enrollChainKey, deployTx.Nonce)

		// Compute the key used to encrypt the result of the query
		queryKey := utils.HMACTruncated(enrollChainKey, append([]byte{6}, executeTx.Nonce...), utils.AESKeyLength)

		// Init the state encryptor
		se := queryStateEncryptor{}
		err := se.init(validator.nodeImpl, queryKey, deployTxKey)
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
	err := se.init(validator.nodeImpl, stateKey, nonceStateKey, deployTxKey, executeTxNonce)
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

	validator.debug("Parsing transaction. Type [%s]. Confidentiality Protocol Version [%s]", executeTx.Type.String(), executeTx.ConfidentialityProtocolVersion)

	deployStateKey, err := validator.getStateKeyFromTransaction(deployTx)

	if executeTx.Type == obc.Transaction_CHAINCODE_QUERY {
		validator.debug("Parsing Query transaction...")

		executeStateKey, err := validator.getStateKeyFromTransaction(executeTx)

		// Compute deployTxKey key from the deploy transaction. This is used to decrypt the actual state
		// of the chaincode
		deployTxKey := utils.HMAC(deployStateKey, deployTx.Nonce)

		// Compute the key used to encrypt the result of the query
		//queryKey := utils.HMACTruncated(executeStateKey, append([]byte{6}, executeTx.Nonce...), utils.AESKeyLength)

		// Init the state encryptor
		se := queryStateEncryptor{}
		err = se.init(validator.nodeImpl, executeStateKey, deployTxKey)
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
	err = se.init(validator.nodeImpl, stateKey, nonceStateKey, deployTxKey, executeTxNonce)
	if err != nil {
		return nil, err
	}

	return &se, nil
}

func (validator *validatorImpl) getStateKeyFromTransaction(tx *obc.Transaction) ([]byte, error) {
	cipher, err := validator.eciesSPI.NewAsymmetricCipherFromPrivateKey(validator.chainPrivateKey)
	if err != nil {
		validator.error("Failed init decryption engine [%s].", err.Error())
		return nil, err
	}

	validator.debug("Decrypting message to validators [% x].", tx.ToValidators)

	msgToValidatorsRaw, err := cipher.Process(tx.ToValidators)
	if err != nil {
		validator.error("Failed decrypting transaction key [%s].", err.Error())
		return nil, err
	}

	msgToValidators := new(chainCodeValidatorMessage1_2)
	_, err = asn1.Unmarshal(msgToValidatorsRaw, msgToValidators)
	if err != nil {
		validator.error("Failed unmarshalling message to validators [%s].", err.Error())
		return nil, err
	}

	return msgToValidators.StateKey, nil
}

type stateEncryptorImpl struct {
	node *nodeImpl

	deployTxKey   []byte
	invokeTxNonce []byte

	stateKey      []byte
	nonceStateKey []byte

	gcmEnc    cipher.AEAD
	nonceSize int

	counter uint64
}

func (se *stateEncryptorImpl) init(node *nodeImpl, stateKey, nonceStateKey, deployTxKey, invokeTxNonce []byte) error {
	// Initi fields
	se.counter = 0
	se.node = node
	se.stateKey = stateKey
	se.nonceStateKey = nonceStateKey
	se.deployTxKey = deployTxKey
	se.invokeTxNonce = invokeTxNonce

	// Init aes
	c, err := aes.NewCipher(se.stateKey)
	if err != nil {
		return err
	}

	// Init gcm for encryption
	se.gcmEnc, err = cipher.NewGCM(c)
	if err != nil {
		return err
	}

	// Init nonce size
	se.nonceSize = se.gcmEnc.NonceSize()
	return nil
}

func (se *stateEncryptorImpl) Encrypt(msg []byte) ([]byte, error) {
	var b = make([]byte, 8)
	binary.BigEndian.PutUint64(b, se.counter)

	se.node.debug("Encrypting with counter [% x].", b)
	//	se.log.Info("Encrypting with txNonce  ", utils.EncodeBase64(se.txNonce))

	nonce := utils.HMACTruncated(se.nonceStateKey, b, se.nonceSize)

	se.counter++

	// Seal will append the output to the first argument; the usage
	// here appends the ciphertext to the nonce. The final parameter
	// is any additional data to be authenticated.
	out := se.gcmEnc.Seal(nonce, nonce, msg, se.invokeTxNonce)

	return append(se.invokeTxNonce, out...), nil
}

func (se *stateEncryptorImpl) Decrypt(raw []byte) ([]byte, error) {
	if len(raw) <= utils.NonceSize {
		return nil, utils.ErrDecrypt
	}

	// raw consists of (txNonce, ct)
	txNonce := raw[:utils.NonceSize]
	//	se.log.Info("Decrypting with txNonce  ", utils.EncodeBase64(txNonce))
	ct := raw[utils.NonceSize:]

	nonce := make([]byte, se.nonceSize)
	copy(nonce, ct)

	key := utils.HMACTruncated(se.deployTxKey, append([]byte{3}, txNonce...), utils.AESKeyLength)
	//	se.log.Info("Decrypting with key  ", utils.EncodeBase64(key))
	c, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(c)
	if err != nil {
		return nil, err
	}

	se.nonceSize = se.gcmEnc.NonceSize()

	out, err := gcm.Open(nil, nonce, ct[se.nonceSize:], txNonce)
	if err != nil {
		return nil, utils.ErrDecrypt
	}
	return out, nil
}

type queryStateEncryptor struct {
	node *nodeImpl

	deployTxKey []byte

	gcmEnc    cipher.AEAD
	nonceSize int
}

func (se *queryStateEncryptor) init(node *nodeImpl, queryKey, deployTxKey []byte) error {
	// Initi fields
	se.node = node
	se.deployTxKey = deployTxKey

	//	se.log.Info("QUERY Encrypting with key  ", utils.EncodeBase64(queryKey))

	// Init aes
	c, err := aes.NewCipher(queryKey)
	if err != nil {
		return err
	}

	// Init gcm for encryption
	se.gcmEnc, err = cipher.NewGCM(c)
	if err != nil {
		return err
	}

	// Init nonce size
	se.nonceSize = se.gcmEnc.NonceSize()
	return nil
}

func (se *queryStateEncryptor) Encrypt(msg []byte) ([]byte, error) {
	nonce, err := utils.GetRandomBytes(se.nonceSize)
	if err != nil {
		se.node.error("Failed getting randomness [%s].", err.Error())
		return nil, err
	}

	// Seal will append the output to the first argument; the usage
	// here appends the ciphertext to the nonce. The final parameter
	// is any additional data to be authenticated.
	out := se.gcmEnc.Seal(nonce, nonce, msg, nil)

	return out, nil
}

func (se *queryStateEncryptor) Decrypt(raw []byte) ([]byte, error) {
	if len(raw) <= utils.NonceSize {
		return nil, utils.ErrDecrypt
	}

	// raw consists of (txNonce, ct)
	txNonce := raw[:utils.NonceSize]
	//	se.log.Info("Decrypting with txNonce  ", utils.EncodeBase64(txNonce))
	ct := raw[utils.NonceSize:]

	nonce := make([]byte, se.nonceSize)
	copy(nonce, ct)

	key := utils.HMACTruncated(se.deployTxKey, append([]byte{3}, txNonce...), utils.AESKeyLength)
	//	se.log.Info("Decrypting with key  ", utils.EncodeBase64(key))
	c, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(c)
	if err != nil {
		return nil, err
	}

	se.nonceSize = se.gcmEnc.NonceSize()

	out, err := gcm.Open(nil, nonce, ct[se.nonceSize:], txNonce)
	if err != nil {
		return nil, utils.ErrDecrypt
	}
	return out, nil
}
