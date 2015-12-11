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
	"github.com/openblockchain/obc-peer/openchain/crypto/utils"
)

func (client *clientImpl) initKeyStore() error {
	// create tables
	client.node.ks.log.Debug("Create Table [%s] at [%s]", "TCert", client.node.ks.conf.getKeyStorePath())
	if _, err := client.node.ks.sqlDB.Exec("CREATE TABLE IF NOT EXISTS TCerts (id INTEGER, cert BLOB, key BLOB, PRIMARY KEY (id))"); err != nil {
		client.node.ks.log.Debug("Failed creating table: %s", err)
		return err
	}

	client.node.ks.log.Debug("keystore created at [%s]", client.node.ks.conf.getKeyStorePath())

	return nil
}

func (ks *keyStore) GetNextTCert(tCertFetcher func(num int) ([][]byte, [][]byte, error)) ([]byte, []byte, error) {
	ks.m.Lock()
	defer ks.m.Unlock()

	cert, key, err := ks.selectNextTCert()
	if err != nil {
		ks.log.Error("Failed selecting next TCert: %s", err)

		return nil, nil, err
	}
	ks.log.Info("cert %s", utils.EncodeBase64(cert))

	if cert == nil {
		// If No TCert is available, fetch new ones, store them and return the first available.

		// 1. Fetch
		ks.log.Info("Fectch TCerts from TCA...")
		certs, keys, err := tCertFetcher(10)
		if err != nil {
			return nil, nil, err
		}

		// 2. Store
		ks.log.Info("Store them...")
		tx, err := ks.sqlDB.Begin()
		if err != nil {
			ks.log.Error("Failed beginning transaction: %s", err)

			return nil, nil, err
		}

		for i, cert := range certs {
			ks.log.Info("Insert index %d", i)

			//			db.log.Info("Insert key %s", utils.EncodeBase64(keys[i]))
			ks.log.Info("Insert cert %s", utils.EncodeBase64(cert))

			// TODO: once the TCert structure is finalized,
			// store only the cert from which the corresponding key
			// can be derived

			_, err := tx.Exec("INSERT INTO TCerts (cert, key) VALUES (?, ?)", cert, keys[i])

			if err != nil {
				ks.log.Error("Failed inserting cert %s", err)
				continue
			}
		}

		err = tx.Commit()
		if err != nil {
			ks.log.Error("Failed committing transaction: %s", err)

			tx.Rollback()

			return nil, nil, err
		}

		ks.log.Info("Fectch TCerts from TCA...done!")

		cert, key, err = ks.selectNextTCert()
		if err != nil {
			ks.log.Error("Failed selecting next TCert after fetching: %s", err)

			return nil, nil, err
		}
	}

	return cert, key, nil
	//	return nil, nil, errors.New("No cert obtained")
	//	return utils.NewSelfSignedCert()
}

func (ks *keyStore) selectNextTCert() ([]byte, []byte, error) {
	ks.log.Info("Select next TCert...")

	// Open transaction
	tx, err := ks.sqlDB.Begin()
	if err != nil {
		ks.log.Error("Failed beginning transaction: %s", err)

		return nil, nil, err
	}

	// Get the first row available
	var id int
	var cert, key []byte
	row := ks.sqlDB.QueryRow("SELECT id, cert, key FROM TCerts")
	err = row.Scan(&id, &cert, &key)

	if err == sql.ErrNoRows {
		return nil, nil, nil
	} else if err != nil {
		ks.log.Error("Error during select: %s", err)

		return nil, nil, err
	}

	ks.log.Info("id %d", id)
	ks.log.Info("cert %s", utils.EncodeBase64(cert))
	//	db.log.Info("key %s", utils.EncodeBase64(key))

	// TODO: instead of removing, move the TCert to a new table
	// which stores the TCerts used

	// Remove that row
	ks.log.Info("Removing row with id [%d]...", id)

	if _, err := tx.Exec("DELETE FROM TCerts WHERE id = ?", id); err != nil {
		ks.log.Error("Failed removing row [%d]: %s", id, err)

		tx.Rollback()

		return nil, nil, err
	}

	ks.log.Info("Removing row with id [%d]...done", id)

	// Finalize
	err = tx.Commit()
	if err != nil {
		ks.log.Error("Failed commiting: %s", err)
		tx.Rollback()

		return nil, nil, err
	}

	ks.log.Info("Select next TCert...done!")

	return cert, key, nil
}
