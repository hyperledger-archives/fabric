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
	"fmt"
	"github.com/openblockchain/obc-peer/protos"
	"github.com/spf13/viper"
	"os"
	"reflect"
	"testing"
)

func TestMain(m *testing.M) {
	setupTestConfig()
	os.Exit(m.Run())
}

func TestLedgerCommit(t *testing.T) {
	ledger := InitTestLedger(t)
	beginTxBatch(t, 1)
	ledger.SetState("chaincode1", "key1", []byte("value1"))
	ledger.SetState("chaincode2", "key2", []byte("value2"))
	ledger.SetState("chaincode3", "key3", []byte("value3"))
	transaction := buildTestTx()
	commitTxBatch(t, 1, []*protos.Transaction{transaction}, []byte("prrof"))
	if !reflect.DeepEqual(getStateFromLedger(t, "chaincode1", "key1"), []byte("value1")) {
		t.Fatalf("state value not same after Tx commit")
	}
}

func TestLedgerRollback(t *testing.T) {
	ledger := InitTestLedger(t)
	beginTxBatch(t, 1)
	ledger.SetState("chaincode1", "key1", []byte("value1"))
	ledger.SetState("chaincode2", "key2", []byte("value2"))
	ledger.SetState("chaincode3", "key3", []byte("value3"))
	rollbackTxBatch(t, 1)

	valueAfterRollback := getStateFromLedger(t, "chaincode1", "key1")

	if valueAfterRollback != nil {
		t.Logf("Value after rollback = [%s]", valueAfterRollback)
		t.Fatalf("state value not nil after Tx rollback")
	}
}

func TestLedgerDifferentID(t *testing.T) {
	ledger := InitTestLedger(t)
	ledger.BeginTxBatch(1)
	ledger.SetState("chaincode1", "key1", []byte("value1"))
	ledger.SetState("chaincode2", "key2", []byte("value2"))
	ledger.SetState("chaincode3", "key3", []byte("value3"))
	transaction := buildTestTx()
	err := ledger.CommitTxBatch(2, []*protos.Transaction{transaction}, []byte("prrof"))
	if err == nil {
		t.Fatalf("ledger should throw error")
	}
}

func setupTestConfig() {
	viper.AddConfigPath("./../../")
	viper.SetConfigName("openchain")
	err := viper.ReadInConfig()
	if err != nil { // Handle errors reading the config file
		panic(fmt.Errorf("Fatal error config file: %s \n", err))
	}
}
