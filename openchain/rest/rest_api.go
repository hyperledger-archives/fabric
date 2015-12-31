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

package rest

import (
	"encoding/json"
	"fmt"
	"google/protobuf"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"

	"golang.org/x/net/context"

	"github.com/gocraft/web"
	"github.com/golang/protobuf/jsonpb"
	"github.com/op/go-logging"
	"github.com/spf13/viper"

	oc "github.com/openblockchain/obc-peer/openchain"
	"github.com/openblockchain/obc-peer/openchain/peer"
	pb "github.com/openblockchain/obc-peer/protos"
)

var logger = logging.MustGetLogger("rest")

// serverOpenchain is a variable that holds the pointer to the
// underlying ServerOpenchain object. serverDevops is a variable that holds
// the pointer to the underlying Devops object. This is necessary due to
// how the gocraft/web package implements context initialization.
var serverOpenchain *oc.ServerOpenchain
var serverDevops *oc.Devops

// ServerOpenchainREST defines the Openchain REST service object. It exposes
// the methods available on the ServerOpenchain service and the Devops service
// through a REST API.
type ServerOpenchainREST struct {
	server *oc.ServerOpenchain
	devops *oc.Devops
}

// SetOpenchainServer is a middleware function that sets the pointer to the
// underlying ServerOpenchain object and the undeflying Devops object.
func (s *ServerOpenchainREST) SetOpenchainServer(rw web.ResponseWriter, req *web.Request, next web.NextMiddlewareFunc) {
	s.server = serverOpenchain
	s.devops = serverDevops

	next(rw, req)
}

// SetResponseType is a middleware function that sets the appropriate response
// headers. Currently, it is setting the "Content-Type" to "application/json" as
// well as the necessary headers in order to enable CORS for Swagger usage.
func (s *ServerOpenchainREST) SetResponseType(rw web.ResponseWriter, req *web.Request, next web.NextMiddlewareFunc) {
	rw.Header().Set("Content-Type", "application/json")

	// Enable CORS
	rw.Header().Set("Access-Control-Allow-Origin", "*")
	rw.Header().Set("Access-Control-Allow-Headers", "accept, content-type")

	next(rw, req)
}

// getRESTFilePath is a helper function to retrieve the local storage directory
// of client login tokens.
func getRESTFilePath() string {
	localStore := viper.GetString("peer.fileSystemPath")
	if !strings.HasSuffix(localStore, "/") {
		localStore = localStore + "/"
	}
	localStore = localStore + "client/"
	return localStore
}

