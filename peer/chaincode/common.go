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
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/hyperledger/fabric/core"
	"github.com/hyperledger/fabric/peer/common"
	"github.com/hyperledger/fabric/peer/util"
	pb "github.com/hyperledger/fabric/protos"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/net/context"
)

// chaincodeInvokeOrQuery invokes or queries the chaincode. If successful, the
// INVOKE form prints the transaction ID on STDOUT, and the QUERY form prints
// the query result on STDOUT. A command-line flag (-r, --raw) determines
// whether the query result is output as raw bytes, or as a printable string.
// The printable form is optionally (-x, --hex) a hexadecimal representation
// of the query response. If the query response is NIL, nothing is output.
func chaincodeInvokeOrQuery(cmd *cobra.Command, args []string, invoke bool) (err error) {

	if err = checkChaincodeCmdParams(cmd); err != nil {
		return
	}

	if chaincodeName == "" {
		err = errors.New("Name not given for invoke/query")
		return
	}

	devopsClient, err := common.GetDevopsClient(cmd)
	if err != nil {
		err = fmt.Errorf("Error building %s: %s", chainFuncName, err)
		return
	}
	// Build the spec
	input := &pb.ChaincodeInput{}
	if err = json.Unmarshal([]byte(chaincodeCtorJSON), &input); err != nil {
		err = fmt.Errorf("Chaincode argument error: %s", err)
		return
	}

	var attributes []string
	if err = json.Unmarshal([]byte(chaincodeAttributesJSON), &attributes); err != nil {
		err = fmt.Errorf("Chaincode argument error: %s", err)
		return
	}

	chaincodeLang = strings.ToUpper(chaincodeLang)
	spec := &pb.ChaincodeSpec{Type: pb.ChaincodeSpec_Type(pb.ChaincodeSpec_Type_value[chaincodeLang]),
		ChaincodeID: &pb.ChaincodeID{Name: chaincodeName}, CtorMsg: input, Attributes: attributes}

	// If security is enabled, add client login token
	if core.SecurityEnabled() {
		if chaincodeUsr == common.UndefinedParamValue {
			err = errors.New("Must supply username for chaincode when security is enabled")
			return
		}

		// Retrieve the CLI data storage path
		// Returns /var/openchain/production/client/
		localStore := util.GetCliFilePath()

		// Check if the user is logged in before sending transaction
		if _, err = os.Stat(localStore + "loginToken_" + chaincodeUsr); err == nil {
			logger.Infof("Local user '%s' is already logged in. Retrieving login token.\n", chaincodeUsr)

			// Read in the login token
			token, err := ioutil.ReadFile(localStore + "loginToken_" + chaincodeUsr)
			if err != nil {
				panic(fmt.Errorf("Fatal error when reading client login token: %s\n", err))
			}

			// Add the login token to the chaincodeSpec
			spec.SecureContext = string(token)

			// If privacy is enabled, mark chaincode as confidential
			if viper.GetBool("security.privacy") {
				logger.Info("Set confidentiality level to CONFIDENTIAL.\n")
				spec.ConfidentialityLevel = pb.ConfidentialityLevel_CONFIDENTIAL
			}
		} else {
			// Check if the token is not there and fail
			if os.IsNotExist(err) {
				err = fmt.Errorf("User '%s' not logged in. Use the 'login' command to obtain a security token.", chaincodeUsr)
				return
			}
			// Unexpected error
			panic(fmt.Errorf("Fatal error when checking for client login token: %s\n", err))
		}
	} else {
		if chaincodeUsr != common.UndefinedParamValue {
			logger.Warning("Username supplied but security is disabled.")
		}
		if viper.GetBool("security.privacy") {
			panic(errors.New("Privacy cannot be enabled as requested because security is disabled"))
		}
	}

	// Build the ChaincodeInvocationSpec message
	invocation := &pb.ChaincodeInvocationSpec{ChaincodeSpec: spec}
	if customIDGenAlg != common.UndefinedParamValue {
		invocation.IdGenerationAlg = customIDGenAlg
	}

	var resp *pb.Response
	if invoke {
		resp, err = devopsClient.Invoke(context.Background(), invocation)
	} else {
		resp, err = devopsClient.Query(context.Background(), invocation)
	}

	if err != nil {
		if invoke {
			err = fmt.Errorf("Error invoking %s: %s\n", chainFuncName, err)
		} else {
			err = fmt.Errorf("Error querying %s: %s\n", chainFuncName, err)
		}
		return
	}
	if invoke {
		transactionID := string(resp.Msg)
		logger.Infof("Successfully invoked transaction: %s(%s)", invocation, transactionID)
		fmt.Println(transactionID)
	} else {
		logger.Infof("Successfully queried transaction: %s", invocation)
		if resp != nil {
			if chaincodeQueryRaw {
				if chaincodeQueryHex {
					err = errors.New("Options --raw (-r) and --hex (-x) are not compatible\n")
					return
				}
				os.Stdout.Write(resp.Msg)
			} else {
				if chaincodeQueryHex {
					fmt.Printf("%x\n", resp.Msg)
				} else {
					fmt.Println(string(resp.Msg))
				}
			}
		}
	}
	return nil
}

func checkChaincodeCmdParams(cmd *cobra.Command) (err error) {

	if chaincodeName == common.UndefinedParamValue {
		if chaincodePath == common.UndefinedParamValue {
			err = fmt.Errorf("Must supply value for %s path parameter.\n", chainFuncName)
			return
		}
	}

	// Check that non-empty chaincode parameters contain only Function and
	// Args keys. Type checking is done later when the JSON is actually
	// unmarshaled into a pb.ChaincodeInput. To better understand what's going
	// on here with JSON parsing see http://blog.golang.org/json-and-go -
	// Generic JSON with interface{}
	if chaincodeCtorJSON != "{}" {
		var f interface{}
		err = json.Unmarshal([]byte(chaincodeCtorJSON), &f)
		if err != nil {
			err = fmt.Errorf("Chaincode argument error: %s", err)
			return
		}
		m := f.(map[string]interface{})
		if len(m) != 2 {
			err = fmt.Errorf("Non-empty JSON chaincode parameters must contain exactly 2 keys - 'Function' and 'Args'")
			return
		}
		for k := range m {
			switch strings.ToLower(k) {
			case "function":
			case "args":
			default:
				err = fmt.Errorf("Illegal chaincode key '%s' - must be either 'Function' or 'Args'", k)
				return
			}
		}
	} else {
		err = errors.New("Empty JSON chaincode parameters must contain exactly 2 keys - 'Function' and 'Args'")
		return
	}

	if chaincodeAttributesJSON != "[]" {
		var f interface{}
		err = json.Unmarshal([]byte(chaincodeAttributesJSON), &f)
		if err != nil {
			err = fmt.Errorf("Chaincode argument error: %s", err)
			return
		}
	}

	return
}
