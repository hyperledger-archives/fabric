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

package crypto

import (
	"database/sql"
	"fmt"
	"github.com/op/go-logging"
	"github.com/openblockchain/obc-peer/openchain/crypto/utils"
	"os"
	"path/filepath"
	"sync"
)

func (node *nodeImpl) initKeyStore() error {
	// TODO: move all the ket/certificate store/load to the keyStore struct

	ks := keyStore{}
	ks.log = node.log
	ks.conf = node.conf
	if err := ks.init(); err != nil {
		return err
	}

	node.ks = &ks

	return nil
}

type keyStore struct {
	isOpen bool

	// backend
	sqlDB *sql.DB

	// Configuration
	conf *configuration

	// Logging
	log *logging.Logger

	// Sync
	m sync.Mutex
}

func (ks *keyStore) init() error {
	ks.m.Lock()
	defer ks.m.Unlock()

	if ks.isOpen {
		return utils.ErrKeyStoreAlreadyInitialized
	}

	err := ks.createKeyStoreIfKeyStorePathEmpty()
	if err != nil {
		return err
	}

	err = ks.openKeyStore()
	if err != nil {
		return err
	}

	return nil
}

func (ks *keyStore) close() error {
	ks.log.Info("Closing keystore...")
	err := ks.sqlDB.Close()

	if err != nil {
		ks.log.Error("Failed closing keystore: %s", err)
	} else {
		ks.log.Info("Closing keystore...done!")
	}

	ks.isOpen = false
	return err
}

func (ks *keyStore) createKeyStoreIfKeyStorePathEmpty() error {
	// Check directory
	ksPath := ks.conf.getKeyStorePath()
	missing, err := utils.DirMissingOrEmpty(ksPath)
	if err != nil {
		ks.log.Error("Failed checking directory %s: %s", ksPath, err)
	}
	ks.log.Debug("Keystore path [%s] missing [%t]", ksPath, missing)

	if !missing {
		// Check file
		missing, err = utils.FileMissing(ks.conf.getKeyStorePath(), ks.conf.getKeyStoreFilename())
		if err != nil {
			ks.log.Error("Failed checking file %s: %s", ks.conf.getKeyStoreFilePath(), err)
		}
		ks.log.Debug("Keystore file [%s] missing [%t]", ks.conf.getKeyStoreFilePath(), missing)
	}

	if missing {
		err := ks.createKeyStore()
		if err != nil {
			ks.log.Debug("Failed creating db At [%s]: %s", ks.conf.getKeyStoreFilePath(), err.Error())
			return nil
		}
	}

	return nil
}

func (ks *keyStore) createKeyStore() error {
	dbPath := ks.conf.getKeyStorePath()
	ks.log.Debug("Creating Keystore at [%s]", dbPath)

	missing, err := utils.FileMissing(dbPath, ks.conf.getKeyStoreFilename())
	if !missing {
		return fmt.Errorf("Keystore dir [%s] already exists", dbPath)
	}

	os.MkdirAll(dbPath, 0755)

	ks.log.Debug("Open Keystore at [%s]", dbPath)
	db, err := sql.Open("sqlite3", filepath.Join(dbPath, ks.conf.getKeyStoreFilename()))
	if err != nil {
		return err
	}

	ks.log.Debug("Ping Keystore at [%s]", dbPath)
	err = db.Ping()
	if err != nil {
		ks.log.Fatal(err)
	}

	defer db.Close()

	// create tables
	log.Debug("Create Table [%s] at [%s]", "Certificates", dbPath)
	if _, err := db.Exec("CREATE TABLE IF NOT EXISTS Certificates (id VARCHAR, cert BLOB, PRIMARY KEY (id))"); err != nil {
		log.Debug("Failed creating table: %s", err)
		return err
	}

	ks.log.Debug("Keystore created at [%s]", dbPath)
	return nil
}

func (ks *keyStore) deleteKeyStore() error {
	ks.log.Debug("Removing KeyStore at [%s]", ks.conf.getKeyStorePath())

	return os.RemoveAll(ks.conf.getKeyStorePath())
}

func (ks *keyStore) openKeyStore() error {
	if ks.isOpen {
		return nil
	}
	ksPath := ks.conf.getKeyStorePath()

	sqlDB, err := sql.Open("sqlite3", filepath.Join(ksPath, ks.conf.getKeyStoreFilename()))

	if err != nil {
		ks.log.Error("Error opening keystore", err)
		return err
	}
	ks.isOpen = true
	ks.sqlDB = sqlDB

	return nil
}
