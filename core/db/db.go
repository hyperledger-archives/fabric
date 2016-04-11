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
	"strings"

	"github.com/op/go-logging"
	"github.com/spf13/viper"
	"github.com/tecbot/gorocksdb"
)

var dbLogger = logging.MustGetLogger("db")

const blockchainCF = "blockchainCF"
const stateCF = "stateCF"
const stateDeltaCF = "stateDeltaCF"
const indexesCF = "indexesCF"

var columnfamilies = []string{blockchainCF, stateCF, stateDeltaCF, indexesCF}

// OpenchainDB encapsulates rocksdb's structures
type OpenchainDB struct {
	DB           *gorocksdb.DB
	BlockchainCF *gorocksdb.ColumnFamilyHandle
	StateCF      *gorocksdb.ColumnFamilyHandle
	StateDeltaCF *gorocksdb.ColumnFamilyHandle
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
	err = os.MkdirAll(path.Dir(dbPath), 0755)
	if err != nil {
		dbLogger.Error("Error calling  os.MkdirAll for directory path [%s]: %s", dbPath, err)
		return fmt.Errorf("Error making directory path [%s]: %s", dbPath, err)
	}
	opts := gorocksdb.NewDefaultOptions()
	defer opts.Destroy()
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

// GetFromBlockchainCFSnapshot get value for given key from column family in a DB snapshot - blockchainCF
func (openchainDB *OpenchainDB) GetFromBlockchainCFSnapshot(snapshot *gorocksdb.Snapshot, key []byte) ([]byte, error) {
	return openchainDB.getFromSnapshot(snapshot, openchainDB.BlockchainCF, key)
}

// GetFromStateCF get value for given key from column family - stateCF
func (openchainDB *OpenchainDB) GetFromStateCF(key []byte) ([]byte, error) {
	return openchainDB.get(openchainDB.StateCF, key)
}

// GetFromStateDeltaCF get value for given key from column family - stateDeltaCF
func (openchainDB *OpenchainDB) GetFromStateDeltaCF(key []byte) ([]byte, error) {
	return openchainDB.get(openchainDB.StateDeltaCF, key)
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

// GetStateCFSnapshotIterator get iterator for column family - stateCF. This iterator
// is based on a snapshot and should be used for long running scans, such as
// reading the entire state. Remember to call iterator.Close() when you are done.
func (openchainDB *OpenchainDB) GetStateCFSnapshotIterator(snapshot *gorocksdb.Snapshot) *gorocksdb.Iterator {
	return openchainDB.getSnapshotIterator(snapshot, openchainDB.StateCF)
}

// GetStateDeltaCFIterator get iterator for column family - stateDeltaCF
func (openchainDB *OpenchainDB) GetStateDeltaCFIterator() *gorocksdb.Iterator {
	return openchainDB.getIterator(openchainDB.StateDeltaCF)
}

// GetSnapshot returns a point-in-time view of the DB. You MUST call snapshot.Release()
// when you are done with the snapshot.
func (openchainDB *OpenchainDB) GetSnapshot() *gorocksdb.Snapshot {
	return openchainDB.DB.NewSnapshot()
}

func getDBPath() string {
	dbPath := viper.GetString("peer.fileSystemPath")
	if dbPath == "" {
		panic("DB path not specified in configuration file. Please check that property 'peer.fileSystemPath' is set")
	}
	if !strings.HasSuffix(dbPath, "/") {
		dbPath = dbPath + "/"
	}
	return dbPath + "db"
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
	defer opts.Destroy()

	opts.SetCreateIfMissing(false)
	db, cfHandlers, err := gorocksdb.OpenDbColumnFamilies(opts, dbPath,
		[]string{"default", blockchainCF, stateCF, stateDeltaCF, indexesCF},
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
	openchainDB.StateDeltaCF.Destroy()
	openchainDB.DB.Close()
	isOpen = false
}

// DeleteState delets ALL state keys/values from the DB. This is generally
// only used during state synchronization when creating a new state from
// a snapshot.
func (openchainDB *OpenchainDB) DeleteState() error {
	err := openchainDB.DB.DropColumnFamily(openchainDB.StateCF)
	if err != nil {
		dbLogger.Error("Error dropping state CF", err)
		return err
	}
	err = openchainDB.DB.DropColumnFamily(openchainDB.StateDeltaCF)
	if err != nil {
		dbLogger.Error("Error dropping state delta CF", err)
		return err
	}
	opts := gorocksdb.NewDefaultOptions()
	defer opts.Destroy()
	openchainDB.StateCF, err = openchainDB.DB.CreateColumnFamily(opts, stateCF)
	if err != nil {
		dbLogger.Error("Error creating state CF", err)
		return err
	}
	openchainDB.StateDeltaCF, err = openchainDB.DB.CreateColumnFamily(opts, stateDeltaCF)
	if err != nil {
		dbLogger.Error("Error creating state delta CF", err)
		return err
	}
	return nil
}

func (openchainDB *OpenchainDB) get(cfHandler *gorocksdb.ColumnFamilyHandle, key []byte) ([]byte, error) {
	opt := gorocksdb.NewDefaultReadOptions()
	defer opt.Destroy()
	slice, err := openchainDB.DB.GetCF(opt, cfHandler, key)
	if err != nil {
		fmt.Println("Error while trying to retrieve key:", key)
		return nil, err
	}
	defer slice.Free()
	data := append([]byte(nil), slice.Data()...)
	return data, nil
}

func (openchainDB *OpenchainDB) getFromSnapshot(snapshot *gorocksdb.Snapshot, cfHandler *gorocksdb.ColumnFamilyHandle, key []byte) ([]byte, error) {
	opt := gorocksdb.NewDefaultReadOptions()
	defer opt.Destroy()
	opt.SetSnapshot(snapshot)
	slice, err := openchainDB.DB.GetCF(opt, cfHandler, key)
	if err != nil {
		fmt.Println("Error while trying to retrieve key:", key)
		return nil, err
	}
	defer slice.Free()
	data := append([]byte(nil), slice.Data()...)
	return data, nil
}

func (openchainDB *OpenchainDB) getIterator(cfHandler *gorocksdb.ColumnFamilyHandle) *gorocksdb.Iterator {
	opt := gorocksdb.NewDefaultReadOptions()
	opt.SetFillCache(true)
	defer opt.Destroy()
	return openchainDB.DB.NewIteratorCF(opt, cfHandler)
}

func (openchainDB *OpenchainDB) getSnapshotIterator(snapshot *gorocksdb.Snapshot, cfHandler *gorocksdb.ColumnFamilyHandle) *gorocksdb.Iterator {
	opt := gorocksdb.NewDefaultReadOptions()
	defer opt.Destroy()
	opt.SetSnapshot(snapshot)
	iter := openchainDB.DB.NewIteratorCF(opt, cfHandler)
	return iter
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
