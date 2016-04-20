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

package chaincode

import (
	"flag"
	"fmt"
	"runtime"
	"strings"

	"github.com/op/go-logging"
	"github.com/spf13/viper"
)

// Config the config wrapper structure
type Config struct {
}

func init() {

}

// SetupTestLogging setup the logging during test execution
func SetupTestLogging() {
	level, err := logging.LogLevel(viper.GetString("logging.peer"))
	if err == nil {
		// No error, use the setting
		logging.SetLevel(level, "main")
		logging.SetLevel(level, "server")
		logging.SetLevel(level, "peer")
	} else {
		chaincodeLog.Warning("Log level not recognized '%s', defaulting to %s: %s", viper.GetString("logging.peer"), logging.ERROR, err)
		logging.SetLevel(logging.ERROR, "main")
		logging.SetLevel(logging.ERROR, "server")
		logging.SetLevel(logging.ERROR, "peer")
	}
}

// SetupTestConfig setup the config during test execution
func SetupTestConfig() {
	flag.Parse()

	// Now set the configuration file
	viper.SetEnvPrefix("HYPERLEDGER")
	viper.AutomaticEnv()
	replacer := strings.NewReplacer(".", "_")
	viper.SetEnvKeyReplacer(replacer)
	viper.SetConfigName("core")          // name of config file (without extension)
	viper.AddConfigPath("./")            // path to look for the config file in
	viper.AddConfigPath("./../../peer/") // path to look for the config file in
	err := viper.ReadInConfig()          // Find and read the config file
	if err != nil {                      // Handle errors reading the config file
		panic(fmt.Errorf("Fatal error config file: %s \n", err))
	}

	SetupTestLogging()

	// Set the number of maxprocs
	var numProcsDesired = viper.GetInt("peer.gomaxprocs")
	chaincodeLog.Debug("setting Number of procs to %d, was %d\n", numProcsDesired, runtime.GOMAXPROCS(2))

}
