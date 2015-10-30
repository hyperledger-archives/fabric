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

package pbft

import (
	"fmt"

	"github.com/openblockchain/obc-peer/openchain/consensus"

	"github.com/op/go-logging"
	"github.com/spf13/viper"
)

// Plugin carries fields related to the consensus algorithm.
type Plugin struct {
	cpi    consensus.CPI // The consensus programming interface
	config *viper.Viper  // The link to the config file
	// blockTimeOut int
}

// Runs when the package is loaded.
func init() {
	// TODO: Empty for now. Populate as needed.
}

// New allocates a new instance/implementation of the consensus algorithm.
func New(c consensus.CPI) consensus.Consenter {
	if consensus.Logger.IsEnabledFor(logging.DEBUG) {
		consensus.Logger.Debug("Creating the consenter.")
	}
	instance := &Plugin{}

	if consensus.Logger.IsEnabledFor(logging.DEBUG) {
		consensus.Logger.Debug("Setting the consenter's CPI.")
	}
	instance.cpi = c

	// Create a link to the config file.
	if consensus.Logger.IsEnabledFor(logging.DEBUG) {
		consensus.Logger.Debug("Linking to the consenter's config file.")
	}
	instance.config = viper.New()
	instance.config.SetConfigName("config")
	instance.config.AddConfigPath("./")
	instance.config.AddConfigPath("../pbft/")
	err := instance.config.ReadInConfig()
	if err != nil {
		panic(fmt.Errorf("Fatal error reading consensus algo config: %s", err))
	}
	// TODO: Initialize the algorithm here.
	// You'll want to set the fields of `instance` using `instance.GetParam()`.
	// e.g. instance.blockTimeOut = strconv.Atoi(instance.GetParam("timeout.block"))
	return instance
}

// GetParam is a getter for the values listed in `config.yaml`.
func (instance *Plugin) GetParam(param string) (val string, err error) {
	if ok := instance.config.IsSet(param); !ok {
		err := fmt.Errorf("Key %s does not exist in algo config.", param)
		return "nil", err
	}
	val = instance.config.GetString(param)
	return val, nil
}

// Recv allows the algorithm to receive (and process) a message.
func (instance *Plugin) Recv(msg []byte) (err error) {

	// TODO: Add logic here.
	if consensus.Logger.IsEnabledFor(logging.INFO) {
		consensus.Logger.Info("Message received.")
	}

	return nil
}
