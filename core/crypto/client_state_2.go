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
	"encoding/asn1"
	"github.com/hyperledger/fabric/core/crypto/primitives"
	"github.com/hyperledger/fabric/core/crypto/utils"
	obc "github.com/hyperledger/fabric/protos"
)

type clientQueryResultDecryptorV2 struct {
	client *clientImpl
}

func (qrd *clientQueryResultDecryptorV2) getVersion() string {
	return "2.0"
}

// DecryptQueryResult is used to decrypt the result of a query transaction
func (qrd *clientQueryResultDecryptorV2) decryptQueryResult(queryTx *obc.Transaction, ct []byte) ([]byte, error) {
	// Verify that the client is initialized
	if !qrd.client.isInitialized {
		return nil, utils.ErrNotInitialized
	}

	queryKey, err := qrd.getStateKey(queryTx)

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

func (qrd *clientQueryResultDecryptorV2) getStateKey(queryTx *obc.Transaction) ([]byte, error) {

	userMessages := new(eUserMessagesV2)
	_, err := asn1.Unmarshal(queryTx.ToUsers, userMessages)
	if err != nil {
		qrd.client.error("Failed unmarshalling user message: [%s]", err)

		return nil, err
	}

	aCipher, err := qrd.client.acSPI.NewAsymmetricCipherFromPrivateKey(qrd.client.enrollEncryptionKey)
	if err != nil {
		qrd.client.error("Failed initiliazing encryption scheme: [%s]", err)

		return nil, err
	}
	invokerMessageRaw, err := aCipher.Process(userMessages.InvokerMessage)
	if err != nil {
		qrd.client.error("Failed decrypting user message: [%s]", err)

		return nil, err
	}

	invokerMessage := new(eInvokerMessageV2)
	_, err = asn1.Unmarshal(invokerMessageRaw, invokerMessage)
	if err != nil {
		qrd.client.error("Failed unmarshalling creator message: [%s]", err)

		return nil, err
	}
	if invokerMessage.Validate() != err {
		qrd.client.error("Failed validating creator message: [%s]", err)

		return nil, err
	}

	return invokerMessage.KInvoke, nil
}