// Register confirms the enrollmentID and secret password of the client with the
// CA and stores the enrollment certificate and key in the Devops server.
func (s *ServerOpenchainREST) Register(rw web.ResponseWriter, req *web.Request) {
	logger.Info("REST client login...")

	// Decode the incoming JSON payload
	var loginSpec pb.Secret
	err := jsonpb.Unmarshal(req.Body, &loginSpec)

	// Check for proper JSON syntax
	if err != nil {
		// Unmarshall returns a " character around unrecognized fields in the case
		// of a schema validation failure. These must be replaced with a ' character.
		// Otherwise, the returned JSON is invalid.
		errVal := strings.Replace(err.Error(), "\"", "'", -1)

		// Client must supply payload
		if err == io.EOF {
			rw.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(rw, "{\"Error\": \"Payload must contain object Secret with enrollId and enrollSecret fields.\"}")
			logger.Error("{\"Error\": \"Payload must contain object Secret with enrollId and enrollSecret fields.\"}")
		} else {
			rw.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(rw, "{\"Error\": \"%s\"}", errVal)
			logger.Error(fmt.Sprintf("{\"Error\": \"%s\"}", errVal))
		}

		return
	}

	// Check that the enrollId and enrollSecret are not left blank.
	if (loginSpec.EnrollId == "") || (loginSpec.EnrollSecret == "") {
		rw.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(rw, "{\"Error\": \"enrollId and enrollSecret may not be blank.\"}")
		logger.Error("{\"Error\": \"enrollId and enrollSecret may not be blank.\"}")

		return
	}

	// Retrieve the REST data storage path
	// Returns /var/openchain/production/client/
	localStore := getRESTFilePath()
	logger.Info("Local data store for client loginToken: %s", localStore)

	// If the user is already logged in, return
	if _, err := os.Stat(localStore + "loginToken_" + loginSpec.EnrollId); err == nil {
		rw.WriteHeader(http.StatusOK)
		fmt.Fprintf(rw, "{\"OK\": \"User %s is already logged in.\"}", loginSpec.EnrollId)
		logger.Info("User '%s' is already logged in.\n", loginSpec.EnrollId)

		return
	}

	// User is not logged in, proceed with login
	logger.Info("Logging in user '%s' on REST interface...\n", loginSpec.EnrollId)

	// Get a devopsClient to perform the login
	clientConn, err := peer.NewPeerClientConnection()
	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(rw, "{\"Error\": \"Error trying to connect to local peer: %s\"}", err)
		logger.Error(fmt.Sprintf("Error trying to connect to local peer: %s", err))

		return
	}
	devopsClient := pb.NewDevopsClient(clientConn)

	// Perform the login
	loginResult, err := devopsClient.Login(context.Background(), &loginSpec)

	// Check if login is successful
	if loginResult.Status == pb.Response_SUCCESS {
		// If /var/openchain/production/client/ directory does not exist, create it
		if _, err := os.Stat(localStore); err != nil {
			if os.IsNotExist(err) {
				// Directory does not exist, create it
				if err := os.Mkdir(localStore, 0755); err != nil {
					rw.WriteHeader(http.StatusInternalServerError)
					fmt.Fprintf(rw, "{\"Error\": \"Fatal error -- %s\"}", err)
					panic(fmt.Errorf("Fatal error when creating %s directory: %s\n", localStore, err))
				}
			} else {
				// Unexpected error
				rw.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintf(rw, "{\"Error\": \"Fatal error -- %s\"}", err)
				panic(fmt.Errorf("Fatal error on os.Stat of %s directory: %s\n", localStore, err))
			}
		}

		// Store client security context into a file
		logger.Info("Storing login token for user '%s'.\n", loginSpec.EnrollId)
		err = ioutil.WriteFile(localStore+"loginToken_"+loginSpec.EnrollId, []byte(loginSpec.EnrollId), 0755)
		if err != nil {
			rw.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(rw, "{\"Error\": \"Fatal error -- %s\"}", err)
			panic(fmt.Errorf("Fatal error when storing client login token: %s\n", err))
		}

		rw.WriteHeader(http.StatusOK)
		fmt.Fprintf(rw, "{\"OK\": \"Login successful for user '%s'.\"}", loginSpec.EnrollId)
		logger.Info("Login successful for user '%s'.\n", loginSpec.EnrollId)
	} else {
		loginErr := strings.Replace(string(loginResult.Msg), "\"", "'", -1)

		rw.WriteHeader(http.StatusUnauthorized)
		fmt.Fprintf(rw, "{\"Error\": \"%s\"}", loginErr)
		logger.Error(fmt.Sprintf("Error on client login: %s", loginErr))
	}

	return
}

// GetEnrollmentID checks whether a given user has already registered with the
// Devops server.
func (s *ServerOpenchainREST) GetEnrollmentID(rw web.ResponseWriter, req *web.Request) {
	// Parse out the user enrollment ID
	enrollmentID := req.PathParams["id"]

	// Retrieve the REST data storage path
	// Returns /var/openchain/production/client/
	localStore := getRESTFilePath()

	// If the user is already logged in, return OK. Otherwise return error.
	if _, err := os.Stat(localStore + "loginToken_" + enrollmentID); err == nil {
		rw.WriteHeader(http.StatusOK)
		fmt.Fprintf(rw, "{\"OK\": \"User %s is already logged in.\"}", enrollmentID)
		logger.Info("User '%s' is already logged in.\n", enrollmentID)
	} else {
		rw.WriteHeader(http.StatusUnauthorized)
		fmt.Fprintf(rw, "{\"Error\": \"User %s must log in.\"}", enrollmentID)
		logger.Info("User '%s' must log in.\n", enrollmentID)
	}

	return
}

