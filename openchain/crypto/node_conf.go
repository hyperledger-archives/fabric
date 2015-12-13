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
	"errors"
	"github.com/op/go-logging"
	"github.com/spf13/viper"
	"path/filepath"
)

func (node *nodeImpl) initConfiguration(prefix, name string) error {
	// Set logger
	node.log = logging.MustGetLogger("CRYPTO." + prefix + "." + name)

	// Set configuration
	node.conf = &configuration{prefix: prefix, name: name}
	return node.conf.loadConfiguration()
}

type configuration struct {
	prefix string
	name   string

	configurationPath string

	configurationPathProperty string
	ecaPAddressProperty string
	tcaPAddressProperty string
}

func (conf *configuration) loadConfiguration() error {
	conf.configurationPathProperty = "peer.fileSystemPath"
	conf.ecaPAddressProperty = "peer.pki.eca.paddr"
	conf.tcaPAddressProperty = "peer.pki.tca.paddr"

	// Check mandatory fields
	if err := conf.checkProperty(conf.configurationPathProperty); err != nil {
		return err
	}
	if err := conf.checkProperty(conf.ecaPAddressProperty); err != nil {
		return err
	}
	if err := conf.checkProperty(conf.tcaPAddressProperty); err != nil {
		return err
	}

	// Set configuration path
	conf.configurationPath = filepath.Join(
		viper.GetString(conf.configurationPathProperty),
		"crypto", conf.prefix, conf.name,
	)


	return nil
}

func (conf *configuration) checkProperty(property string) error {
	res := viper.GetString(property)
	if res == "" {
		return errors.New("Property not specified in configuration file. Please check that property is set: " + property)
	}
	return nil
}

func (conf *configuration) getTCAPAddr() string {
	return viper.GetString(conf.tcaPAddressProperty)
}

func (conf *configuration) getECAPAddr() string {
	return viper.GetString(conf.ecaPAddressProperty)
}

func (conf *configuration) getConfPath() string {
	return conf.configurationPath
}

func (conf *configuration) getKeyStorePath() string {
	return conf.getConfPath()
}

func (conf *configuration) getKeyStoreFilename() string {
	return "keystore"
}

func (conf *configuration) getKeyStoreFilePath() string {
	return filepath.Join(conf.getKeyStorePath(), conf.getKeyStoreFilename())
}

func (conf *configuration) getKeysPath() string {
	return conf.getConfPath()
}

func (conf *configuration) getEnrollmentKeyPath() string {
	return filepath.Join(conf.getKeysPath(), conf.getEnrollmentKeyFilename())
}

func (conf *configuration) getEnrollmentKeyFilename() string {
	return "enrollment.key"
}

func (conf *configuration) getEnrollmentCertPath() string {
	return filepath.Join(conf.getKeysPath(), conf.getEnrollmentCertFilename())
}

func (conf *configuration) getEnrollmentCertFilename() string {
	return "enrollment.cert"
}

func (conf *configuration) getEnrollmentIDPath() string {
	return filepath.Join(conf.getKeysPath(), conf.getEnrollmentIDFilename())
}

func (conf *configuration) getEnrollmentIDFilename() string {
	return "enrollment.id"
}

func (conf *configuration) getTCACertsChainPath() string {
	return filepath.Join(conf.getKeysPath(), conf.getTCACertsChainFilename())
}

func (conf *configuration) getTCACertsChainFilename() string {
	return "tca.cert.chain"
}

func (conf *configuration) getECACertsChainPath() string {
	return filepath.Join(conf.getKeysPath(), conf.getECACertsChainFilename())
}

func (conf *configuration) getECACertsChainFilename() string {
	return "eca.cert.chain"
}
