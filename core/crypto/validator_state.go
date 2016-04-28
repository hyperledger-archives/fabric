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
	"encoding/binary"
	"github.com/hyperledger/fabric/core/crypto/primitives"
	"github.com/hyperledger/fabric/core/crypto/utils"
	obc "github.com/hyperledger/fabric/protos"
)

var (
	dummyStateEncryptor = &dummyStateEncryptorImpl{}
)

func (validator *validatorImpl) addStateProcessor(sp StateProcessor) (err error) {
	validator.stateProcessors[sp.getVersion()] = sp

	return
}

func (validator *validatorImpl) initStateProcessors() (err error) {
	validator.stateProcessors = make(map[string]StateProcessor)

	// Init confidentiality processors
	validator.addStateProcessor(&validatorStateProcessorV1_1{validator})
	validator.addStateProcessor(&validatorStateProcessorV1_2{validator})
	validator.addStateProcessor(&validatorStateProcessorV2{validator})

	return
}

func (validator *validatorImpl) GetStateEncryptor(deployTx, executeTx *obc.Transaction) (StateEncryptor, error) {

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

	if deployTx.ConfidentialityLevel == obc.ConfidentialityLevel_PUBLIC &&
		executeTx.ConfidentialityLevel == obc.ConfidentialityLevel_PUBLIC {
		// return dummy state encryptor
		return dummyStateEncryptor, nil
	}

	if deployTx.ConfidentialityLevel == obc.ConfidentialityLevel_PUBLIC &&
		executeTx.ConfidentialityLevel == obc.ConfidentialityLevel_CONFIDENTIAL {
		// TODO: this is not necessary... should return an error here

		return nil, errors.New("[PUBLIC, CONFIDENTIAL] Not implemented yet")
	}

	if deployTx.ConfidentialityLevel == obc.ConfidentialityLevel_CONFIDENTIAL &&
		executeTx.ConfidentialityLevel == obc.ConfidentialityLevel_PUBLIC {
		// TODO: this is not necessary... should return an error here

		return nil, errors.New("[CONFIDENTIAL, PUBLIC] Not implemented yet")
	}

	validator.debug("Confidentiality Procotol Version [%s]", executeTx.ConfidentialityProtocolVersion)
	processor, ok := validator.stateProcessors[executeTx.ConfidentialityProtocolVersion]
	if !ok {
		return nil, utils.ErrInvalidProtocolVersion
	}

	return processor.getStateEncryptor(deployTx, executeTx)
}

type dummyStateEncryptorImpl struct{}

func (se *dummyStateEncryptorImpl) Encrypt(msg []byte) ([]byte, error) {
	return msg, nil
}

func (se *dummyStateEncryptorImpl) Decrypt(raw []byte) ([]byte, error) {
	return raw, nil
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

	nonce := primitives.HMACTruncated(se.nonceStateKey, b, se.nonceSize)

	se.counter++

	// Seal will append the output to the first argument; the usage
	// here appends the ciphertext to the nonce. The final parameter
	// is any additional data to be authenticated.
	out := se.gcmEnc.Seal(nonce, nonce, msg, se.invokeTxNonce)

	return append(se.invokeTxNonce, out...), nil
}

func (se *stateEncryptorImpl) Decrypt(raw []byte) ([]byte, error) {
	if len(raw) <= primitives.NonceSize {
		return nil, utils.ErrDecrypt
	}

	// raw consists of (txNonce, ct)
	txNonce := raw[:primitives.NonceSize]
	//	se.log.Info("Decrypting with txNonce  ", utils.EncodeBase64(txNonce))
	ct := raw[primitives.NonceSize:]

	nonce := make([]byte, se.nonceSize)
	copy(nonce, ct)

	key := primitives.HMACTruncated(se.deployTxKey, append([]byte{3}, txNonce...), primitives.AESKeyLength)
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
	nonce, err := primitives.GetRandomBytes(se.nonceSize)
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
	if len(raw) <= primitives.NonceSize {
		return nil, utils.ErrDecrypt
	}

	// raw consists of (txNonce, ct)
	txNonce := raw[:primitives.NonceSize]
	//	se.log.Info("Decrypting with txNonce  ", utils.EncodeBase64(txNonce))
	ct := raw[primitives.NonceSize:]

	nonce := make([]byte, se.nonceSize)
	copy(nonce, ct)

	key := primitives.HMACTruncated(se.deployTxKey, append([]byte{3}, txNonce...), primitives.AESKeyLength)
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