// DeleteEnrollmentID removes the login token of the specified user from the
// Devops server. Once the login token is removed, the specified user will no
// longer be able to transact without logging in again. On the REST interface,
// this method may be used as a means of logging out an active client.
func (s *ServerOpenchainREST) DeleteEnrollmentID(rw web.ResponseWriter, req *web.Request) {
	// Parse out the user enrollment ID
	enrollmentID := req.PathParams["id"]

	// Retrieve the REST data storage path
	// Returns /var/openchain/production/client/
	localStore := getRESTFilePath()

	// Construct the path to the login token and to the directory containing the
	// cert and key.
	// /var/openchain/production/client/loginToken_username
	loginTok := localStore + "loginToken_" + enrollmentID
	// /var/openchain/production/crypto/client/username
	cryptoDir := viper.GetString("peer.fileSystemPath") + "/crypto/client/" + enrollmentID

	// Stat both paths to determine if the user is currently logged in
	_, err1 := os.Stat(loginTok)
	_, err2 := os.Stat(cryptoDir)

	// If the user is not logged in, nothing to delete. Return OK.
	if os.IsNotExist(err1) && os.IsNotExist(err2) {
		rw.WriteHeader(http.StatusOK)
		fmt.Fprintf(rw, "{\"OK\": \"User %s is not logged in.\"}", enrollmentID)
		logger.Info("User '%s' is not logged in.\n", enrollmentID)

		return
	}

	// The user is logged in, delete the user's login token
	if err := os.RemoveAll(loginTok); err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(rw, "{\"Error\": \"Error trying to delete login token for user %s: %s\"}", enrollmentID, err)
		logger.Error(fmt.Sprintf("{\"Error\": \"Error trying to delete login token for user %s: %s\"}", enrollmentID, err))

		return
	}

	// The user is logged in, delete the user's cert and key directory
	if err := os.RemoveAll(cryptoDir); err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(rw, "{\"Error\": \"Error trying to delete login directory for user %s: %s\"}", enrollmentID, err)
		logger.Error(fmt.Sprintf("{\"Error\": \"Error trying to delete login directory for user %s: %s\"}", enrollmentID, err))

		return
	}

	rw.WriteHeader(http.StatusOK)
	fmt.Fprintf(rw, "{\"OK\": \"Deleted login token and directory for user %s.\"}", enrollmentID)
	logger.Info("Deleted login token and directory for user %s.\n", enrollmentID)

	return
}

// GetBlockchainInfo returns information about the blockchain ledger such as
// height, current block hash, and previous block hash.
func (s *ServerOpenchainREST) GetBlockchainInfo(rw web.ResponseWriter, req *web.Request) {
	info, err := s.server.GetBlockchainInfo(context.Background(), &google_protobuf.Empty{})

	encoder := json.NewEncoder(rw)

	// Check for error
	if err != nil {
		// Failure
		rw.WriteHeader(400)
		fmt.Fprintf(rw, "{\"Error\": \"%s\"}", err)
	} else {
		// Success
		rw.WriteHeader(200)
		encoder.Encode(info)
	}
}

// GetBlockByNumber returns the data contained within a specific block in the
// blockchain. The genesis block is block zero.
func (s *ServerOpenchainREST) GetBlockByNumber(rw web.ResponseWriter, req *web.Request) {
	// Parse out the Block id
	blockNumber, err := strconv.ParseUint(req.PathParams["id"], 10, 64)

	// Check for proper Block id syntax
	if err != nil {
		// Failure
		rw.WriteHeader(400)
		fmt.Fprintf(rw, "{\"Error\": \"Block id must be an integer (uint64).\"}")
	} else {
		// Retrieve Block from blockchain
		block, err := s.server.GetBlockByNumber(context.Background(), &pb.BlockNumber{Number: blockNumber})

		encoder := json.NewEncoder(rw)

		// Check for error
		if err != nil {
			// Failure
			rw.WriteHeader(400)
			fmt.Fprintf(rw, "{\"Error\": \"%s\"}", err)
		} else {
			// Success
			rw.WriteHeader(200)
			encoder.Encode(block)
		}
	}
}

