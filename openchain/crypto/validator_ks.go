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

func (validator *validatorImpl) initKeyStore() error {
	// create tables
	log.Debug("Create Table [%s] at [%s]", "Certificates", validator.peer.node.conf.getKeysPath())
	if _, err := validator.peer.node.ks.sqlDB.Exec("CREATE TABLE IF NOT EXISTS Certificates (id VARCHAR, cert BLOB, PRIMARY KEY (id))"); err != nil {
		log.Debug("Failed creating table: %s", err)
		return err
	}

	validator.peer.node.ks.log.Debug("Keystore created at [%s]", validator.peer.node.conf.getKeysPath())

	return nil
}

func (ks *keyStore) GetEnrollmentCert(id []byte, certFetcher func(id []byte) ([]byte, error)) ([]byte, error) {
	ks.m.Lock()
	defer ks.m.Unlock()

	sid := utils.EncodeBase64(id)

	cert, err := ks.selectEnrollmentCert(sid)
	if err != nil {
		ks.log.Error("Failed selecting enrollment cert: %s", err)

		return nil, err
	}
	ks.log.Info("GetEnrollmentCert:cert %s", utils.EncodeBase64(cert))

	if cert == nil {
		// If No cert is available, fetch from ECA

		// 1. Fetch
		ks.log.Info("Fectch Enrollment Certificate from ECA...")
		cert, err = certFetcher(id)
		if err != nil {
			return nil, err
		}

		// 2. Store
		ks.log.Info("Store certificate...")
		tx, err := ks.sqlDB.Begin()
		if err != nil {
			ks.log.Error("Failed beginning transaction: %s", err)

			return nil, err
		}

		ks.log.Info("Insert id %s", sid)
		ks.log.Info("Insert cert %s", utils.EncodeBase64(cert))

		_, err = tx.Exec("INSERT INTO Certificates (id, cert) VALUES (?, ?)", sid, cert)

		if err != nil {
			ks.log.Error("Failed inserting cert %s", err)

			tx.Rollback()

			return nil, err
		}

		err = tx.Commit()
		if err != nil {
			ks.log.Error("Failed committing transaction: %s", err)

			tx.Rollback()

			return nil, err
		}

		ks.log.Info("Fectch Enrollment Certificate from ECA...done!")

		cert, err = ks.selectEnrollmentCert(sid)
		if err != nil {
			ks.log.Error("Failed selecting next TCert after fetching: %s", err)

			return nil, err
		}
	}

	return cert, nil
}

func (ks *keyStore) selectEnrollmentCert(id string) ([]byte, error) {
	ks.log.Info("Select Enrollment TCert...")

	// Get the first row available
	var cert []byte
	row := ks.sqlDB.QueryRow("SELECT cert FROM Certificates where id = ?", id)
	err := row.Scan(&cert)

	if err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		ks.log.Error("Error during select: %s", err)

		return nil, err
	}

	ks.log.Info("cert %s", utils.EncodeBase64(cert))

	ks.log.Info("Select Enrollment Cert...done!")

	return cert, nil
}
