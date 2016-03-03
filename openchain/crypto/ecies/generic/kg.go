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

package generic

import (
	"crypto/elliptic"
	"github.com/openblockchain/obc-peer/openchain/crypto/ecies"
	"io"
)

type keyGeneratorParameterImpl struct {
	rand   io.Reader
	curve  elliptic.Curve
	params *Params
}

type keyGeneratorImpl struct {
	isForEncryption bool
	params          *keyGeneratorParameterImpl
}

func (kgp keyGeneratorParameterImpl) GetRand() io.Reader {
	return kgp.rand
}

func (kg *keyGeneratorImpl) Init(params ecies.KeyGeneratorParameters) error {
	if params == nil {
		return ecies.ErrInvalidKeyGeneratorParameter
	}
	switch kgparams := params.(type) {
	case *keyGeneratorParameterImpl:
		kg.params = kgparams
	default:
		return ecies.ErrInvalidKeyGeneratorParameter
	}

	return nil
}

func (kg *keyGeneratorImpl) GenerateKey() (ecies.PrivateKey, error) {

	privKey, err := eciesGenerateKey(
		kg.params.rand,
		kg.params.curve,
		kg.params.params,
	)
	if err != nil {
		return nil, err
	}

	return &secretKeyImpl{privKey, nil, kg.params.params, kg.params.rand}, nil
}