// GetState returns the value for the specified chaincodeID and key.
func (s *ServerOpenchainREST) GetState(rw web.ResponseWriter, req *web.Request) {
	// Parse out the chaincodeId and key.
	chaincodeID := req.PathParams["chaincodeId"]
	key := req.PathParams["key"]

	// Retrieve Chaincode state.
	state, err := s.server.GetState(context.Background(), chaincodeID, key)

	// Check for error
	if err != nil {
		// Failure
		rw.WriteHeader(400)
		fmt.Fprintf(rw, "{\"Error\": \"%s\"}", err)
	} else {
		// Success
		if state == nil { // no match on ChaincodeID and key
			rw.WriteHeader(200)
			fmt.Fprintf(rw, "{\"State\": \"null\"}")
		} else {
			rw.WriteHeader(200)
			fmt.Fprintf(rw, "{\"State\": \"%s\"}", state)
		}
	}
}

// Build creates the docker container that holds the Chaincode and all required
// entities.
func (s *ServerOpenchainREST) Build(rw web.ResponseWriter, req *web.Request) {
	// Decode the incoming JSON payload
	var spec pb.ChaincodeSpec
	err := jsonpb.Unmarshal(req.Body, &spec)

	// Check for proper JSON syntax
	if err != nil {
		// Unmarshall returns a " character around unrecognized fields in the case
		// of a schema validation failure. These must be replaced with a ' character
		// as otherwise the returned JSON is invalid.
		errVal := strings.Replace(err.Error(), "\"", "'", -1)

		rw.WriteHeader(http.StatusBadRequest)

		// Client must supply payload
		if err == io.EOF {
			fmt.Fprintf(rw, "{\"Error\": \"Must provide ChaincodeSpec.\"}")
		} else {
			fmt.Fprintf(rw, "{\"Error\": \"%s\"}", errVal)
		}
		return
	}

	// Check for incomplete ChaincodeSpec
	if spec.ChaincodeID.Path.Url == "" {
		fmt.Fprintf(rw, "{\"Error\": \"Must specify Chaincode URL path.\"}")
		return
	}
	if spec.ChaincodeID.Path.Version == "" {
		fmt.Fprintf(rw, "{\"Error\": \"Must specify Chaincode version.\"}")
		return
	}

	// Build the ChaincodeSpec
	_, err = s.devops.Build(context.Background(), &spec)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(rw, "{\"Error\": \"%s\"}", err)
		return
	}

	rw.WriteHeader(http.StatusOK)
	fmt.Fprintf(rw, "{\"OK\": \"Successfully built chainCode.\"}")
}

