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

const (
	TLSCA_CERT_CHAIN = "tlsca.cert.chain"
)

func (node *nodeImpl) initConfiguration(prefix, name string) error {
	// Set logger
	node.log = logging.MustGetLogger("CRYPTO." + prefix + "." + name)

	// Set configuration
	node.conf = &configuration{prefix: prefix, name: name}
	return node.conf.init()
}

type configuration struct {
	prefix string
	name   string

	configurationPath string
	keystorePath      string
	rawsPath          string

	configurationPathProperty string
	ecaPAddressProperty       string
	tcaPAddressProperty       string
	tlscaPAddressProperty     string

	tlsServerName string
}

func (conf *configuration) init() error {
	conf.configurationPathProperty = "peer.fileSystemPath"
	conf.ecaPAddressProperty = "peer.pki.eca.paddr"
	conf.tcaPAddressProperty = "peer.pki.tca.paddr"
	conf.tlscaPAddressProperty = "peer.pki.tlsca.paddr"

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
	if err := conf.checkProperty(conf.tlscaPAddressProperty); err != nil {
		return err
	}

	// Set configuration path
	conf.configurationPath = filepath.Join(
		viper.GetString(conf.configurationPathProperty),
		"crypto", conf.prefix, conf.name,
	)

	// Set ks path
	conf.keystorePath = filepath.Join(conf.configurationPath, "ks")

	// Set raws path
	conf.rawsPath = filepath.Join(conf.keystorePath, "raw")

	// Set TLS host override
	conf.tlsServerName = "tlsca"
	if viper.IsSet("peer.pki.tls.server-host-override") {
		ovveride := viper.GetString("peer.pki.tls.server-host-override")
		if ovveride != "" {
			conf.tlsServerName = ovveride
		}
	}

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

func (conf *configuration) getTLSCAPAddr() string {
	return viper.GetString(conf.tlscaPAddressProperty)
}

func (conf *configuration) getConfPath() string {
	return conf.configurationPath
}

func (conf *configuration) getKeyStorePath() string {
	return conf.keystorePath
}

func (conf *configuration) getRawsPath() string {
	return conf.rawsPath
}

func (conf *configuration) getKeyStoreFilename() string {
	return "db"
}

func (conf *configuration) getKeyStoreFilePath() string {
	return filepath.Join(conf.getKeyStorePath(), conf.getKeyStoreFilename())
}

func (conf *configuration) getPathForAlias(alias string) string {
	return filepath.Join(conf.getRawsPath(), alias)
}

func (conf *configuration) getEnrollmentKeyFilename() string {
	return "enrollment.key"
}

func (conf *configuration) getEnrollmentCertFilename() string {
	return "enrollment.cert"
}

func (conf *configuration) getEnrollmentIDPath() string {
	return filepath.Join(conf.getRawsPath(), conf.getEnrollmentIDFilename())
}

func (conf *configuration) getEnrollmentIDFilename() string {
	return "enrollment.id"
}

func (conf *configuration) getTCACertsChainFilename() string {
	return "tca.cert.chain"
}

func (conf *configuration) getECACertsChainFilename() string {
	return "eca.cert.chain"
}

func (conf *configuration) getTLSCACertsChainFilename() string {
	return TLSCA_CERT_CHAIN
}

func (conf *configuration) getTLSCACertsExternalPath() string {
	return viper.GetString("peer.pki.tls.rootcert.file")
}

func (conf *configuration) isTLSEnabled() bool {
	return viper.GetBool("peer.pki.tls.enabled")
}

func (conf *configuration) isTLSClientAuthEnabled() bool {
	return viper.GetBool("peer.pki.tls.client.auth.enabled")
}

func (conf *configuration) getTCAServerName() string {
	return conf.tlsServerName
}

func (conf *configuration) getECAServerName() string {
	return conf.tlsServerName
}

func (conf *configuration) getTLSCAServerName() string {
	return conf.tlsServerName
}

func (conf *configuration) getTLSKeyFilename() string {
	return "tls.key"
}

func (conf *configuration) getTLSCertFilename() string {
	return "tls.cert"
}

func (conf *configuration) getTLSRootCertFilename() string {
	return "tls.cert.chain"
}

func (conf *configuration) getEnrollmentChainKeyFilename() string {
	return "chain.key"
}

func (conf *configuration) getTCertOwnerKDFKeyFilename() string {
	return "tca.kdf.key"
}

//func (conf *configuration) getRole() string {
//	return viper.GetString(Role)
//}
//
//func (conf *configuration) getAffiliation() string {
//	return viper.GetString(Affiliation)
//}
