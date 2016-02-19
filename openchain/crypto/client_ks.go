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
	"os"
)

func (client *clientImpl) initKeyStore() error {
	// Create TCerts directory
	os.MkdirAll(client.node.conf.getTCertsPath(), 0755)

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

func (ks *keyStore) storeTCert(tCert tCert) (err error) {
	ks.log.Debug("Storing used TCert...")

	// Open transaction
	tx, err := ks.sqlDB.Begin()
	if err != nil {
		ks.log.Error("Failed beginning transaction [%s].", err.Error())

		return
	}

	// Insert into UsedTCert
	if _, err = tx.Exec("INSERT INTO UsedTCert (cert) VALUES (?)", tCert.GetCertificate().Raw); err != nil {
		ks.log.Error("Failed inserting TCert to UsedTCert: [%s].", err)

		tx.Rollback()

		return
	}

	// Finalize
	err = tx.Commit()
	if err != nil {
		ks.log.Error("Failed commiting [%s].", err.Error())
		tx.Rollback()

		return
	}

	ks.log.Debug("Storing used TCert...done!")

	//name, err := utils.TempFile(ks.conf.getTCertsPath(), "tcert_")
	//if err != nil {
	//	ks.log.Error("Failed storing TCert: [%s]", err)
	//	return
	//}
	//
	//err = ioutil.WriteFile(name, tCert.GetCertificate().Raw, 0700)
	//if err != nil {
	//	ks.log.Error("Failed storing TCert: [%s]", err)
	//	return
	//}

	return
}