// Deploy first builds the chaincode package and subsequently deploys it to the
// blockchain.
func (s *ServerOpenchainREST) Deploy(rw web.ResponseWriter, req *web.Request) {
	logger.Info("REST deploying chaincode...")

	// Decode the incoming JSON payload
	var spec pb.ChaincodeSpec
	err := jsonpb.Unmarshal(req.Body, &spec)

	// Check for proper JSON syntax
	if err != nil {
		// Unmarshall returns a " character around unrecognized fields in the case
		// of a schema validation failure. These must be replaced with a ' character.
		// Otherwise, the returned JSON is invalid.
		errVal := strings.Replace(err.Error(), "\"", "'", -1)

		// Client must supply payload
		if err == io.EOF {
			rw.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(rw, "{\"Error\": \"Payload must contain a ChaincodeSpec.\"}")
			logger.Error("{\"Error\": \"Payload must contain a ChaincodeSpec.\"}")
		} else {
			rw.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(rw, "{\"Error\": \"%s\"}", errVal)
			logger.Error(fmt.Sprintf("{\"Error\": \"%s\"}", errVal))
		}

		return
	}

	// Check that the ChaincodeID is not left blank.
	if spec.ChaincodeID.Path == nil {
		rw.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(rw, "{\"Error\": \"Payload must contain a ChaincodeID.\"}")
		logger.Error("{\"Error\": \"Payload must contain a ChaincodeID.\"}")

		return
	}

	// Check that the Chaincode URL path and version are not left blank.
	if (spec.ChaincodeID.Path.Url == "") || (spec.ChaincodeID.Path.Version == "") {
		rw.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(rw, "{\"Error\": \"Chaincode URL path and version may not be blank.\"}")
		logger.Error("{\"Error\": \"Chaincode URL path and version  may not be blank.\"}")

		return
	}

	// If security is enabled, add client login token
	if viper.GetBool("security.enabled") {
		chaincodeUsr := spec.SecureContext
		if chaincodeUsr == "" {
			rw.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(rw, "{\"Error\": \"Must supply username for chaincode when security is enabled.\"}")
			logger.Error("{\"Error\": \"Must supply username for chaincode when security is enabled.\"}")

			return
		}

		// Retrieve the REST data storage path
		// Returns /var/openchain/production/client/
		localStore := getRESTFilePath()

		// Check if the user is logged in before sending transaction
		if _, err := os.Stat(localStore + "loginToken_" + chaincodeUsr); err == nil {
			logger.Info("Local user '%s' is already logged in. Retrieving login token.\n", chaincodeUsr)

			// Read in the login token
			token, err := ioutil.ReadFile(localStore + "loginToken_" + chaincodeUsr)
			if err != nil {
				rw.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintf(rw, "{\"Error\": \"Fatal error -- %s\"}", err)
				panic(fmt.Errorf("Fatal error when reading client login token: %s\n", err))
			}

			// Add the login token to the chaincodeSpec
			spec.SecureContext = string(token)
		} else {
			// Check if the token is not there and fail
			if os.IsNotExist(err) {
				rw.WriteHeader(http.StatusUnauthorized)
				fmt.Fprintf(rw, "{\"Error\": \"User not logged in. Use the '/registrar' endpoint to obtain a security token.\"}")
				logger.Error("{\"Error\": \"User not logged in. Use the '/registrar' endpoint to obtain a security token.\"}")

				return
			}
			// Unexpected error
			rw.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(rw, "{\"Error\": \"Fatal error -- %s\"}", err)
			panic(fmt.Errorf("Fatal error when checking for client login token: %s\n", err))
		}
	}

	// Deploy the ChaincodeSpec
	_, err = s.devops.Deploy(context.Background(), &spec)
	if err != nil {
		// Replace " characters with '
		errVal := strings.Replace(err.Error(), "\"", "'", -1)

		rw.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(rw, "{\"Error\": \"%s\"}", errVal)
		logger.Error(fmt.Sprintf("{\"Error\": \"Deploying Chaincode -- %s\"}", errVal))

		return
	}

	rw.WriteHeader(http.StatusOK)
	fmt.Fprintf(rw, "{\"OK\": \"Successfully deployed chainCode.\"}")
	logger.Info("Successfuly deployed chainCode.\n")
}

