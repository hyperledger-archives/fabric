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

package buckettree

import (
	"fmt"
	"os"
	"testing"

	"github.com/openblockchain/obc-peer/openchain/db"
	"github.com/openblockchain/obc-peer/openchain/ledger/statemgmt"
	"github.com/openblockchain/obc-peer/openchain/ledger/testutil"
	"github.com/tecbot/gorocksdb"
)

var testDBWrapper = db.NewTestDBWrapper()

func TestMain(m *testing.M) {
	testutil.SetupTestConfig()
	os.Exit(m.Run())
}

// testHasher is a hash function for testing.
// It returns the hash for a key from pre-populated map
type testHasher struct {
	testHashFunctionInput map[string]uint32
}

func newTestHasher() *testHasher {
	return &testHasher{make(map[string]uint32)}
}

func (testHasher *testHasher) populate(chaincodeID string, key string, hash uint32) {
	testHasher.testHashFunctionInput[string(statemgmt.ConstructCompositeKey(chaincodeID, key))] = hash
}

func (testHasher *testHasher) getHashFunction() hashFunc {
	return func(data []byte) uint32 {
		key := string(data)
		value, ok := testHasher.testHashFunctionInput[key]
		if !ok {
			panic(fmt.Sprintf("A test should add entry before looking up. Entry looked up = [%s]", key))
		}
		return value
	}
}

type stateImplTestWrapper struct {
	stateImpl *StateImpl
	t         *testing.T
}

func newStateImplTestWrapper(t *testing.T) *stateImplTestWrapper {
	stateImpl := NewStateImpl()
	err := stateImpl.Initialize()
	testutil.AssertNoError(t, err, "Error while constrcuting stateImpl")
	return &stateImplTestWrapper{stateImpl, t}
}

func (testWrapper *stateImplTestWrapper) constructNewStateImpl() {
	stateImpl := NewStateImpl()
	err := stateImpl.Initialize()
	testutil.AssertNoError(testWrapper.t, err, "Error while constructing new state tree")
	testWrapper.stateImpl = stateImpl
}

func (testWrapper *stateImplTestWrapper) get(chaincodeID string, key string) []byte {
	value, err := testWrapper.stateImpl.Get(chaincodeID, key)
	testutil.AssertNoError(testWrapper.t, err, "Error while getting value")
	testWrapper.t.Logf("state value for chaincodeID,key=[%s,%s] = [%s], ", chaincodeID, key, string(value))
	return value
}

func (testWrapper *stateImplTestWrapper) prepareWorkingSet(stateDelta *statemgmt.StateDelta) {
	err := testWrapper.stateImpl.PrepareWorkingSet(stateDelta)
	testutil.AssertNoError(testWrapper.t, err, "Error while PrepareWorkingSet")
}

func (testWrapper *stateImplTestWrapper) computeCryptoHash() []byte {
	cryptoHash, err := testWrapper.stateImpl.ComputeCryptoHash()
	testutil.AssertNoError(testWrapper.t, err, "Error while computing crypto hash")
	return cryptoHash
}

func (testWrapper *stateImplTestWrapper) prepareWorkingSetAndComputeCryptoHash(stateDelta *statemgmt.StateDelta) []byte {
	testWrapper.prepareWorkingSet(stateDelta)
	return testWrapper.computeCryptoHash()
}

func (testWrapper *stateImplTestWrapper) addChangesForPersistence(writeBatch *gorocksdb.WriteBatch) {
	err := testWrapper.stateImpl.AddChangesForPersistence(writeBatch)
	testutil.AssertNoError(testWrapper.t, err, "Error while adding changes to db write-batch")
}

func (testWrapper *stateImplTestWrapper) persistChangesAndResetInMemoryChanges() {
	writeBatch := gorocksdb.NewWriteBatch()
	testWrapper.addChangesForPersistence(writeBatch)
	testDBWrapper.WriteToDB(testWrapper.t, writeBatch)
	testWrapper.stateImpl.ClearWorkingSet(true)
}

func createFreshDBAndInitTestStateImplWithCustomHasher(t *testing.T, numBuckets int, maxGroupingAtEachLevel int) (*testHasher, *stateImplTestWrapper, *statemgmt.StateDelta) {
	testDBWrapper.CreateFreshDB(t)
	testHasher := newTestHasher()
	stateImplTestWrapper := newStateImplTestWrapper(t)
	stateDelta := statemgmt.NewStateDelta()
	conf = initConfig(numBuckets, maxGroupingAtEachLevel, testHasher.getHashFunction())
	return testHasher, stateImplTestWrapper, stateDelta
}
