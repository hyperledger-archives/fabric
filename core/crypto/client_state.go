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
	"github.com/hyperledger/fabric/core/crypto/utils"
	obc "github.com/hyperledger/fabric/protos"
)

func (client *clientImpl) addQueryResultDecryptor(qrd QueryResultDecryptor) (err error) {
	client.queryResultDecryptors[qrd.getVersion()] = qrd

	return
}

func (client *clientImpl) initQueryResultDecryptors() (err error) {
	client.queryResultDecryptors = make(map[string]QueryResultDecryptor)

	// Init confidentiality processors
	client.addQueryResultDecryptor(&clientQueryResultDecryptorV1_1{client})
	client.addQueryResultDecryptor(&clientQueryResultDecryptorV1_2{client})
	client.addQueryResultDecryptor(&clientQueryResultDecryptorV2{client})

	return
}

// DecryptQueryResult is used to decrypt the result of a query transaction
func (client *clientImpl) DecryptQueryResult(queryTx *obc.Transaction, ct []byte) ([]byte, error) {
	client.debug("Confidentiality protocol version [%s]", queryTx.ConfidentialityProtocolVersion)

	decryptor, ok := client.queryResultDecryptors[queryTx.ConfidentialityProtocolVersion]
	if !ok {
		return nil, utils.ErrInvalidProtocolVersion
	}

	return decryptor.decryptQueryResult(queryTx, ct)
}
