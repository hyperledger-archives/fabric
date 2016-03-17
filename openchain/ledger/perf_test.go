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

package ledger

import (
	"flag"
	"github.com/op/go-logging"
	"github.com/openblockchain/obc-peer/openchain/db"
	"github.com/openblockchain/obc-peer/openchain/ledger/testutil"
	"github.com/openblockchain/obc-peer/openchain/util"
	"github.com/openblockchain/obc-peer/protos"
	"github.com/tecbot/gorocksdb"
	"strconv"
	"testing"
)

func BenchmarkDB(b *testing.B) {
	b.Logf("testParams:%q", testParams)
	flags := flag.NewFlagSet("testParams", flag.ExitOnError)
	kvSize := flags.Int("KVSize", 1000, "size of the key-value")
	toPopulateDB := flags.Bool("PopulateDB", false, "Run in populate DB mode")
	maxKeySuffix := flags.Int("MaxKeySuffix", 1, "the keys are appended with _1, _2,.. upto MaxKeySuffix")
	keyPrefix := flags.String("KeyPrefix", "Key_", "The generated workload will have keys such as KeyPrefix_1, KeyPrefix_2, and so on")
	flags.Parse(testParams)
	if *toPopulateDB {
		b.ResetTimer()
		populateDB(b, *kvSize, *maxKeySuffix, *keyPrefix)
		return
	}

	dbWrapper := db.NewTestDBWrapper()
	randNumGen := testutil.NewTestRandomNumberGenerator(*maxKeySuffix)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := []byte(*keyPrefix + strconv.Itoa(randNumGen.Next()))
		value := dbWrapper.GetFromDB(b, key)
		b.SetBytes(int64(len(value)))
	}
}

func BenchmarkLedgerSingleKeyTransaction(b *testing.B) {
	b.Logf("testParams:%q", testParams)
	flags := flag.NewFlagSet("testParams", flag.ExitOnError)
	key := flags.String("Key", "key", "key name")
	kvSize := flags.Int("KVSize", 1000, "size of the key-value")
	batchSize := flags.Int("BatchSize", 100, "size of the key-value")
	numBatches := flags.Int("NumBatches", 100, "number of batches")
	numWritesToLedger := flags.Int("NumWritesToLedger", 4, "size of the key-value")
	flags.Parse(testParams)

	b.Logf(`Running test with params: key=%s, kvSize=%d, batchSize=%d, numBatches=%d, NumWritesToLedger=%d`,
		*key, *kvSize, *batchSize, *numBatches, *numWritesToLedger)

	testutil.SetLogLevel(logging.ERROR, "indexes")
	testutil.SetLogLevel(logging.ERROR, "ledger")
	testutil.SetLogLevel(logging.ERROR, "state")
	testutil.SetLogLevel(logging.ERROR, "statemgmt")
	testutil.SetLogLevel(logging.ERROR, "buckettree")
	testutil.SetLogLevel(logging.ERROR, "db")

	ledgerTestWrapper := createFreshDBAndTestLedgerWrapper(b)
	ledger := ledgerTestWrapper.ledger

	chaincode := "chaincodeId"
	value := testutil.ConstructRandomBytes(b, *kvSize-(len(chaincode)+len(*key)))
	tx := constructDummyTx(b)
	serializedBytes, _ := tx.Bytes()
	b.Logf("Size of serialized bytes for tx = %d", len(serializedBytes))
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		for i := 0; i < *numBatches; i++ {
			ledger.BeginTxBatch(1)
			// execute one batch
			var transactions []*protos.Transaction
			for j := 0; j < *batchSize; j++ {
				ledger.TxBegin("txUuid")
				_, err := ledger.GetState(chaincode, *key, true)
				if err != nil {
					b.Fatalf("Error in getting state: %s", err)
				}
				for l := 0; l < *numWritesToLedger; l++ {
					ledger.SetState(chaincode, *key, value)
				}
				ledger.TxFinished("txUuid", true)
				transactions = append(transactions, tx)
			}
			ledger.CommitTxBatch(1, transactions, nil, []byte("proof"))
		}
	}
	b.StopTimer()

	//varify value persisted
	value, _ = ledger.GetState(chaincode, *key, true)
	size := ledger.GetBlockchainSize()
	b.Logf("Value size=%d, Blockchain height=%d", len(value), size)
}

func populateDB(tb testing.TB, kvSize int, totalKeys int, keyPrefix string) {
	dbWrapper := db.NewTestDBWrapper()
	dbWrapper.CreateFreshDB(tb)
	batch := gorocksdb.NewWriteBatch()
	for i := 0; i < totalKeys; i++ {
		key := []byte(keyPrefix + strconv.Itoa(i))
		value := testutil.ConstructRandomBytes(tb, kvSize-len(key))
		batch.Put(key, value)
		if i%1000 == 0 {
			dbWrapper.WriteToDB(tb, batch)
			batch = gorocksdb.NewWriteBatch()
		}
	}
	dbWrapper.CloseDB(tb)
}

func constructDummyTx(tb testing.TB) *protos.Transaction {
	uuid := util.GenerateUUID()
	tx, err := protos.NewTransaction(protos.ChaincodeID{Path: "dummyChaincodeId"}, uuid, "dummyFunction", []string{"dummyParamValue1, dummyParamValue2"})
	testutil.AssertNil(tb, err)
	return tx
}
