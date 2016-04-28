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
	"github.com/hyperledger/fabric/core/crypto/primitives"
	obc "github.com/hyperledger/fabric/protos"
)

type validatorStateProcessorV1_1 struct {
	validator *validatorImpl
}

func (sp *validatorStateProcessorV1_1) getVersion() string {
	return "1.1"
}

func (sp *validatorStateProcessorV1_1) getStateEncryptor(deployTx, executeTx *obc.Transaction) (StateEncryptor, error) {
	sp.validator.debug("Parsing transaction. Type [%s]. Confidentiality Protocol Version [%d]", executeTx.Type.String(), executeTx.ConfidentialityProtocolVersion)

	// client.enrollChainKey is an AES key represented as byte array
	enrollChainKey := sp.validator.enrollSymChainKey

	if executeTx.Type == obc.Transaction_CHAINCODE_QUERY {
		sp.validator.debug("Parsing Query transaction...")

		// Compute deployTxKey key from the deploy transaction. This is used to decrypt the actual state
		// of the chaincode
		deployTxKey := primitives.HMAC(enrollChainKey, deployTx.Nonce)

		// Compute the key used to encrypt the result of the query
		queryKey := primitives.HMACTruncated(enrollChainKey, append([]byte{6}, executeTx.Nonce...), primitives.AESKeyLength)

		// Init the state encryptor
		se := queryStateEncryptor{}
		err := se.init(sp.validator.nodeImpl, queryKey, deployTxKey)
		if err != nil {
			return nil, err
		}

		return &se, nil
	}

	// Compute deployTxKey key from the deploy transaction
	deployTxKey := primitives.HMAC(enrollChainKey, deployTx.Nonce)

	// Mask executeTx.Nonce
	executeTxNonce := primitives.HMACTruncated(deployTxKey, primitives.Hash(executeTx.Nonce), primitives.NonceSize)

	// Compute stateKey to encrypt the states and nonceStateKey to generates IVs. This
	// allows validators to reach consesus
	stateKey := primitives.HMACTruncated(deployTxKey, append([]byte{3}, executeTxNonce...), primitives.AESKeyLength)
	nonceStateKey := primitives.HMAC(deployTxKey, append([]byte{4}, executeTxNonce...))

	// Init the state encryptor
	se := stateEncryptorImpl{}
	err := se.init(sp.validator.nodeImpl, stateKey, nonceStateKey, deployTxKey, executeTxNonce)
	if err != nil {
		return nil, err
	}

	return &se, nil
}
