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
	"encoding/asn1"
	"errors"
	"github.com/golang/protobuf/proto"
	"github.com/openblockchain/obc-peer/openchain/crypto/ecies"
	"github.com/openblockchain/obc-peer/openchain/crypto/utils"
	obc "github.com/openblockchain/obc-peer/protos"
)

func (validator *validatorImpl) deepCloneTransaction(tx *obc.Transaction) (*obc.Transaction, error) {
	raw, err := proto.Marshal(tx)
	if err != nil {
		validator.error("Failed cloning transaction [%s].", err.Error())

		return nil, err
	}

	clone := &obc.Transaction{}
	err = proto.Unmarshal(raw, clone)
	if err != nil {
		validator.error("Failed cloning transaction [%s].", err.Error())

		return nil, err
	}

	return clone, nil
}

func (validator *validatorImpl) deepCloneAndDecryptTx(tx *obc.Transaction) (*obc.Transaction, error) {
	switch tx.ConfidentialityProtocolVersion {
	case "1.1":
		return validator.deepCloneAndDecryptTx1_1(tx)
	case "1.2":
		return validator.deepCloneAndDecryptTx1_2(tx)
	}
	return nil, utils.ErrInvalidProtocolVersion
}

func (validator *validatorImpl) deepCloneAndDecryptTx1_1(tx *obc.Transaction) (*obc.Transaction, error) {
	if tx.Nonce == nil || len(tx.Nonce) == 0 {
		return nil, errors.New("Failed decrypting payload. Invalid nonce.")
	}

	// clone tx
	clone, err := validator.deepCloneTransaction(tx)
	if err != nil {
		validator.error("Failed deep cloning [%s].", err.Error())
		return nil, err
	}

	// Derive root key
	// client.enrollChainKey is an AES key represented as byte array
	enrollChainKey := validator.enrollChainKey.([]byte)

	key := utils.HMAC(enrollChainKey, clone.Nonce)

	//	validator.log.Info("Deriving from  ", utils.EncodeBase64(validator.peer.node.enrollChainKey))
	//	validator.log.Info("Nonce  ", utils.EncodeBase64(tx.Nonce))
	//	validator.log.Info("Derived key  ", utils.EncodeBase64(key))
	//	validator.log.Info("Encrypted Payload  ", utils.EncodeBase64(tx.EncryptedPayload))
	//	validator.log.Info("Encrypted ChaincodeID  ", utils.EncodeBase64(tx.EncryptedChaincodeID))

	// Decrypt Payload
	payloadKey := utils.HMACTruncated(key, []byte{1}, utils.AESKeyLength)
	payload, err := utils.CBCPKCS7Decrypt(payloadKey, utils.Clone(clone.Payload))
	if err != nil {
		validator.error("Failed decrypting payload [%s].", err.Error())
		return nil, err
	}
	clone.Payload = payload

	// Decrypt ChaincodeID
	chaincodeIDKey := utils.HMACTruncated(key, []byte{2}, utils.AESKeyLength)
	chaincodeID, err := utils.CBCPKCS7Decrypt(chaincodeIDKey, utils.Clone(clone.ChaincodeID))
	if err != nil {
		validator.error("Failed decrypting chaincode [%s].", err.Error())
		return nil, err
	}
	clone.ChaincodeID = chaincodeID

	// Decrypt metadata
	if len(clone.Metadata) != 0 {
		metadataKey := utils.HMACTruncated(key, []byte{3}, utils.AESKeyLength)
		metadata, err := utils.CBCPKCS7Decrypt(metadataKey, utils.Clone(clone.Metadata))
		if err != nil {
			validator.error("Failed decrypting metadata [%s].", err.Error())
			return nil, err
		}
		clone.Metadata = metadata
	}

	return clone, nil
}

func (validator *validatorImpl) deepCloneAndDecryptTx1_2(tx *obc.Transaction) (*obc.Transaction, error) {
	if tx.Nonce == nil || len(tx.Nonce) == 0 {
		return nil, errors.New("Failed decrypting payload. Invalid nonce.")
	}

	// clone tx
	clone, err := validator.deepCloneTransaction(tx)
	if err != nil {
		validator.error("Failed deep cloning [%s].", err.Error())
		return nil, err
	}

	var ccPrivateKey ecies.PrivateKey

	validator.debug("Transaction type [%s].", tx.Type.String())

	validator.debug("Extract transaction key...")

	// Derive transaction key
	cipher, err := validator.eciesSPI.NewAsymmetricCipherFromPrivateKey(validator.chainPrivateKey)
	if err != nil {
		validator.error("Failed init decryption engine [%s].", err.Error())
		return nil, err
	}

	validator.debug("Decrypting message to validators [% x].", tx.ToValidators)

	msgToValidatorsRaw, err := cipher.Process(tx.ToValidators)
	if err != nil {
		validator.error("Failed decrypting message to validators [%s].", err.Error())
		return nil, err
	}

	msgToValidators := new(chainCodeValidatorMessage1_2)
	_, err = asn1.Unmarshal(msgToValidatorsRaw, msgToValidators)
	if err != nil {
		validator.error("Failed unmarshalling message to validators [%s].", err.Error())
		return nil, err
	}

	validator.debug("Deserializing transaction key [% x].", msgToValidators.PrivateKey)
	ccPrivateKey, err = validator.eciesSPI.DeserializePrivateKey(msgToValidators.PrivateKey)
	if err != nil {
		validator.error("Failed deserializing transaction key [%s].", err.Error())
		return nil, err
	}

	validator.debug("Extract transaction key...done")

	cipher, err = validator.eciesSPI.NewAsymmetricCipherFromPrivateKey(ccPrivateKey)
	if err != nil {
		validator.error("Failed init transaction decryption engine [%s].", err.Error())
		return nil, err
	}
	// Decrypt Payload
	payload, err := cipher.Process(clone.Payload)
	if err != nil {
		validator.error("Failed decrypting payload [%s].", err.Error())
		return nil, err
	}
	clone.Payload = payload

	// Decrypt ChaincodeID
	chaincodeID, err := cipher.Process(clone.ChaincodeID)
	if err != nil {
		validator.error("Failed decrypting chaincode [%s].", err.Error())
		return nil, err
	}
	clone.ChaincodeID = chaincodeID

	// Decrypt metadata
	if len(clone.Metadata) != 0 {
		metadata, err := cipher.Process(clone.Metadata)
		if err != nil {
			validator.error("Failed decrypting metadata [%s].", err.Error())
			return nil, err
		}
		clone.Metadata = metadata
	}

	return clone, nil
}
