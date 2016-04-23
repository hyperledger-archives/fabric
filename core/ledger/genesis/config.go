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

package genesis

import (
	"sync"

	"github.com/spf13/viper"
)

var loadConfigOnce sync.Once

var genesis map[string]interface{}
var mode string
var deploySystemChaincodeEnabled bool

func initConfigs() {
	loadConfigOnce.Do(func() { loadConfigs() })
}

func loadConfigs() {
	genesisLogger.Info("Loading configurations...")
	genesis = viper.GetStringMap("ledger.blockchain.genesisBlock")
	mode = viper.GetString("chaincode.chaincoderunmode")
	genesisLogger.Info("Configurations loaded: genesis=%s, mode=[%s], deploySystemChaincodeEnabled=[%t]",
		genesis, mode, deploySystemChaincodeEnabled)
	if viper.IsSet("ledger.blockchain.deploy-system-chaincode") {
		// If the deployment of system chaincode is enabled in the configuration file return the configured value
		deploySystemChaincodeEnabled = viper.GetBool("ledger.blockchain.deploy-system-chaincode")
	} else {
		// Deployment of system chaincode is enabled by default if no configuration was specified.
		deploySystemChaincodeEnabled = true
	}
}

func getGenesis() map[string]interface{} {
	initConfigs()
	return genesis
}

func getMode() string {
	initConfigs()
	return mode
}

func isDeploySystemChaincodeEnabled() bool {
	initConfigs()
	return deploySystemChaincodeEnabled
}
