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
	"strconv"
	"strings"

	"github.com/openblockchain/obc-peer/openchain/consensus"
	pb "github.com/openblockchain/obc-peer/protos"

	"github.com/spf13/viper"
)

const configPrefix = "OPENCHAIN_OBCPBFT"

var pluginInstance consensus.Consenter // singleton service
var config *viper.Viper

func init() {
	config = loadConfig()
}

// GetPlugin returns the handle to the Consenter singleton
func GetPlugin(c consensus.Stack) consensus.Consenter {
	if pluginInstance == nil {
		pluginInstance = New(c)
	}
	return pluginInstance
}

// New creates a new Obc* instance that provides the Consenter interface.
// Internally, it uses an opaque pbft-core instance.
func New(stack consensus.Stack) consensus.Consenter {
	handle, _, _ := stack.GetNetworkHandles()
	id, _ := getValidatorID(handle)

	switch strings.ToLower(config.GetString("general.mode")) {
	case "classic":
		return newObcClassic(id, config, stack)
	case "batch":
		return newObcBatch(id, config, stack)
	case "sieve":
		return newObcSieve(id, config, stack)
	default:
		panic(fmt.Errorf("Invalid PBFT mode: %s", config.GetString("general.mode")))
	}
}

func loadConfig() (config *viper.Viper) {
	config = viper.New()

	// for environment variables
	config.SetEnvPrefix(configPrefix)
	config.AutomaticEnv()
	replacer := strings.NewReplacer(".", "_")
	config.SetEnvKeyReplacer(replacer)

	config.SetConfigName("config")
	config.AddConfigPath("./")
	config.AddConfigPath("./openchain/consensus/obcpbft/")
	config.AddConfigPath("../../openchain/consensus/obcpbft")
	err := config.ReadInConfig()
	if err != nil {
		panic(fmt.Errorf("Error reading %s plugin config: %s", configPrefix, err))
	}
	return
}

// Returns the uint64 ID corresponding to a peer handle
func getValidatorID(handle *pb.PeerID) (id uint64, err error) {
	// as requested here: https://github.com/openblockchain/obc-peer/issues/462#issuecomment-170785410
	if startsWith := strings.HasPrefix(handle.Name, "vp"); startsWith {
		id, err = strconv.ParseUint(handle.Name[2:], 10, 64)
		if err != nil {
			return id, fmt.Errorf("Error extracting ID from \"%s\" handle: %v", handle.Name, err)
		}
		return
	}

	err = fmt.Errorf(`For MVP, set the VP's peer.id to vpX,
		where X is a unique integer between 0 and N-1
		(N being the maximum number of VPs in the network`)
	return
}

// Returns the peer handle that corresponds to a validator ID (uint64 assigned to it for PBFT)
func getValidatorHandle(id uint64) (handle *pb.PeerID, err error) {
	// as requested here: https://github.com/openblockchain/obc-peer/issues/462#issuecomment-170785410
	name := "vp" + strconv.FormatUint(id, 10)
	return &pb.PeerID{Name: name}, nil
}
