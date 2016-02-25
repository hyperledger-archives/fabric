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
	validator.debug("Create Table [%s] if not exists", "Certificates")
	if _, err := validator.ks.sqlDB.Exec("CREATE TABLE IF NOT EXISTS Certificates (id VARCHAR, certsign BLOB, certenc BLOB, PRIMARY KEY (id))"); err != nil {
		validator.debug("Failed creating table [%s].", err.Error())
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
		ks.node.error("Failed selecting enrollment cert [%s].", err.Error())

		return nil, err
	}

	if certSign == nil {
		ks.node.debug("Cert for [%s] not available. Fetching from ECA....", sid)

		// If No cert is available, fetch from ECA

		// 1. Fetch
		ks.node.debug("Fectch Enrollment Certificate from ECA...")
		certSign, certEnc, err = certFetcher(id)
		if err != nil {
			return nil, err
		}

		// 2. Store
		ks.node.debug("Store certificate...")
		tx, err := ks.sqlDB.Begin()
		if err != nil {
			ks.node.error("Failed beginning transaction [%s].", err.Error())

			return nil, err
		}

		ks.node.debug("Insert id [%s].", sid)
		ks.node.debug("Insert cert [% x].", certSign)

		_, err = tx.Exec("INSERT INTO Certificates (id, certsign, certenc) VALUES (?, ?, ?)", sid, certSign, certEnc)

		if err != nil {
			ks.node.error("Failed inserting cert [%s].", err.Error())

			tx.Rollback()

			return nil, err
		}

		err = tx.Commit()
		if err != nil {
			ks.node.error("Failed committing transaction [%s].", err.Error())

			tx.Rollback()

			return nil, err
		}

		ks.node.debug("Fectch Enrollment Certificate from ECA...done!")

		certSign, certEnc, err = ks.selectSignEnrollmentCert(sid)
		if err != nil {
			ks.node.error("Failed selecting next TCert after fetching [%s].", err.Error())

			return nil, err
		}
	}

	ks.node.debug("Cert for [%s] = [% x]", sid, certSign)

	return certSign, nil
}

func (ks *keyStore) selectSignEnrollmentCert(id string) ([]byte, []byte, error) {
	ks.node.debug("Select Sign Enrollment Cert for id [%s]", id)

	// Get the first row available
	var cert []byte
	row := ks.sqlDB.QueryRow("SELECT certsign FROM Certificates where id = ?", id)
	err := row.Scan(&cert)

	if err == sql.ErrNoRows {
		return nil, nil, nil
	} else if err != nil {
		ks.node.error("Error during select [%s].", err.Error())

		return nil, nil, err
	}

	ks.node.debug("Cert [% x].", cert)

	ks.node.debug("Select Enrollment Cert...done!")

	return cert, nil, nil
}
