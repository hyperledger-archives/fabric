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
	"io"
	"os"
	"path"

	"github.com/op/go-logging"
	"github.com/spf13/viper"
	"github.com/tecbot/gorocksdb"
)

var dbLogger = logging.MustGetLogger("db")

const blockchainCF = "blockchainCF"
const stateCF = "stateCF"
const statehashCF = "statehashCF"
const indexesCF = "indexesCF"

var columnfamilies = []string{blockchainCF, stateCF, statehashCF, indexesCF}

// OpenchainDB encapsulates rocksdb's structures
type OpenchainDB struct {
	DB           *gorocksdb.DB
	BlockchainCF *gorocksdb.ColumnFamilyHandle
	StateCF      *gorocksdb.ColumnFamilyHandle
	StateHashCF  *gorocksdb.ColumnFamilyHandle
	IndexesCF    *gorocksdb.ColumnFamilyHandle
}

var openchainDB *OpenchainDB
var isOpen bool

// CreateDB creates a rocks db database
func CreateDB() error {
	dbPath := getDBPath()
	dbLogger.Debug("Creating DB at [%s]", dbPath)
	missing, err := dirMissingOrEmpty(dbPath)
	if err != nil {
		return err
	}

	if !missing {
		return fmt.Errorf("db dir [%s] already exists", dbPath)
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
	dbLogger.Debug("DB created at [%s]", dbPath)
	return nil
}

// GetDBHandle returns a handle to OpenchainDB
func GetDBHandle() *OpenchainDB {
	var err error
	if isOpen {
		return openchainDB
	}

	err = createDBIfDBPathEmpty()
	if err != nil {
		panic(fmt.Sprintf("Error while trying to create DB: %s", err))
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

// GetFromIndexesCF get value for given key from column family - indexCF
func (openchainDB *OpenchainDB) GetFromIndexesCF(key []byte) ([]byte, error) {
	return openchainDB.get(openchainDB.IndexesCF, key)
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

func createDBIfDBPathEmpty() error {
	dbPath := getDBPath()
	missing, err := dirMissingOrEmpty(dbPath)
	if err != nil {
		return err
	}
	dbLogger.Debug("Is db path [%s] empty [%t]", dbPath, missing)
	if missing {
		err := CreateDB()
		if err != nil {
			return nil
		}
	}
	return nil
}

func openDB() (*OpenchainDB, error) {
	if isOpen {
		return openchainDB, nil
	}
	dbPath := getDBPath()
	opts := gorocksdb.NewDefaultOptions()
	opts.SetCreateIfMissing(false)
	db, cfHandlers, err := gorocksdb.OpenDbColumnFamilies(opts, dbPath,
		[]string{"default", blockchainCF, stateCF, statehashCF, indexesCF},
		[]*gorocksdb.Options{opts, opts, opts, opts, opts})

	if err != nil {
		fmt.Println("Error opening DB", err)
		return nil, err
	}
	isOpen = true
	return &OpenchainDB{db, cfHandlers[1], cfHandlers[2], cfHandlers[3], cfHandlers[4]}, nil
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

func dirMissingOrEmpty(path string) (bool, error) {
	dirExists, err := dirExists(path)
	if err != nil {
		return false, err
	}
	if !dirExists {
		return true, nil
	}

	dirEmpty, err := dirEmpty(path)
	if err != nil {
		return false, err
	}
	if dirEmpty {
		return true, nil
	}
	return false, nil
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

func dirEmpty(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()

	_, err = f.Readdir(1)
	if err == io.EOF {
		return true, nil
	}
	return false, err
}
