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

package statemgmt

import (
	"testing"

	"github.com/hyperledger/fabric/core/ledger/testutil"
)

func TestStateDeltaMarshalling(t *testing.T) {
	stateDelta := NewStateDelta()
	stateDelta.Set("chaincode1", "key1", []byte("value1"), nil)
	stateDelta.Set("chaincode2", "key2", []byte("value2"), nil)
	stateDelta.Delete("chaincode3", "key3", nil)

	by := stateDelta.Marshal()
	t.Logf("length of marshalled bytes = [%d]", len(by))
	stateDelta1 := NewStateDelta()
	stateDelta1.Unmarshal(by)

	testutil.AssertEquals(t, stateDelta1, stateDelta)
}

func TestStateDeltaCryptoHash(t *testing.T) {
	stateDelta := NewStateDelta()

	testutil.AssertNil(t, stateDelta.ComputeCryptoHash())

	stateDelta.Set("chaincodeID1", "key2", []byte("value2"), nil)
	stateDelta.Set("chaincodeID1", "key1", []byte("value1"), nil)
	stateDelta.Set("chaincodeID2", "key2", []byte("value2"), nil)
	stateDelta.Set("chaincodeID2", "key1", []byte("value1"), nil)
	testutil.AssertEquals(t, stateDelta.ComputeCryptoHash(), testutil.ComputeCryptoHash([]byte("chaincodeID1key1value1key2value2chaincodeID2key1value1key2value2")))

	stateDelta.Delete("chaincodeID2", "key1", nil)
	testutil.AssertEquals(t, stateDelta.ComputeCryptoHash(), testutil.ComputeCryptoHash([]byte("chaincodeID1key1value1key2value2chaincodeID2key1key2value2")))
}
