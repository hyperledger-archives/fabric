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
	"fmt"

	"golang.org/x/crypto/sha3"

	"github.com/tecbot/gorocksdb"
)

// StateDB defines the database struct
type StateDB struct {
	db *gorocksdb.DB
}

// OpenStateDB opens an existing or creates a new state DB
func OpenStateDB(dbPath string, createIfMissing bool) (*StateDB, error) {
	opts := gorocksdb.NewDefaultOptions()
	opts.SetCreateIfMissing(createIfMissing)
	db, err := gorocksdb.OpenDb(opts, dbPath)
	if err != nil {
		fmt.Println("Error opening DB", err)
		return nil, err
	}
	return &StateDB{db}, nil
}

// Put a key-value pair into a contract state
func (stateDB *StateDB) Put(contractID string, key, value []byte) error {
	writeOpts := gorocksdb.NewDefaultWriteOptions()
	dbKey := buildKey(contractID, key)
	err := stateDB.db.Put(writeOpts, dbKey, value)
	if err != nil {
		return err
	}
	return nil
}

// Get a value for a given contract ID and key
func (stateDB *StateDB) Get(contractID string, key []byte) ([]byte, error) {
	readOpts := gorocksdb.NewDefaultReadOptions()
	dbKey := buildKey(contractID, key)
	value, err := stateDB.db.GetBytes(readOpts, dbKey)
	if err != nil {
		return nil, err
	}
	return value, nil
}

// Delete a key-value pair for a given contract ID and key
func (stateDB *StateDB) Delete(contractID string, key []byte) error {
	writeOpts := gorocksdb.NewDefaultWriteOptions()
	dbKey := buildKey(contractID, key)
	err := stateDB.db.Delete(writeOpts, dbKey)
	if err != nil {
		return err
	}
	return nil
}

// GetHash returns a has of the entire state
func (stateDB *StateDB) GetHash() []byte {
	readOpts := gorocksdb.NewDefaultReadOptions()
	iter := stateDB.db.NewIterator(readOpts)
	defer iter.Close()

	iter.SeekToFirst()
	var buffer bytes.Buffer
	for ; iter.Valid(); iter.Next() {
		buffer.Write(iter.Key().Data())
		buffer.Write(iter.Value().Data())
	}

	hash := make([]byte, 64)
	sha3.ShakeSum256(hash, buffer.Bytes())

	return hash
}

var dbKeyDelimiter = []byte{0x00}

func buildKey(contractID string, key []byte) []byte {
	retKey := []byte(contractID)
	retKey = append(retKey, dbKeyDelimiter...)
	retKey = append(retKey, key...)
	return retKey
}
