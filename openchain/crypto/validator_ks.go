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
	"github.com/openblockchain/obc-peer/openchain/crypto/utils"
)

func (validator *validatorImpl) initKeyStore() error {
	// create tables
	validator.peer.node.log.Debug("Create Table [%s] if not exists", "Certificates")
	if _, err := validator.peer.node.ks.sqlDB.Exec("CREATE TABLE IF NOT EXISTS Certificates (id VARCHAR, certsign BLOB, certenc BLOB, PRIMARY KEY (id))"); err != nil {
		validator.peer.node.log.Debug("Failed creating table [%s].", err.Error())
		return err
	}

	return nil
}

func (ks *keyStore) GetSignEnrollmentCert(id []byte, certFetcher func(id []byte) ([]byte, []byte, error)) ([]byte, error) {
	if len(id) == 0 {
		return nil, fmt.Errorf("Invalid peer id. It is empty.")
	}

	ks.m.Lock()
	defer ks.m.Unlock()

	sid := utils.EncodeBase64(id)

	certSign, certEnc, err := ks.selectSignEnrollmentCert(sid)
	if err != nil {
		ks.log.Error("Failed selecting enrollment cert [%s].", err.Error())

		return nil, err
	}

	if certSign == nil {
		ks.log.Debug("Cert for [%s] not available. Fetching from ECA....", utils.EncodeBase64(id))

		// If No cert is available, fetch from ECA

		// 1. Fetch
		ks.log.Debug("Fectch Enrollment Certificate from ECA...")
		certSign, certEnc, err = certFetcher(id)
		if err != nil {
			return nil, err
		}

		// 2. Store
		ks.log.Debug("Store certificate...")
		tx, err := ks.sqlDB.Begin()
		if err != nil {
			ks.log.Error("Failed beginning transaction [%s].", err.Error())

			return nil, err
		}

		ks.log.Debug("Insert id [%s].", sid)
		ks.log.Debug("Insert cert [%s].", utils.EncodeBase64(certSign))

		_, err = tx.Exec("INSERT INTO Certificates (id, certsign, certenc) VALUES (?, ?, ?)", sid, certSign, certEnc)

		if err != nil {
			ks.log.Error("Failed inserting cert [%s].", err.Error())

			tx.Rollback()

			return nil, err
		}

		err = tx.Commit()
		if err != nil {
			ks.log.Error("Failed committing transaction [%s].", err.Error())

			tx.Rollback()

			return nil, err
		}

		ks.log.Debug("Fectch Enrollment Certificate from ECA...done!")

		certSign, certEnc, err = ks.selectSignEnrollmentCert(sid)
		if err != nil {
			ks.log.Error("Failed selecting next TCert after fetching [%s].", err.Error())

			return nil, err
		}
	}

	ks.log.Debug("Cert for [%s] = [%s]", sid, utils.EncodeBase64(certSign))

	return certSign, nil
}

func (ks *keyStore) selectSignEnrollmentCert(id string) ([]byte, []byte, error) {
	ks.log.Debug("Select Sign Enrollment Cert for id [%s]", id)

	// Get the first row available
	var cert []byte
	row := ks.sqlDB.QueryRow("SELECT certsign FROM Certificates where id = ?", id)
	err := row.Scan(&cert)

	if err == sql.ErrNoRows {
		return nil, nil, nil
	} else if err != nil {
		ks.log.Error("Error during select [%s].", err.Error())

		return nil, nil, err
	}

	ks.log.Debug("Cert [%s].", utils.EncodeBase64(cert))

	ks.log.Debug("Select Enrollment Cert...done!")

	return cert, nil, nil
}
