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

package client

import (
	"errors"
	"github.com/spf13/viper"
	"path/filepath"
)

const (
	ConfigurationPath = "client.crypto.path"
	ECAPAddress       = "client.crypto.eca.paddr"
	TCAPAddress       = "client.crypto.tca.paddr"
)

func initConf() error {
	// Check mandatory fields
	if err := checkProperty(ConfigurationPath); err != nil {
		return err
	}
	if err := checkProperty(ECAPAddress); err != nil {
		return err
	}
	if err := checkProperty(TCAPAddress); err != nil {
		return err
	}
	return nil
}

func checkProperty(property string) error {
	res := viper.GetString(property)
	if res == "" {
		return errors.New("Property not specified in configuration file. Please check that property is set: " + property)
	}
	return nil
}

func getTCAPAddr() string {
	return viper.GetString(TCAPAddress)
}

func getECAPAddr() string {
	return viper.GetString(ECAPAddress)
}

func getConfPath() string {
	return viper.GetString(ConfigurationPath)
}

func getDBPath() string {
	return getConfPath()
}

func getDBFilename() string {
	return "client.db"
}

func getDBFilePath() string {
	return filepath.Join(getDBPath(), getDBFilename())
}

func getKeysPath() string {
	return getConfPath()
}

func getEnrollmentKeyPath() string {
	return filepath.Join(getKeysPath(), getEnrollmentKeyFilename())
}

func getEnrollmentKeyFilename() string {
	return "enrollment.key"
}

func getEnrollmentCertPath() string {
	return filepath.Join(getKeysPath(), getEnrollmentCertFilename())
}

func getEnrollmentCertFilename() string {
	return "enrollment.cert"
}

func getEnrollmentIDPath() string {
	return filepath.Join(getKeysPath(), getEnrollmentIDFilename())
}

func getEnrollmentIDFilename() string {
	return "enrollment.id"
}

func getEnrollmentChainKeyPath() string {
	return filepath.Join(getKeysPath(), getEnrollmentChainKeyFilename())
}

func getEnrollmentChainKeyFilename() string {
	return "chain.key"
}

func getTCACertsChainPath() string {
	return filepath.Join(getKeysPath(), getTCACertsChainFilename())
}

func getTCACertsChainFilename() string {
	return "tca.cert.chain"
}

func getTCertOwnerKDFKeyPath() string {
	return filepath.Join(getKeysPath(), getTCertOwnerKDFKeyFilename())
}

func getTCertOwnerKDFKeyFilename() string {
	return "tca.kdf.key"
}

func getECACertsChainPath() string {
	return filepath.Join(getKeysPath(), getECACertsChainFilename())
}

func getECACertsChainFilename() string {
	return "eca.cert.chain"
}
