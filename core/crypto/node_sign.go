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
	"math/big"
)

func (node *nodeImpl) sign(signKey interface{}, msg []byte) ([]byte, error) {
	node.debug("Signing message [% x].", msg)

	return primitives.ECDSASign(signKey, msg)
}

func (node *nodeImpl) verify(verKey interface{}, msg, signature []byte) (bool, error) {
	node.debug("Verifing signature [% x] against message [% x].", signature, msg)

	return primitives.ECDSAVerify(verKey, msg, signature)
}

func (node *nodeImpl) signWithEnrollmentKey(msg []byte) ([]byte, error) {
	node.debug("Signing message [% x].", msg)

	return primitives.ECDSASign(node.enrollSigningKey, msg)
}

func (node *nodeImpl) ecdsaSignWithEnrollmentKey(msg []byte) (*big.Int, *big.Int, error) {
	node.debug("Signing message direct [% x].", msg)

	return primitives.ECDSASignDirect(node.enrollSigningKey, msg)
}

func (node *nodeImpl) verifyWithEnrollmentCert(msg, signature []byte) (bool, error) {
	node.debug("Verifing signature [% x] against message [% x].", signature, msg)

	return primitives.ECDSAVerify(node.enrollCert.PublicKey, msg, signature)
}
