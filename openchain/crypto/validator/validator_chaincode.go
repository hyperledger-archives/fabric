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
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"errors"
	"github.com/golang/protobuf/proto"
	"github.com/openblockchain/obc-peer/openchain/crypto/utils"
	obc "github.com/openblockchain/obc-peer/protos"
)

func (validator *Validator) decryptTx(tx *obc.Transaction) error {

	// Derive root key
	if tx.Nonce == nil || len(tx.Nonce) == 0 {
		return errors.New("Failed decrypting payload. Invalid nonce.")
	}
	key := utils.HMAC(validator.enrollChainKey, tx.Nonce)

	log.Info("Deriving from %s", utils.EncodeBase64(validator.enrollChainKey))
	log.Info("Nonce %s", utils.EncodeBase64(tx.Nonce))
	log.Info("Derived key %s", utils.EncodeBase64(key))
	log.Info("Encrypted Payload %s", utils.EncodeBase64(tx.EncryptedPayload))
	log.Info("Encrypted ChaincodeID %s", utils.EncodeBase64(tx.EncryptedChaincodeID))

	// Decrypt using the derived key

	payloadKey := utils.HMACTruncated(key, []byte{1}, utils.AESKeyLength)
	encryptedPayload := make([]byte, len(tx.EncryptedPayload))
	copy(encryptedPayload, tx.EncryptedPayload)
	payload, err := utils.CBCPKCS7Decrypt(payloadKey, encryptedPayload)
	if err != nil {
		log.Error("Failed decrypting payload %s", err)
		return err
	}
	tx.Payload = payload

	chaincodeIdKey := utils.HMACTruncated(key, []byte{2}, utils.AESKeyLength)
	encryptedChaincodeID := make([]byte, len(tx.EncryptedChaincodeID))
	copy(encryptedChaincodeID, tx.EncryptedChaincodeID)
	rawChaincodeID, err := utils.CBCPKCS7Decrypt(chaincodeIdKey, encryptedChaincodeID)

	chaincodeID := &obc.ChaincodeID{}
	if err := proto.Unmarshal(rawChaincodeID, chaincodeID); err != nil {
		log.Error("Failed decrypting chaincodeID %s", err)
		// TODO: cleanup the decrypted values
		return err
	}
	tx.ChaincodeID = chaincodeID

	return nil
}

func (validator *Validator) GetStateEncryptionScheme(deployTx, invokeTx *obc.Transaction) (EncryptionScheme, error) {
	// Derive root key from the deploy transaction
	if deployTx.Nonce == nil || len(deployTx.Nonce) == 0 {
		return nil, errors.New("Failed getting state ES. Invalid deploy nonce.")
	}
	if invokeTx.Nonce == nil || len(invokeTx.Nonce) == 0 {
		return nil, errors.New("Failed getting state ES. Invalid invoke nonce.")
	}
	// TODO: check that deployTx and invokeTx refers to the same chaincode

	rootKey := utils.HMAC(validator.enrollChainKey, deployTx.Nonce)

	aesKey := utils.HMACTruncated(rootKey, append([]byte{3}, invokeTx.Nonce...), utils.AESKeyLength)
	nonceKey := utils.HMAC(rootKey, append([]byte{4}, invokeTx.Nonce...))

	ses := stateEncryptionScheme{aesKey: aesKey, nonceKey: nonceKey}
	err := ses.init()
	if err != nil {
		return nil, err
	}

	return &ses, nil
}

type stateEncryptionScheme struct {
	aesKey   []byte
	nonceKey []byte

	gcmEnc    cipher.AEAD
	gcmDec    cipher.AEAD
	nonceSize int

	counter uint64
}

func (ses *stateEncryptionScheme) init() error {
	ses.counter = 0

	c, err := aes.NewCipher(ses.aesKey)
	if err != nil {
		return err
	}

	ses.gcmEnc, err = cipher.NewGCM(c)
	if err != nil {
		return err
	}

	c1, err := aes.NewCipher(ses.aesKey)
	if err != nil {
		return err
	}

	ses.gcmDec, err = cipher.NewGCM(c1)
	if err != nil {
		return err
	}

	ses.nonceSize = ses.gcmEnc.NonceSize()

	return nil
}

func (ses *stateEncryptionScheme) Encrypt(msg []byte) ([]byte, error) {
	var b = make([]byte, 8)
	binary.BigEndian.PutUint64(b, ses.counter)
	log.Info("b %s", utils.EncodeBase64(b))

	nonce := utils.HMACTruncated(ses.nonceKey, b, ses.nonceSize)

	ses.counter++

	// Seal will append the output to the first argument; the usage
	// here appends the ciphertext to the nonce. The final parameter
	// is any additional data to be authenticated.
	out := ses.gcmEnc.Seal(nonce, nonce, msg, nil)

	return out, nil
}

func (ses *stateEncryptionScheme) Decrypt(ct []byte) ([]byte, error) {
	if len(ct) <= utils.NonceSize {
		return nil, ErrDecrypt
	}

	nonce := make([]byte, ses.nonceSize)
	copy(nonce, ct)

	out, err := ses.gcmDec.Open(nil, nonce, ct[ses.nonceSize:], nil)
	if err != nil {
		return nil, ErrDecrypt
	}
	return out, nil

	return nil, nil
}
