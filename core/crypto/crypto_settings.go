/*
Copyright IBM Corp. 2016 All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

		 http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package crypto

import (
	"github.com/hyperledger/fabric/core/crypto/primitives"
	"github.com/op/go-logging"
	"github.com/spf13/viper"
)

var (
	log = logging.MustGetLogger("crypto")
)

// Init initializes the crypto layer. It load from viper the security level
// and the logging setting.
func Init() (err error) {
	// Init log
	log.ExtraCalldepth++

	level, err := logging.LogLevel(viper.GetString("logging.crypto"))
	if err == nil {
		// No error, use the setting
		logging.SetLevel(level, "crypto")
		log.Infof("Log level recognized '%s', set to %s", viper.GetString("logging.crypto"),
			logging.GetLevel("crypto"))
	} else {
		log.Warningf("Log level not recognized '%s', defaulting to %s: %s", viper.GetString("logging.crypto"),
			logging.GetLevel("crypto"), err)
	}

	// Init security level

	securityLevel := 256
	if viper.IsSet("security.level") {
		ovveride := viper.GetInt("security.level")
		if ovveride != 0 {
			securityLevel = ovveride
		}
	}

	hashAlgorithm := "SHA3"
	if viper.IsSet("security.hashAlgorithm") {
		ovveride := viper.GetString("security.hashAlgorithm")
		if ovveride != "" {
			hashAlgorithm = ovveride
		}
	}

	log.Debugf("Working at security level [%d]", securityLevel)
	if err = primitives.InitSecurityLevel(hashAlgorithm, securityLevel); err != nil {
		log.Debugf("Failed setting security level: [%s]", err)

		return
	}

	return
}
