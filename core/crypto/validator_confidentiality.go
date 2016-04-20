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
	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric/core/crypto/primitives/aes"
	"github.com/hyperledger/fabric/core/crypto/utils"
	obc "github.com/hyperledger/fabric/protos"
)

func (validator *validatorImpl) addConfidentialityProcessor(cp ValidatorConfidentialityProcessor) (err error) {
	validator.confidentialityProcessors[cp.getVersion()] = cp

	return
}

func (validator *validatorImpl) initConfidentialityProcessors() (err error) {
	validator.confidentialityProcessors = make(map[string]ValidatorConfidentialityProcessor)

	// Init confidentiality processors
	validator.addConfidentialityProcessor(&validatorConfidentialityProcessorV1_1{validator})
	validator.addConfidentialityProcessor(&validatorConfidentialityProcessorV1_2{validator})
	validator.addConfidentialityProcessor(&validatorConfidentialityProcessorV2{
		validator, aes.NewAES256GSMSPI(), validator.acSPI,
	})

	return
}

func (validator *validatorImpl) preValidationConfidentiality(tx *obc.Transaction) (*obc.Transaction, error) {
	validator.debug("Confidentiality protocol version [%s]", tx.ConfidentialityProtocolVersion)

	processor, ok := validator.confidentialityProcessors[tx.ConfidentialityProtocolVersion]
	if !ok {
		return nil, utils.ErrInvalidProtocolVersion
	}

	return processor.preValidation(validator.newTransactionContextFromTx(nil, tx))
}

func (validator *validatorImpl) preExecutionConfidentiality(deployTx, tx *obc.Transaction) (*obc.Transaction, error) {
	validator.debug("Confidentiality protocol version [%s]", tx.ConfidentialityProtocolVersion)

	processor, ok := validator.confidentialityProcessors[tx.ConfidentialityProtocolVersion]
	if !ok {
		return nil, utils.ErrInvalidProtocolVersion
	}

	return processor.preExecution(validator.newTransactionContextFromTx(deployTx, tx))
}

func (validator *validatorImpl) cloneTransaction(tx *obc.Transaction) (*obc.Transaction, error) {
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
