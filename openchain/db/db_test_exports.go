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

package db

import (
	"github.com/spf13/viper"
	"github.com/tecbot/gorocksdb"
	"os"
	"testing"
)

type TestDBWrapper struct {
	performCleanup bool
}

func NewTestDBWrapper() *TestDBWrapper {
	return &TestDBWrapper{}
}

///////////////////////////
// Test db creation and cleanup functions

func (testDB *TestDBWrapper) CreateFreshDB(t *testing.T) {
	// cleaning up test db here so that each test does not have to call it explicitly
	// at the end of the test
	testDB.Cleanup()
	testDB.removeDBPath()
	t.Logf("Creating testDB")
	err := CreateDB()
	if err != nil {
		t.Fatalf("Error in creating test db. Error = [%s]", err)
	}
	testDB.performCleanup = true
}

func (testDB *TestDBWrapper) Cleanup() {
	if testDB.performCleanup {
		GetDBHandle().CloseDB()
		testDB.performCleanup = false
	}
}

func (testDB *TestDBWrapper) removeDBPath() {
	dbPath := viper.GetString("peer.db.path")
	os.RemoveAll(dbPath)
}

func (testDB *TestDBWrapper) WriteToDB(t *testing.T, writeBatch *gorocksdb.WriteBatch) {
	opt := gorocksdb.NewDefaultWriteOptions()
	err := GetDBHandle().DB.Write(opt, writeBatch)
	if err != nil {
		t.Fatalf("Error while writing to db. Error:%s", err)
	}
}

func (testDB *TestDBWrapper) GetFromStateCF(t *testing.T, key []byte) []byte {
	openchainDB := GetDBHandle()
	value, err := openchainDB.GetFromStateCF(key)
	if err != nil {
		t.Fatalf("Error while getting from db. Error:%s", err)
	}
	return value
}

func (testDB *TestDBWrapper) GetFromStateDeltaCF(t *testing.T, key []byte) []byte {
	openchainDB := GetDBHandle()
	value, err := openchainDB.GetFromStateHashCF(key)
	if err != nil {
		t.Fatalf("Error while getting from db. Error:%s", err)
	}
	return value
}
