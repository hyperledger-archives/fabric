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

package node

import (
	"fmt"

	"github.com/hyperledger/fabric/core"
	"github.com/op/go-logging"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const nodeFuncName = "node"

var (
	stopPidFile string
)
var logger = logging.MustGetLogger("nodeCmd")

// Cmd returns the cobra command for Node
func Cmd() *cobra.Command {
	nodeCmd.AddCommand(startCmd())
	nodeCmd.AddCommand(nodeStatusCmd)

	nodeStopCmd.Flags().StringVar(&stopPidFile, "stop-peer-pid-file", viper.GetString("peer.fileSystemPath"), "Location of peer pid local file, for forces kill")
	nodeCmd.AddCommand(nodeStopCmd)

	return nodeCmd
}

var nodeCmd = &cobra.Command{
	Use:   nodeFuncName,
	Short: fmt.Sprintf("%s specific commands.", nodeFuncName),
	Long:  fmt.Sprintf("%s specific commands.", nodeFuncName),
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		core.LoggingInit(nodeFuncName)
	},
}
