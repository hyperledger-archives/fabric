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

package obcpbft

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric/consensus"

	"github.com/spf13/viper"
)

const configPrefix = "CORE_PBFT"

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
	// set the whitelist cap in the peer package
	cap := config.GetInt("general.N")
	stack.SetWhitelistCap(cap)

	//instantiate
	mode := strings.ToLower(config.GetString("general.mode"))
	switch mode {
	case "classic":
		return newObcClassic(config, stack)
	case "batch":
		return newObcBatch(config, stack)
	case "sieve":
		return newObcSieve(config, stack)
	default:
		panic(fmt.Errorf("Invalid PBFT mode: %s", mode))
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
	config.AddConfigPath("../consensus/obcpbft/")
	config.AddConfigPath("../../consensus/obcpbft")
	// Path to look for the config file in based on GOPATH
	gopath := os.Getenv("GOPATH")
	for _, p := range filepath.SplitList(gopath) {
		obcpbftpath := filepath.Join(p, "src/github.com/hyperledger/fabric/consensus/obcpbft")
		config.AddConfigPath(obcpbftpath)
	}

	err := config.ReadInConfig()
	if err != nil {
		panic(fmt.Errorf("Error reading %s plugin config: %s", configPrefix, err))
	}
	return
}

type obcGeneric struct {
	stack consensus.Stack
	pbft  *pbftCore
}

func (op *obcGeneric) skipTo(seqNo uint64, id []byte, replicas []uint64) {
	op.stack.SkipTo(seqNo, id, op.stack.GetValidatorHandles(replicas))
}

func (op *obcGeneric) getState() []byte {
	return op.stack.GetBlockchainInfoBlob()
}

func (op *obcGeneric) getLastSeqNo() (uint64, error) {
	raw, err := op.stack.GetBlockHeadMetadata()
	if err != nil {
		return 0, err
	}
	meta := &Metadata{}
	proto.Unmarshal(raw, meta)
	return meta.SeqNo, nil
}

// StateUpdated is a signal from the stack that it has fast-forwarded its state
func (op *obcGeneric) StateUpdated(seqNo uint64, id []byte) {
	op.pbft.stateUpdated(seqNo, id)
}

// StateUpdating is a signal from the stack that state transfer has started
func (op *obcGeneric) StateUpdating(seqNo uint64, id []byte) {
	op.pbft.stateUpdating(seqNo, id)
}
