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
	"github.com/openblockchain/obc-peer/openchain/db"
	"github.com/openblockchain/obc-peer/openchain/ledger/testutil"
	"github.com/tecbot/gorocksdb"
	"strconv"
	"testing"
)

func BenchmarkDB(b *testing.B) {
	b.StopTimer()
	b.Logf("testParams:%q", testParams)
	flags := flag.NewFlagSet("testParams", flag.ExitOnError)
	kvSize := flags.Int("KVSize", 1000, "size of the key-value")
	toPopulateDB := flags.Bool("PopulateDB", false, "Run in populate DB mode")
	maxKeySuffix := flags.Int("MaxKeySuffix", 1, "the keys are appended with _1, _2,.. upto MaxKeySuffix")
	keyPrefix := flags.String("KeyPrefix", "Key_", "The generated workload will have keys such as KeyPrefix_1, KeyPrefix_2, and so on")
	flags.Parse(testParams)
	if *toPopulateDB {
		b.StartTimer()
		populateDB(b, *kvSize, *maxKeySuffix, *keyPrefix)
		return
	}

	dbWrapper := db.NewTestDBWrapper()
	randNumGen := testutil.NewTestRandomNumberGenerator(*maxKeySuffix)
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		key := []byte(*keyPrefix + strconv.Itoa(randNumGen.Next()))
		value := dbWrapper.GetFromDB(b, key)
		b.SetBytes(int64(len(value)))
	}
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
