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
	"os"
)

func (client *clientImpl) initKeyStore() error {
	// Create TCerts directory
	os.MkdirAll(client.conf.getTCertsPath(), 0755)

	// create tables
	client.debug("Create Table if not exists [TCert] at [%s].", client.conf.getKeyStorePath())
	if _, err := client.ks.sqlDB.Exec("CREATE TABLE IF NOT EXISTS TCerts (id INTEGER, cert BLOB, PRIMARY KEY (id))"); err != nil {
		client.debug("Failed creating table [%s].", err)
		return err
	}

	client.debug("Create Table if not exists [UsedTCert] at [%s].", client.conf.getKeyStorePath())
	if _, err := client.ks.sqlDB.Exec("CREATE TABLE IF NOT EXISTS UsedTCert (id INTEGER, cert BLOB, PRIMARY KEY (id))"); err != nil {
		client.debug("Failed creating table [%s].", err)
		return err
	}

	return nil
}

func (ks *keyStore) storeUsedTCert(tCert tCert) (err error) {
	ks.m.Lock()
	defer ks.m.Unlock()

	ks.node.debug("Storing used TCert...")

	// Open transaction
	tx, err := ks.sqlDB.Begin()
	if err != nil {
		ks.node.error("Failed beginning transaction [%s].", err)

		return
	}

	// Insert into UsedTCert
	if _, err = tx.Exec("INSERT INTO UsedTCert (cert) VALUES (?)", tCert.GetCertificate().Raw); err != nil {
		ks.node.error("Failed inserting TCert to UsedTCert: [%s].", err)

		tx.Rollback()

		return
	}

	// Finalize
	err = tx.Commit()
	if err != nil {
		ks.node.error("Failed commiting [%s].", err)
		tx.Rollback()

		return
	}

	ks.node.debug("Storing used TCert...done!")

	//name, err := utils.TempFile(ks.conf.getTCertsPath(), "tcert_")
	//if err != nil {
	//	ks.node.error("Failed storing TCert: [%s]", err)
	//	return
	//}
	//
	//err = ioutil.WriteFile(name, tCert.GetCertificate().Raw, 0700)
	//if err != nil {
	//	ks.node.error("Failed storing TCert: [%s]", err)
	//	return
	//}

	return
}

func (ks *keyStore) storeUnusedTCerts(tCerts []tCert) (err error) {
	ks.node.debug("Storing unused TCerts...")

	if len(tCerts) == 0 {
		ks.node.debug("Empty list of unused TCerts.")
		return
	}

	// Open transaction
	tx, err := ks.sqlDB.Begin()
	if err != nil {
		ks.node.error("Failed beginning transaction [%s].", err)

		return
	}

	for _, tCert := range tCerts {
		// Insert into UsedTCert
		if _, err = tx.Exec("INSERT INTO TCerts (cert) VALUES (?)", tCert.GetCertificate().Raw); err != nil {
			ks.node.error("Failed inserting unused TCert to TCerts: [%s].", err)

			tx.Rollback()

			return
		}
	}

	// Finalize
	err = tx.Commit()
	if err != nil {
		ks.node.error("Failed commiting [%s].", err)
		tx.Rollback()

		return
	}

	ks.node.debug("Storing unused TCerts...done!")

	return
}

func (ks *keyStore) loadUnusedTCert() ([]byte, error) {
	// Get the first row available
	var id int
	var cert []byte
	row := ks.sqlDB.QueryRow("SELECT id, cert FROM TCerts")
	err := row.Scan(&id, &cert)

	if err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		ks.node.error("Error during select [%s].", err.Error())

		return nil, err
	}

	// Remove from TCert
	if _, err := ks.sqlDB.Exec("DELETE FROM TCerts WHERE id = ?", id); err != nil {
		ks.node.error("Failed removing row [%d] from TCert: [%s].", id, err.Error())

		return nil, err
	}

	return cert, nil
}

func (ks *keyStore) loadUnusedTCerts() ([][]byte, error) {
	// Get unused TCerts
	rows, err := ks.sqlDB.Query("SELECT cert FROM TCerts")
	if err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		ks.node.error("Error during select [%s].", err)

		return nil, err
	}

	tCertDERs := [][]byte{}
	for {
		if rows.Next() {
			var tCertDER []byte
			if err := rows.Scan(&tCertDER); err != nil {
				ks.node.error("Error during scan [%s].", err)

				continue
			}
			tCertDERs = append(tCertDERs, tCertDER)
		} else {
			break
		}
	}

	// Delete all entries
	if _, err = ks.sqlDB.Exec("DELETE FROM TCerts"); err != nil {
		ks.node.error("Failed cleaning up unused TCert entries: [%s].", err)

		return nil, err
	}

	return tCertDERs, nil
}
