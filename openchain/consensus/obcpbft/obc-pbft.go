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

package obcpbft

import (
	"fmt"
	"strings"

	"github.com/openblockchain/obc-peer/openchain/consensus"

	"github.com/spf13/viper"
)

const configPrefix = "OPENCHAIN_OBCPBFT"

var pluginInstance consensus.Consenter // singleton service

// GetPlugin returns the handle to the Plugin singleton
func GetPlugin(c consensus.CPI) consensus.Consenter {
	if pluginInstance == nil {
		pluginInstance = New(c)
	}
	return pluginInstance
}

// New creates a new Obc* instance that provides the Consenter interface.
// Internally, it uses an opaque pbft-core instance.
func New(cpi consensus.CPI) consensus.Consenter {
	config := readConfig()
	addr, _, _ := cpi.GetReplicaHash()
	id, _ := cpi.GetReplicaID(addr)

	switch config.GetString("general.mode") {
	case "classic":
		return newObcClassic(id, config, cpi)
	case "batch":
		return newObcBatch(id, config, cpi)
	case "sieve":
		return newObcSieve(id, config, cpi)
	default:
		panic(fmt.Errorf("Invalid PBFT mode: %s", config.GetString("general.mode")))
	}
}

func readConfig() (config *viper.Viper) {
	config = viper.New()

	// for environment variables
	config.SetEnvPrefix(configPrefix)
	config.AutomaticEnv()
	replacer := strings.NewReplacer(".", "_")
	config.SetEnvKeyReplacer(replacer)

	config.SetConfigName("config")
	config.AddConfigPath("./")
	config.AddConfigPath("./openchain/consensus/obcpbft/")
	err := config.ReadInConfig()
	if err != nil {
		panic(fmt.Errorf("Fatal error reading consensus algo config: %s", err))
	}
	return
}
