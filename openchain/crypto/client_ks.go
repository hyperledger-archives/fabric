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
	client.node.ks.log.Debug("Create Table if not exists [TCert] at [%s].", client.node.ks.conf.getKeyStorePath())
	if _, err := client.node.ks.sqlDB.Exec("CREATE TABLE IF NOT EXISTS TCerts (id INTEGER, cert BLOB, PRIMARY KEY (id))"); err != nil {
		client.node.ks.log.Debug("Failed creating table [%s].", err.Error())
		return err
	}

	client.node.ks.log.Debug("Create Table if not exists [UsedTCert] at [%s].", client.node.ks.conf.getKeyStorePath())
	if _, err := client.node.ks.sqlDB.Exec("CREATE TABLE IF NOT EXISTS UsedTCert (id INTEGER, cert BLOB, PRIMARY KEY (id))"); err != nil {
		client.node.ks.log.Debug("Failed creating table [%s].", err.Error())
		return err
	}

	return nil
}

func (ks *keyStore) GetNextTCert(tCertFetcher func(num int) ([][]byte, error)) ([]byte, error) {
	ks.m.Lock()
	defer ks.m.Unlock()

	cert, err := ks.selectNextTCert()
	if err != nil {
		ks.log.Error("Failed selecting next TCert [%s].", err.Error())

		return nil, err
	}

	if cert == nil {
		// If No TCert is available, fetch new ones, store them and return the first available.

		// 1. Fetch
		ks.log.Debug("Fectch TCerts from TCA...")
		certs, err := tCertFetcher(10)
		if err != nil {
			return nil, err
		}

		// 2. Store
		ks.log.Debug("Store them...")
		tx, err := ks.sqlDB.Begin()
		if err != nil {
			ks.log.Error("Failed beginning transaction [%s].", err.Error())

			return nil, err
		}

		for i, cert := range certs {
			ks.log.Debug("Insert index [%d]", i)

			//			db.log.Info("Insert key  ", utils.EncodeBase64(keys[i]))
			ks.log.Debug("Insert cert [%s].", utils.EncodeBase64(cert))

			_, err := tx.Exec("INSERT INTO TCerts (cert) VALUES (?)", cert)

			if err != nil {
				ks.log.Error("Failed inserting cert [%s].", err.Error())
				continue
			}
		}

		err = tx.Commit()
		if err != nil {
			ks.log.Error("Failed committing transaction [%s].", err.Error())

			tx.Rollback()

			return nil, err
		}

		ks.log.Debug("Fectch TCerts from TCA...done!")

		cert, err = ks.selectNextTCert()
		if err != nil {
			ks.log.Error("Failed selecting next TCert after fetching [%s].", err.Error())

			return nil, err
		}
	}

	ks.log.Debug("TCert [%s].", utils.EncodeBase64(cert))

	return cert, nil
}

func (ks *keyStore) selectNextTCert() ([]byte, error) {
	ks.log.Debug("Lookup TCert in cache...")

	// Open transaction
	tx, err := ks.sqlDB.Begin()
	if err != nil {
		ks.log.Error("Failed beginning transaction [%s].", err.Error())

		return nil, err
	}

	// Get the first row available
	var id int
	var cert []byte
	row := ks.sqlDB.QueryRow("SELECT id, cert FROM TCerts")
	err = row.Scan(&id, &cert)

	if err == sql.ErrNoRows {
		ks.log.Error("No TCert in cache found.")

		return nil, nil
	} else if err != nil {
		ks.log.Error("Error during select [%s].", err.Error())

		return nil, err
	}

	ks.log.Debug("id [%d]", id)
	ks.log.Debug("cert [%s].", utils.EncodeBase64(cert))

	ks.log.Debug("Move row with id [%d] to UsedTCert...", id)

	// Copy to UsedTCert
	if _, err := tx.Exec("INSERT INTO UsedTCert (cert) VALUES (?)", cert); err != nil {
		ks.log.Error("Failed copying row [%d] to UsedTCert: [%s].", id, err.Error())

		tx.Rollback()

		return nil, err
	}

	// Remove from TCert
	if _, err := tx.Exec("DELETE FROM TCerts WHERE id = ?", id); err != nil {
		ks.log.Error("Failed removing row [%d] from TCert: [%s].", id, err.Error())

		tx.Rollback()

		return nil, err
	}

	ks.log.Debug("Moving row with id [%d]...done", id)

	// Finalize
	err = tx.Commit()
	if err != nil {
		ks.log.Error("Failed commiting [%s].", err.Error())
		tx.Rollback()

		return nil, err
	}

	ks.log.Debug("Lookup TCert in cache...done!")

	return cert, nil
}
