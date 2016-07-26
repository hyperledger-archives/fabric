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

package chaincode

import (
	"fmt"

	"github.com/hyperledger/fabric/core"
	"github.com/op/go-logging"
	"github.com/spf13/cobra"
)

const (
	chainFuncName       = "chaincode"
	undefinedParamValue = ""
)

var logger = logging.MustGetLogger("chaincodeCmd")

// Cmd returns the cobra command for Chaincode
func Cmd() *cobra.Command {
	chaincodeCmd.PersistentFlags().StringVarP(&chaincodeLang, "lang", "l", "golang", fmt.Sprintf("Language the %s is written in", chainFuncName))
	chaincodeCmd.PersistentFlags().StringVarP(&chaincodeCtorJSON, "ctor", "c", "{}", fmt.Sprintf("Constructor message for the %s in JSON format", chainFuncName))
	chaincodeCmd.PersistentFlags().StringVarP(&chaincodeAttributesJSON, "attributes", "a", "[]", fmt.Sprintf("User attributes for the %s in JSON format", chainFuncName))
	chaincodeCmd.PersistentFlags().StringVarP(&chaincodePath, "path", "p", undefinedParamValue, fmt.Sprintf("Path to %s", chainFuncName))
	chaincodeCmd.PersistentFlags().StringVarP(&chaincodeName, "name", "n", undefinedParamValue, fmt.Sprintf("Name of the chaincode returned by the deploy transaction"))
	chaincodeCmd.PersistentFlags().StringVarP(&chaincodeUsr, "username", "u", undefinedParamValue, fmt.Sprintf("Username for chaincode operations when security is enabled"))
	chaincodeCmd.PersistentFlags().StringVarP(&customIDGenAlg, "tid", "t", undefinedParamValue, fmt.Sprintf("Name of a custom ID generation algorithm (hashing and decoding) e.g. sha256base64"))

	chaincodeCmd.AddCommand(deployCmd())
	chaincodeCmd.AddCommand(invokeCmd())
	chaincodeCmd.AddCommand(queryCmd())

	return chaincodeCmd
}

// Chaincode-related variables.
var (
	chaincodeLang           string
	chaincodeCtorJSON       string
	chaincodePath           string
	chaincodeName           string
	chaincodeUsr            string
	chaincodeQueryRaw       bool
	chaincodeQueryHex       bool
	chaincodeAttributesJSON string
	customIDGenAlg          string
)

var chaincodeCmd = &cobra.Command{
	Use:   chainFuncName,
	Short: fmt.Sprintf("%s specific commands.", chainFuncName),
	Long:  fmt.Sprintf("%s specific commands.", chainFuncName),
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		core.LoggingInit(chainFuncName)
	},
}
