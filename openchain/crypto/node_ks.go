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
	"github.com/op/go-logging"
	"github.com/openblockchain/obc-peer/openchain/crypto/utils"
	"os"
	"path/filepath"
	"sync"

	// Required to succefully initialized the driver
	"crypto/x509"
	_ "github.com/mattn/go-sqlite3"
	"io/ioutil"
)

/*
var (
	defaultCerts = make(map[string][]byte)
)

func addDefaultCert(key string, cert []byte) error {
	log.Debug("Adding Default Cert [%s][%s]", key, utils.EncodeBase64(cert))

	der, err := utils.PEMtoDER(cert)
	if err != nil {
		log.Error("Failed adding default cert: [%s]", err)

		return err
	}

	defaultCerts[key] = der

	return nil
}
*/

func (node *nodeImpl) initKeyStore(pwd []byte) error {
	ks := keyStore{}
	if err := ks.init(node.log, node.conf, pwd); err != nil {
		return err
	}
	node.ks = &ks

	/*
		// Add default certs
		for key, value := range defaultCerts {
			node.log.Debug("Adding Default Cert to the keystore [%s][%s]", key, utils.EncodeBase64(value))
			ks.storeCert(key, value)
		}
	*/

	return nil
}

type keyStore struct {
	isOpen bool

	pwd []byte

	// backend
	sqlDB *sql.DB

	// Configuration
	conf *configuration

	// Logging
	log *logging.Logger

	// Sync
	m sync.Mutex
}

func (ks *keyStore) init(logger *logging.Logger, conf *configuration, pwd []byte) error {
	ks.m.Lock()
	defer ks.m.Unlock()

	if ks.isOpen {
		return utils.ErrKeyStoreAlreadyInitialized
	}

	ks.log = logger
	ks.conf = conf
	ks.pwd = utils.Clone(pwd)

	err := ks.createKeyStoreIfNotExists()
	if err != nil {
		return err
	}

	err = ks.openKeyStore()
	if err != nil {
		return err
	}

	return nil
}

func (ks *keyStore) isAliasSet(alias string) bool {
	missing, _ := utils.FilePathMissing(ks.conf.getPathForAlias(alias))
	if missing {
		return false
	}

	return true
}

func (ks *keyStore) storePrivateKey(alias string, privateKey interface{}) error {
	rawKey, err := utils.PrivateKeyToPEM(privateKey, ks.pwd)
	if err != nil {
		ks.log.Error("Failed converting private key to PEM [%s]: [%s]", alias, err)
		return err
	}

	err = ioutil.WriteFile(ks.conf.getPathForAlias(alias), rawKey, 0700)
	if err != nil {
		ks.log.Error("Failed storing private key [%s]: [%s]", alias, err)
		return err
	}

	return nil
}

func (ks *keyStore) storePrivateKeyInClear(alias string, privateKey interface{}) error {
	rawKey, err := utils.PrivateKeyToPEM(privateKey, nil)
	if err != nil {
		ks.log.Error("Failed converting private key to PEM [%s]: [%s]", alias, err)
		return err
	}

	err = ioutil.WriteFile(ks.conf.getPathForAlias(alias), rawKey, 0700)
	if err != nil {
		ks.log.Error("Failed storing private key [%s]: [%s]", alias, err)
		return err
	}

	return nil
}

func (ks *keyStore) loadPrivateKey(alias string) (interface{}, error) {
	path := ks.conf.getPathForAlias(alias)
	ks.log.Debug("Loading private key [%s] at [%s]...", alias, path)

	raw, err := ioutil.ReadFile(path)
	if err != nil {
		ks.log.Error("Failed loading private key [%s]: [%s].", alias, err.Error())

		return nil, err
	}

	privateKey, err := utils.PEMtoPrivateKey(raw, ks.pwd)
	if err != nil {
		ks.log.Error("Failed parsing private key [%s]: [%s].", alias, err.Error())

		return nil, err
	}

	return privateKey, nil
}

func (ks *keyStore) storeKey(alias string, key []byte) error {
	pem, err := utils.AEStoEncryptedPEM(key, ks.pwd)
	if err != nil {
		ks.log.Error("Failed converting key to PEM [%s]: [%s]", alias, err)
		return err
	}

	err = ioutil.WriteFile(ks.conf.getPathForAlias(alias), pem, 0700)
	if err != nil {
		ks.log.Error("Failed storing key [%s]: [%s]", alias, err)
		return err
	}

	return nil
}

func (ks *keyStore) loadKey(alias string) ([]byte, error) {
	path := ks.conf.getPathForAlias(alias)
	ks.log.Debug("Loading key [%s] at [%s]...", alias, path)

	pem, err := ioutil.ReadFile(path)
	if err != nil {
		ks.log.Error("Failed loading key [%s]: [%s].", alias, err.Error())

		return nil, err
	}

	key, err := utils.PEMtoAES(pem, ks.pwd)
	if err != nil {
		ks.log.Error("Failed parsing key [%s]: [%s]", alias, err)

		return nil, err
	}

	return key, nil
}

