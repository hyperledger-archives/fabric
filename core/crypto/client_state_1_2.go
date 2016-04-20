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
	"github.com/hyperledger/fabric/core/crypto/primitives"
	"github.com/hyperledger/fabric/core/crypto/utils"
	obc "github.com/hyperledger/fabric/protos"
)

type clientQueryResultDecryptorV1_2 struct {
	client *clientImpl
}

func (qrd *clientQueryResultDecryptorV1_2) getVersion() string {
	return "1.2"
}

// DecryptQueryResult is used to decrypt the result of a query transaction
func (qrd *clientQueryResultDecryptorV1_2) decryptQueryResult(queryTx *obc.Transaction, ct []byte) ([]byte, error) {
	// Verify that the client is initialized
	if !qrd.client.isInitialized {
		return nil, utils.ErrNotInitialized
	}

	var queryKey []byte
	queryKey = primitives.HMACTruncated(qrd.client.queryStateKey, append([]byte{6}, queryTx.Nonce...), primitives.AESKeyLength)

	if len(ct) <= primitives.NonceSize {
		return nil, utils.ErrDecrypt
	}

	c, err := aes.NewCipher(queryKey)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(c)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	copy(nonce, ct)

	out, err := gcm.Open(nil, nonce, ct[gcm.NonceSize():], nil)
	if err != nil {
		qrd.client.error("Failed decrypting query result [%s].", err.Error())
		return nil, utils.ErrDecrypt
	}
	return out, nil
}