// Invoke executes a specified function within a target Chaincode.
func (s *ServerOpenchainREST) Invoke(rw web.ResponseWriter, req *web.Request) {
	logger.Info("REST invoking chaincode...")

	// Decode the incoming JSON payload
	var spec pb.ChaincodeInvocationSpec
	err := jsonpb.Unmarshal(req.Body, &spec)

	// Check for proper JSON syntax
	if err != nil {
		// Unmarshall returns a " character around unrecognized fields in the case
		// of a schema validation failure. These must be replaced with a ' character.
		// Otherwise, the returned JSON is invalid.
		errVal := strings.Replace(err.Error(), "\"", "'", -1)

		// Client must supply payload
		if err == io.EOF {
			rw.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(rw, "{\"Error\": \"Payload must contain a ChaincodeInvocationSpec.\"}")
			logger.Error("{\"Error\": \"Payload must contain a ChaincodeInvocationSpec.\"}")
		} else {
			rw.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(rw, "{\"Error\": \"%s\"}", errVal)
			logger.Error(fmt.Sprintf("{\"Error\": \"%s\"}", errVal))
		}

		return
	}

	// Check that the ChaincodeSpec is not left blank.
	if spec.ChaincodeSpec == nil {
		rw.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(rw, "{\"Error\": \"Payload must contain a ChaincodeSpec.\"}")
		logger.Error("{\"Error\": \"Payload must contain a ChaincodeSpec.\"}")

		return
	}

	// Check that the ChaincodeID is not left blank.
	if spec.ChaincodeSpec.ChaincodeID == nil {
		rw.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(rw, "{\"Error\": \"Payload must contain a ChaincodeID.\"}")
		logger.Error("{\"Error\": \"Payload must contain a ChaincodeID.\"}")

		return
	}

	// NOTE - Old approach. Invoke transaction should now specify Name returned on the Deploy Transaction
	//if (spec.ChaincodeSpec.ChaincodeID.Url == "") || (spec.ChaincodeSpec.ChaincodeID.Version == "") {

	// Check that the Chaincode Name is not blank.
	if spec.ChaincodeSpec.ChaincodeID.Name == "" {
		rw.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(rw, "{\"Error\": \"Chaincode URL path and version may not be blank.\"}")
		logger.Error("{\"Error\": \"Chaincode URL path and version  may not be blank.\"}")

		return
	}

	// Check that the CtorMsg is not left blank.
	if (spec.ChaincodeSpec.CtorMsg == nil) || (spec.ChaincodeSpec.CtorMsg.Function == "") {
		rw.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(rw, "{\"Error\": \"Payload must contain a CtorMsg with a Chaincode function name.\"}")
		logger.Error("{\"Error\": \"Payload must contain a CtorMsg with a Chaincode function name.\"}")

		return
	}

	// If security is enabled, add client login token
	if viper.GetBool("security.enabled") {
		chaincodeUsr := spec.ChaincodeSpec.SecureContext
		if chaincodeUsr == "" {
			rw.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(rw, "{\"Error\": \"Must supply username for chaincode when security is enabled.\"}")
			logger.Error("{\"Error\": \"Must supply username for chaincode when security is enabled.\"}")

			return
		}

		// Retrieve the REST data storage path
		// Returns /var/openchain/production/client/
		localStore := getRESTFilePath()

		// Check if the user is logged in before sending transaction
		if _, err := os.Stat(localStore + "loginToken_" + chaincodeUsr); err == nil {
			logger.Info("Local user '%s' is already logged in. Retrieving login token.\n", chaincodeUsr)

			// Read in the login token
			token, err := ioutil.ReadFile(localStore + "loginToken_" + chaincodeUsr)
			if err != nil {
				rw.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintf(rw, "{\"Error\": \"Fatal error -- %s\"}", err)
				panic(fmt.Errorf("Fatal error when reading client login token: %s\n", err))
			}

			// Add the login token to the chaincodeSpec
			spec.ChaincodeSpec.SecureContext = string(token)
		} else {
			// Check if the token is not there and fail
			if os.IsNotExist(err) {
				rw.WriteHeader(http.StatusUnauthorized)
				fmt.Fprintf(rw, "{\"Error\": \"User not logged in. Use the '/registrar' endpoint to obtain a security token.\"}")
				logger.Error("{\"Error\": \"User not logged in. Use the '/registrar' endpoint to obtain a security token.\"}")

				return
			}
			// Unexpected error
			rw.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(rw, "{\"Error\": \"Fatal error -- %s\"}", err)
			panic(fmt.Errorf("Fatal error when checking for client login token: %s\n", err))
		}
	}

	// Invoke the chainCode
	_, err = s.devops.Invoke(context.Background(), &spec)
	if err != nil {
		// Replace " characters with '
		errVal := strings.Replace(err.Error(), "\"", "'", -1)

		rw.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(rw, "{\"Error\": \"%s\"}", errVal)
		logger.Error(fmt.Sprintf("{\"Error\": \"Invoking Chaincode -- %s\"}", errVal))

		return
	}

	rw.WriteHeader(http.StatusOK)
	fmt.Fprintf(rw, "{\"OK\": \"Successfully invoked chainCode.\"}")
	logger.Info("Successfuly invoked chainCode.\n")
}

