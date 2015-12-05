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

package client

import (
	"github.com/openblockchain/obc-peer/openchain/crypto/utils"
	"math/big"
)

func (client *Client) sign(signKey interface{}, msg []byte) ([]byte, error) {
	log.Info("Signing message %s", utils.EncodeBase64(msg))
	return utils.ECDSASign(signKey, msg)
}

func (client *Client) ecdsaSignWithEnrollmentKey(msg []byte) (*big.Int, *big.Int, error) {
	log.Info("Signing message direct %s", utils.EncodeBase64(msg))
	return utils.ECDSASignDirect(client.enrollPrivKey, msg)
}

func (client *Client) verify(verKey interface{}, msg, signature []byte) (bool, error) {
	log.Info("Verifing signature %s against message %s",
		utils.EncodeBase64(signature),
		utils.EncodeBase64(msg),
	)

	return utils.ECDSAVerify(verKey, msg, signature)
}
