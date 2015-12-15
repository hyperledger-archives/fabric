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
"github.com/op/go-logging"
)

func (validator *validatorImpl) decryptTx(tx *obc.Transaction) error {

	// Derive root key
	if tx.Nonce == nil || len(tx.Nonce) == 0 {
		return errors.New("Failed decrypting payload. Invalid nonce.")
	}
	key := utils.HMAC(validator.peer.node.enrollChainKey, tx.Nonce)

	validator.peer.node.log.Info("Deriving from %s", utils.EncodeBase64(validator.peer.node.enrollChainKey))
	validator.peer.node.log.Info("Nonce %s", utils.EncodeBase64(tx.Nonce))
	validator.peer.node.log.Info("Derived key %s", utils.EncodeBase64(key))
	validator.peer.node.log.Info("Encrypted Payload %s", utils.EncodeBase64(tx.EncryptedPayload))
	validator.peer.node.log.Info("Encrypted ChaincodeID %s", utils.EncodeBase64(tx.EncryptedChaincodeID))

	// Decrypt using the derived key

	payloadKey := utils.HMACTruncated(key, []byte{1}, utils.AESKeyLength)
	encryptedPayload := make([]byte, len(tx.EncryptedPayload))
	copy(encryptedPayload, tx.EncryptedPayload)
	payload, err := utils.CBCPKCS7Decrypt(payloadKey, encryptedPayload)
	if err != nil {
		validator.peer.node.log.Error("Failed decrypting payload %s", err)
		return err
	}
	tx.Payload = payload

	chaincodeIdKey := utils.HMACTruncated(key, []byte{2}, utils.AESKeyLength)
	encryptedChaincodeID := make([]byte, len(tx.EncryptedChaincodeID))
	copy(encryptedChaincodeID, tx.EncryptedChaincodeID)
	rawChaincodeID, err := utils.CBCPKCS7Decrypt(chaincodeIdKey, encryptedChaincodeID)

	chaincodeID := &obc.ChaincodeID{}
	if err := proto.Unmarshal(rawChaincodeID, chaincodeID); err != nil {
		validator.peer.node.log.Error("Failed decrypting chaincodeID %s", err)

		// Cleanup the decrypted values so far

		tx.Payload = nil
		tx.ChaincodeID = nil

		return err
	}
	tx.ChaincodeID = chaincodeID

	return nil
}

type stateEncryptorImpl struct {
	log *logging.Logger

	txKey []byte
	txNonce []byte

	stateKey []byte
	nonceStateKey []byte

	gcmEnc    cipher.AEAD
	nonceSize int

	counter uint64
}

func (sei *stateEncryptorImpl) init(logger *logging.Logger, stateKey, nonceStateKey, txKey, txNonce []byte) error {
	// Initi fields
	sei.counter = 0
	sei.log = logger
	sei.stateKey = stateKey
	sei.nonceStateKey = nonceStateKey
	sei.txKey = txKey
	sei.txNonce = txNonce

	log.Info("State key %s", utils.EncodeBase64(sei.stateKey))

	// Init aes
	c, err := aes.NewCipher(sei.stateKey)
	if err != nil {
		return err
	}

	// Init gcm for encryption
	sei.gcmEnc, err = cipher.NewGCM(c)
	if err != nil {
		return err
	}

	// Init nonce size
	sei.nonceSize = sei.gcmEnc.NonceSize()
	return nil
}

func (sei *stateEncryptorImpl) Encrypt(msg []byte) ([]byte, error) {
	var b = make([]byte, 8)
	binary.BigEndian.PutUint64(b, sei.counter)
	// TODO: log from validator
	sei.log.Info("Encrypting with counter %s", utils.EncodeBase64(b))
//	sei.log.Info("Encrypting with txNonce %s", utils.EncodeBase64(sei.txNonce))

	nonce := utils.HMACTruncated(sei.nonceStateKey, b, sei.nonceSize)

	sei.counter++

	// Seal will append the output to the first argument; the usage
	// here appends the ciphertext to the nonce. The final parameter
	// is any additional data to be authenticated.
	out := sei.gcmEnc.Seal(nonce, nonce, msg, sei.txNonce)

	return append(sei.txNonce, out...), nil
}

func (sei *stateEncryptorImpl) Decrypt(raw []byte) ([]byte, error) {
	if len(raw) <= utils.NonceSize {
		return nil, utils.ErrDecrypt
	}

	// raw consists of (txNonce, ct)
	txNonce := raw[:utils.NonceSize]
//	sei.log.Info("Decrypting with txNonce %s", utils.EncodeBase64(txNonce))
	ct := raw[utils.NonceSize:]

	nonce := make([]byte, sei.nonceSize)
	copy(nonce, ct)

	key := utils.HMACTruncated(sei.txKey, append([]byte{3}, txNonce...), utils.AESKeyLength)
//	sei.log.Info("Decrypting with key %s", utils.EncodeBase64(key))
	c, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(c)
	if err != nil {
		return nil, err
	}

	sei.nonceSize = sei.gcmEnc.NonceSize()

	out, err := gcm.Open(nil, nonce, ct[sei.nonceSize:], txNonce)
	if err != nil {
		return nil, utils.ErrDecrypt
	}
	return out, nil
}
