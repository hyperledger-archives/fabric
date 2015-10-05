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
	"fmt"
	"os"
	"path"

	"github.com/spf13/viper"
	"github.com/tecbot/gorocksdb"
)

const blockchainCF = "blockchainCF"
const stateCF = "stateCF"
const statehashCF = "statehashCF"

var columnfamilies = []string{blockchainCF, stateCF, statehashCF}

// OpenchainDB encapsulates rocksdb's structures
type OpenchainDB struct {
	DB           *gorocksdb.DB
	BlockchainCF *gorocksdb.ColumnFamilyHandle
	StateCF      *gorocksdb.ColumnFamilyHandle
	StateHashCF  *gorocksdb.ColumnFamilyHandle
}

var openchainDB *OpenchainDB
var isOpen bool

// CreateDB creates a rocks db database
func CreateDB() error {
	dbPath := getDBPath()
	dbPathExists, err := dirExists(dbPath)
	if err != nil {
		return err
	}

	if dbPathExists {
		return fmt.Errorf("db dir [%s] already exists.", dbPath)
	}
	os.MkdirAll(path.Dir(dbPath), 0755)
	opts := gorocksdb.NewDefaultOptions()
	opts.SetCreateIfMissing(true)

	db, err := gorocksdb.OpenDb(opts, dbPath)
	if err != nil {
		return err
	}

	defer db.Close()

	for _, cf := range columnfamilies {
		_, err = db.CreateColumnFamily(opts, cf)
		if err != nil {
			return err
		}
	}
	return nil
}

// GetDBHandle returns a handle to OpenchainDB
func GetDBHandle() *OpenchainDB {
	var err error
	if isOpen {
		return openchainDB
	}
	openchainDB, err = openDB()
	if err != nil {
		panic(fmt.Sprintf("Could not open openchain db error = [%s]", err))
	}
	return openchainDB
}

// GetFromBlockchainCF get value for given key from column family - blockchainCF
func (openchainDB *OpenchainDB) GetFromBlockchainCF(key []byte) ([]byte, error) {
	return openchainDB.get(openchainDB.BlockchainCF, key)
}

// GetFromStateCF get value for given key from column family - stateCF
func (openchainDB *OpenchainDB) GetFromStateCF(key []byte) ([]byte, error) {
	return openchainDB.get(openchainDB.StateCF, key)
}

// GetFromStateHashCF get value for given key from column family - statehashCF
func (openchainDB *OpenchainDB) GetFromStateHashCF(key []byte) ([]byte, error) {
	return openchainDB.get(openchainDB.StateHashCF, key)
}

// GetBlockchainCFIterator get iterator for column family - blockchainCF
func (openchainDB *OpenchainDB) GetBlockchainCFIterator() *gorocksdb.Iterator {
	return openchainDB.getIterator(openchainDB.BlockchainCF)
}

// GetStateCFIterator get iterator for column family - stateCF
func (openchainDB *OpenchainDB) GetStateCFIterator() *gorocksdb.Iterator {
	return openchainDB.getIterator(openchainDB.StateCF)
}

// GetStateHashCFIterator get iterator for column family - statehashCF
func (openchainDB *OpenchainDB) GetStateHashCFIterator() *gorocksdb.Iterator {
	return openchainDB.getIterator(openchainDB.StateHashCF)
}

func getDBPath() string {
	dbPath := viper.GetString("peer.db.path")
	if dbPath == "" {
		panic("DB path not specified in configuration file. Please check that property 'peer.db.path' is set")
	}
	return dbPath
}

func openDB() (*OpenchainDB, error) {
	if isOpen {
		return openchainDB, nil
	}
	dbPath := getDBPath()
	opts := gorocksdb.NewDefaultOptions()
	opts.SetCreateIfMissing(false)
	db, cfHandlers, err := gorocksdb.OpenDbColumnFamilies(opts, dbPath,
		[]string{"default", blockchainCF, stateCF, statehashCF},
		[]*gorocksdb.Options{opts, opts, opts, opts})

	if err != nil {
		fmt.Println("Error opening DB", err)
		return nil, err
	}
	isOpen = true
	return &OpenchainDB{db, cfHandlers[1], cfHandlers[2], cfHandlers[3]}, nil
}

// CloseDB releases all column family handles and closes rocksdb
func (openchainDB *OpenchainDB) CloseDB() {
	openchainDB.BlockchainCF.Destroy()
	openchainDB.StateCF.Destroy()
	openchainDB.StateHashCF.Destroy()
	openchainDB.DB.Close()
	isOpen = false
}

func (openchainDB *OpenchainDB) get(cfHandler *gorocksdb.ColumnFamilyHandle, key []byte) ([]byte, error) {
	opt := gorocksdb.NewDefaultReadOptions()
	slice, err := openchainDB.DB.GetCF(opt, cfHandler, key)
	if err != nil {
		fmt.Println("Error while trying to retrieve key:", key)
		return nil, err
	}
	return slice.Data(), nil
}

func (openchainDB *OpenchainDB) getIterator(cfHandler *gorocksdb.ColumnFamilyHandle) *gorocksdb.Iterator {
	opt := gorocksdb.NewDefaultReadOptions()
	return openchainDB.DB.NewIteratorCF(opt, cfHandler)
}

func dirExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}
