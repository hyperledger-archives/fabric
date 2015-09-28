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

package openchain

import (
	"bytes"
	"os"
	"testing"

	"github.com/tecbot/gorocksdb"
)

func TestPut(t *testing.T) {

	// Open the DB
	dbPath := os.TempDir() + "/OpenchainStateDBTestPut"
	opts := gorocksdb.NewDefaultOptions()
	destoryErr := gorocksdb.DestroyDb(dbPath, opts)
	if destoryErr != nil {
		t.Error("Error destroying DB", destoryErr)
	}

	stateDB, err := OpenStateDB(dbPath, true)
	if err != nil {
		t.Error("Error opening DB", err)
	}

	contract := "stateCodes"
	key := []byte("NC")
	value := []byte("North Carolina")
	putErr := stateDB.Put(contract, key, value)
	if putErr != nil {
		t.Error("Error putting value", err)
	}

	retVal, err := stateDB.Get(contract, key)
	if err != nil {
		t.Error("Error getting value", err)
	}

	if bytes.Compare(retVal, value) != 0 {
		t.Error("Expected values to match, but got", retVal)
	}
}

func TestDelete(t *testing.T) {

	// Open the DB
	dbPath := os.TempDir() + "/OpenchainStateDBTestDelete"
	opts := gorocksdb.NewDefaultOptions()
	destoryErr := gorocksdb.DestroyDb(dbPath, opts)
	if destoryErr != nil {
		t.Error("Error destroying DB", destoryErr)
	}

	stateDB, err := OpenStateDB(dbPath, true)
	if err != nil {
		t.Error("Error opening DB", err)
	}

	contract := "stateCodes"
	key := []byte("NC")
	value := []byte("North Carolina")
	putErr := stateDB.Put(contract, key, value)
	if putErr != nil {
		t.Error("Error putting value", err)
	}

	delErr := stateDB.Delete(contract, key)
	if delErr != nil {
		t.Error("Error deleting value", err)
	}

	retVal, err := stateDB.Get(contract, key)
	if err != nil {
		t.Error("Error getting value", err)
	}

	if retVal != nil {
		t.Error("Expected nil, but got", retVal)
	}
}
