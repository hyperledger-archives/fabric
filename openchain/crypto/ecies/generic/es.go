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
	"github.com/openblockchain/obc-peer/openchain/crypto/ecies"
)

type encryptionSchemeImpl struct {
	isForEncryption bool

	// Parameters
	params ecies.AsymmetricCipherParameters
	pub    *publicKeyImpl
	priv   *secretKeyImpl
}

func (es *encryptionSchemeImpl) Init(params ecies.AsymmetricCipherParameters) error {
	if params == nil {
		return ecies.ErrInvalidKeyParameter
	}
	es.isForEncryption = params.IsPublic()
	es.params = params

	if es.isForEncryption {
		switch pk := params.(type) {
		case *publicKeyImpl:
			es.pub = pk
		default:
			return ecies.ErrInvalidKeyParameter
		}
	} else {
		switch sk := params.(type) {
		case *secretKeyImpl:
			es.priv = sk
		default:
			return ecies.ErrInvalidKeyParameter
		}
	}

	return nil
}

func (es *encryptionSchemeImpl) Process(msg []byte) ([]byte, error) {
	if es.isForEncryption {
		// Encrypt
		return eciesEncrypt(es.params.GetRand(), es.pub.pub, nil, nil, msg)
	} else {
		// Decrypt
		return eciesDecrypt(es.priv.priv, nil, nil, msg)
	}

	return nil, nil
}