// Query performs the requested query on the target Chaincode.
func (s *ServerOpenchainREST) Query(rw web.ResponseWriter, req *web.Request) {
	logger.Info("REST querying chaincode...")

	// Decode the incoming JSON payload
	var spec pb.ChaincodeInvocationSpec
	err := jsonpb.Unmarshal(req.Body, &spec)

	// Check for proper JSON syntax
	if err != nil {
		// Unmarshall returns a " character around unrecognized fields in the case
		// of a schema validation failure. These must be replaced with a ' character.
		// Otherwise, the returned JSON is invalid.
		errVal := strings.Replace(err.Error(), "\"", "'", -1)

		// Client must supply payload
		if err == io.EOF {
			rw.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(rw, "{\"Error\": \"Payload must contain a ChaincodeInvocationSpec.\"}")
			logger.Error("{\"Error\": \"Payload must contain a ChaincodeInvocationSpec.\"}")
		} else {
			rw.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(rw, "{\"Error\": \"%s\"}", errVal)
			logger.Error(fmt.Sprintf("{\"Error\": \"%s\"}", errVal))
		}

		return
	}

	// Check that the ChaincodeSpec is not left blank.
	if spec.ChaincodeSpec == nil {
		rw.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(rw, "{\"Error\": \"Payload must contain a ChaincodeSpec.\"}")
		logger.Error("{\"Error\": \"Payload must contain a ChaincodeSpec.\"}")

		return
	}

	// Check that the ChaincodeID is not left blank.
	if spec.ChaincodeSpec.ChaincodeID == nil {
		rw.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(rw, "{\"Error\": \"Payload must contain a ChaincodeID.\"}")
		logger.Error("{\"Error\": \"Payload must contain a ChaincodeID.\"}")

		return
	}

	// NOTE - Old approach. Invoke transaction should now specify Name returned on the Deploy Transaction
	//if (spec.ChaincodeSpec.ChaincodeID.Url == "") || (spec.ChaincodeSpec.ChaincodeID.Version == "") {

	// Check that the Chaincode Name is not blank.
	if spec.ChaincodeSpec.ChaincodeID.Name == "" {
		rw.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(rw, "{\"Error\": \"Chaincode URL path and version may not be blank.\"}")
		logger.Error("{\"Error\": \"Chaincode URL path and version  may not be blank.\"}")

		return
	}

	// Check that the CtorMsg is not left blank.
	if (spec.ChaincodeSpec.CtorMsg == nil) || (spec.ChaincodeSpec.CtorMsg.Function == "") {
		rw.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(rw, "{\"Error\": \"Payload must contain a CtorMsg with a Chaincode function name.\"}")
		logger.Error("{\"Error\": \"Payload must contain a CtorMsg with a Chaincode function name.\"}")

		return
	}

	// If security is enabled, add client login token
	if viper.GetBool("security.enabled") {
		chaincodeUsr := spec.ChaincodeSpec.SecureContext
		if chaincodeUsr == "" {
			rw.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(rw, "{\"Error\": \"Must supply username for chaincode when security is enabled.\"}")
			logger.Error("{\"Error\": \"Must supply username for chaincode when security is enabled.\"}")

			return
		}

		// Retrieve the REST data storage path
		// Returns /var/openchain/production/client/
		localStore := getRESTFilePath()

		// Check if the user is logged in before sending transaction
		if _, err := os.Stat(localStore + "loginToken_" + chaincodeUsr); err == nil {
			logger.Info("Local user '%s' is already logged in. Retrieving login token.\n", chaincodeUsr)

			// Read in the login token
			token, err := ioutil.ReadFile(localStore + "loginToken_" + chaincodeUsr)
			if err != nil {
				rw.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintf(rw, "{\"Error\": \"Fatal error -- %s\"}", err)
				panic(fmt.Errorf("Fatal error when reading client login token: %s\n", err))
			}

			// Add the login token to the chaincodeSpec
			spec.ChaincodeSpec.SecureContext = string(token)
		} else {
			// Check if the token is not there and fail
			if os.IsNotExist(err) {
				rw.WriteHeader(http.StatusUnauthorized)
				fmt.Fprintf(rw, "{\"Error\": \"User not logged in. Use the '/registrar' endpoint to obtain a security token.\"}")
				logger.Error("{\"Error\": \"User not logged in. Use the '/registrar' endpoint to obtain a security token.\"}")

				return
			}
			// Unexpected error
			rw.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(rw, "{\"Error\": \"Fatal error -- %s\"}", err)
			panic(fmt.Errorf("Fatal error when checking for client login token: %s\n", err))
		}
	}

	// Query the chainCode
	resp, err := s.devops.Query(context.Background(), &spec)
	if err != nil {
		// Replace " characters with '
		errVal := strings.Replace(err.Error(), "\"", "'", -1)

		rw.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(rw, "{\"Error\": \"%s\"}", errVal)
		logger.Error(fmt.Sprintf("{\"Error\": \"Querying Chaincode -- %s\"}", errVal))

		return
	}

	// Replace " characters with '
	respVal := strings.Replace(string(resp.Msg), "\"", "'", -1)

	rw.WriteHeader(http.StatusOK)
	fmt.Fprintf(rw, "{\"OK\": %s}", respVal)
	logger.Info("Successfuly invoked chainCode.\n")
}

