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
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"errors"
	"github.com/golang/protobuf/proto"
	"github.com/openblockchain/obc-peer/openchain/crypto/utils"
	obc "github.com/openblockchain/obc-peer/protos"
)

func (validator *validatorImpl) deepCloneAndDecryptTx(tx *obc.Transaction) (*obc.Transaction, error) {
	if tx.Nonce == nil || len(tx.Nonce) == 0 {
		return nil, errors.New("Failed decrypting payload. Invalid nonce.")
	}

	// clone tx
	clone, err := validator.deepCloneTransaction(tx)
	if err != nil {
		validator.peer.node.error("Failed deep cloning [%s].", err.Error())
		return nil, err
	}

	// Derive root key
	key := utils.HMAC(validator.peer.node.enrollChainKey, clone.Nonce)

	//	validator.peer.node.info("Deriving from  ", utils.EncodeBase64(validator.peer.node.enrollChainKey))
	//	validator.peer.node.info("Nonce  ", utils.EncodeBase64(tx.Nonce))
	//	validator.peer.node.info("Derived key  ", utils.EncodeBase64(key))
	//	validator.peer.node.info("Encrypted Payload  ", utils.EncodeBase64(tx.EncryptedPayload))
	//	validator.peer.node.info("Encrypted ChaincodeID  ", utils.EncodeBase64(tx.EncryptedChaincodeID))

	// Decrypt Payload
	payloadKey := utils.HMACTruncated(key, []byte{1}, utils.AESKeyLength)
	payload, err := utils.CBCPKCS7Decrypt(payloadKey, utils.Clone(clone.Payload))
	if err != nil {
		validator.peer.node.error("Failed decrypting payload [%s].", err.Error())
		return nil, err
	}
	clone.Payload = payload

	// Decrypt ChaincodeID
	chaincodeIDKey := utils.HMACTruncated(key, []byte{2}, utils.AESKeyLength)
	chaincodeID, err := utils.CBCPKCS7Decrypt(chaincodeIDKey, utils.Clone(clone.ChaincodeID))
	if err != nil {
		validator.peer.node.error("Failed decrypting chaincode [%s].", err.Error())
		return nil, err
	}
	clone.ChaincodeID = chaincodeID

	// Decrypt metadata
	if len(clone.Metadata) != 0 {
		metadataKey := utils.HMACTruncated(key, []byte{3}, utils.AESKeyLength)
		metadata, err := utils.CBCPKCS7Decrypt(metadataKey, utils.Clone(clone.Metadata))
		if err != nil {
			validator.peer.node.error("Failed decrypting metadata [%s].", err.Error())
			return nil, err
		}
		clone.Metadata = metadata
	}

	return clone, nil
}

func (validator *validatorImpl) deepCloneTransaction(tx *obc.Transaction) (*obc.Transaction, error) {
	raw, err := proto.Marshal(tx)
	if err != nil {
		validator.peer.node.error("Failed cloning transaction [%s].", err.Error())

		return nil, err
	}

	clone := &obc.Transaction{}
	err = proto.Unmarshal(raw, clone)
	if err != nil {
		validator.peer.node.error("Failed cloning transaction [%s].", err.Error())

		return nil, err
	}

	return clone, nil
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
