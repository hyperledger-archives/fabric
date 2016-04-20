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
	obc "github.com/hyperledger/fabric/protos"
)

const (
	DefaultConfidentialityProtocolVersion = "1.2"
)

type MessageValidator interface {
	Validate() error
}

type TransactionContext interface {
	GetDeployTransaction() *obc.Transaction

	GetTransaction() *obc.Transaction

	GetChaincodeSpec() *obc.ChaincodeSpec

	GetBinding() []byte
}

type TransactionCertificateProvider interface {
	GetEnrollmentCertificate() (TransactionCertificate, error)

	GetNewTransactionCertificate() (TransactionCertificate, error)
}

type TransactionCertificate interface {
	GetRaw() []byte

	Sign(msg []byte) ([]byte, error)

	Verify(signature, msg []byte) error
}

type ConfidentialityProcessor interface {
	getVersion() string

	process(ctx TransactionContext) (*obc.Transaction, error)
}

type ValidatorConfidentialityProcessor interface {
	getVersion() string

	preValidation(ctx TransactionContext) (*obc.Transaction, error)

	preExecution(ctx TransactionContext) (*obc.Transaction, error)
}

type StateProcessor interface {
	getVersion() string

	getStateEncryptor(deployTx, executeTx *obc.Transaction) (StateEncryptor, error)
}

type QueryResultDecryptor interface {
	getVersion() string

	decryptQueryResult(queryTx *obc.Transaction, ct []byte) ([]byte, error)
}