func (ks *keyStore) storeCert(alias string, der []byte) error {
	err := ioutil.WriteFile(ks.conf.getPathForAlias(alias), utils.DERCertToPEM(der), 0700)
	if err != nil {
		ks.log.Error("Failed storing certificate [%s]: [%s]", alias, err)
		return err
	}

	return nil
}

func (ks *keyStore) loadCert(alias string) ([]byte, error) {
	path := ks.conf.getPathForAlias(alias)
	ks.log.Debug("Loading certificate [%s] at [%s]...", alias, path)

	pem, err := ioutil.ReadFile(path)
	if err != nil {
		ks.log.Error("Failed loading certificate [%s]: [%s].", alias, err.Error())

		return nil, err
	}

	return pem, nil
}

func (ks *keyStore) loadExternalCert(path string) ([]byte, error) {
	ks.log.Debug("Loading external certificate at [%s]...", path)

	pem, err := ioutil.ReadFile(path)
	if err != nil {
		ks.log.Error("Failed loading external certificate: [%s].", err.Error())

		return nil, err
	}

	return pem, nil
}

func (ks *keyStore) loadCertX509AndDer(alias string) (*x509.Certificate, []byte, error) {
	path := ks.conf.getPathForAlias(alias)
	ks.log.Debug("Loading certificate [%s] at [%s]...", alias, path)

	pem, err := ioutil.ReadFile(path)
	if err != nil {
		ks.log.Error("Failed loading certificate [%s]: [%s].", alias, err.Error())

		return nil, nil, err
	}

	cert, der, err := utils.PEMtoCertificateAndDER(pem)
	if err != nil {
		ks.log.Error("Failed parsing certificate [%s]: [%s].", alias, err.Error())

		return nil, nil, err
	}

	return cert, der, nil
}

func (ks *keyStore) close() error {
	ks.log.Info("Closing keystore...")
	err := ks.sqlDB.Close()

	if err != nil {
		ks.log.Error("Failed closing keystore [%s].", err.Error())
	} else {
		ks.log.Info("Closing keystore...done!")
	}

	ks.isOpen = false
	return err
}

func (ks *keyStore) createKeyStoreIfNotExists() error {
	// Check keystore directory
	ksPath := ks.conf.getKeyStorePath()
	missing, err := utils.DirMissingOrEmpty(ksPath)
	ks.log.Debug("Keystore path [%s] missing [%t]: [%s]", ksPath, missing, utils.ErrToString(err))

	if !missing {
		// Check keystore file
		missing, err = utils.FileMissing(ks.conf.getKeyStorePath(), ks.conf.getKeyStoreFilename())
		ks.log.Debug("Keystore [%s] missing [%t]:[%s]", ks.conf.getKeyStoreFilePath(), missing, utils.ErrToString(err))
	}

	if missing {
		err := ks.createKeyStore()
		if err != nil {
			ks.log.Debug("Failed creating db At [%s]: ", ks.conf.getKeyStoreFilePath(), err.Error())
			return nil
		}
	}

	return nil
}

func (ks *keyStore) createKeyStore() error {
	// Create keystore directory root if it doesn't exist yet
	ksPath := ks.conf.getKeyStorePath()
	ks.log.Debug("Creating Keystore at [%s]...", ksPath)

	missing, err := utils.FileMissing(ksPath, ks.conf.getKeyStoreFilename())
	if !missing {
		ks.log.Debug("Creating Keystore at [%s]. Keystore already there", ksPath)
		return nil
	}

	os.MkdirAll(ksPath, 0755)

	// Create Raw material folder
	os.MkdirAll(ks.conf.getRawsPath(), 0755)

	// Create DB
	ks.log.Debug("Open Keystore DB...")
	db, err := sql.Open("sqlite3", filepath.Join(ksPath, ks.conf.getKeyStoreFilename()))
	if err != nil {
		return err
	}

	ks.log.Debug("Ping Keystore DB...")
	err = db.Ping()
	if err != nil {
		ks.log.Fatal(err)
	}
	defer db.Close()

	ks.log.Debug("Keystore created at [%s].", ksPath)
	return nil
}

func (ks *keyStore) deleteKeyStore() error {
	ks.log.Debug("Removing KeyStore at [%s].", ks.conf.getKeyStorePath())

	return os.RemoveAll(ks.conf.getKeyStorePath())
}

func (ks *keyStore) openKeyStore() error {
	if ks.isOpen {
		return nil
	}

	// Open DB
	ksPath := ks.conf.getKeyStorePath()
	ks.log.Debug("Open keystore at [%s]...", ksPath)

	sqlDB, err := sql.Open("sqlite3", filepath.Join(ksPath, ks.conf.getKeyStoreFilename()))
	if err != nil {
		ks.log.Error("Error opening keystore%s", err.Error())
		return err
	}
	ks.isOpen = true
	ks.sqlDB = sqlDB

	ks.log.Debug("Open keystore at [%s]...done", ksPath)

	return nil
}