// NotFound returns a custom landing page when a given openchain end point
// had not been defined.
func (s *ServerOpenchainREST) NotFound(rw web.ResponseWriter, r *web.Request) {
	rw.WriteHeader(http.StatusNotFound)
	fmt.Fprintf(rw, "{\"Error\": \"Openchain endpoint not found.\"}")
}

// StartOpenchainRESTServer initializes the REST service and adds the required
// middleware and routes.
func StartOpenchainRESTServer(server *oc.ServerOpenchain, devops *oc.Devops) {
	// Initialize the REST service object
	logger.Info("Initializing the REST service...")
	router := web.New(ServerOpenchainREST{})

	// Record the pointer to the underlying ServerOpenchain and Devops objects.
	serverOpenchain = server
	serverDevops = devops

	// Add middleware
	router.Middleware((*ServerOpenchainREST).SetOpenchainServer)
	router.Middleware((*ServerOpenchainREST).SetResponseType)

	// Add routes
	router.Post("/registrar", (*ServerOpenchainREST).Register)
	router.Get("/registrar/:id", (*ServerOpenchainREST).GetEnrollmentID)
	router.Delete("/registrar/:id", (*ServerOpenchainREST).DeleteEnrollmentID)

	router.Get("/chain", (*ServerOpenchainREST).GetBlockchainInfo)
	router.Get("/chain/blocks/:id", (*ServerOpenchainREST).GetBlockByNumber)

	router.Get("/state/:chaincodeId/:key", (*ServerOpenchainREST).GetState)

	router.Post("/devops/build", (*ServerOpenchainREST).Build)
	router.Post("/devops/deploy", (*ServerOpenchainREST).Deploy)
	router.Post("/devops/invoke", (*ServerOpenchainREST).Invoke)
	router.Post("/devops/query", (*ServerOpenchainREST).Query)

	// Add not found page
	router.NotFound((*ServerOpenchainREST).NotFound)

	// Start server
	err := http.ListenAndServe(viper.GetString("rest.address"), router)
	if err != nil {
		logger.Error(fmt.Sprintf("ListenAndServe: %s", err))
	}
}
