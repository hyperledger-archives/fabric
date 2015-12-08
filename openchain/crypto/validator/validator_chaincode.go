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
	"errors"
	"github.com/openblockchain/obc-peer/openchain/crypto/utils"
	obc "github.com/openblockchain/obc-peer/protos"
)

func (validator *Validator) decryptPayload(tx *obc.Transaction) ([]byte, error) {

	// Derive key
	if tx.Nonce == nil || len(tx.Nonce) == 0 {
		return nil, errors.New("Failed decrypting payload. Invalid nonce.")
	}
	key := utils.HMACTruncated(validator.enrollChainKey, tx.Nonce, utils.AES_KEY_LENGTH_BYTES)
	//	log.Info("Deriving from %s", utils.EncodeBase64(validator.enrollChainKey))
	//	log.Info("Nonce %s", utils.EncodeBase64(tx.Nonce))
	//	log.Info("Derived key %s", utils.EncodeBase64(key))
	//	log.Info("Payload %s", utils.EncodeBase64(tx.Payload))

	// Encrypt using the derived key
	ct := make([]byte, len(validator.id))
	copy(ct, tx.Payload)
	return utils.CBCPKCS7Decrypt(key, ct)
}
