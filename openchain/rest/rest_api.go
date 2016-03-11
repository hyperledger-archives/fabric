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
	"net/url"
	"os"
	"strconv"
	"strings"

	"golang.org/x/net/context"

	"github.com/gocraft/web"
	"github.com/golang/protobuf/jsonpb"
	"github.com/op/go-logging"
	"github.com/spf13/viper"

	oc "github.com/openblockchain/obc-peer/openchain"
	"github.com/openblockchain/obc-peer/openchain/chaincode"
	"github.com/openblockchain/obc-peer/openchain/crypto"
	"github.com/openblockchain/obc-peer/openchain/crypto/utils"
	pb "github.com/openblockchain/obc-peer/protos"
)

var restLogger = logging.MustGetLogger("rest")

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

// restResult defines the structure of the REST interface JSON response.
type restResult struct {
	OK    string `json:",omitempty"`
	Error string `json:",omitempty"`
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
	restLogger.Info("REST client login...")

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
			restLogger.Error("{\"Error\": \"Payload must contain object Secret with enrollId and enrollSecret fields.\"}")
		} else {
			rw.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(rw, "{\"Error\": \"%s\"}", errVal)
			restLogger.Error(fmt.Sprintf("{\"Error\": \"%s\"}", errVal))
		}

		return
	}

	// Check that the enrollId and enrollSecret are not left blank.
	if (loginSpec.EnrollId == "") || (loginSpec.EnrollSecret == "") {
		rw.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(rw, "{\"Error\": \"enrollId and enrollSecret may not be blank.\"}")
		restLogger.Error("{\"Error\": \"enrollId and enrollSecret may not be blank.\"}")

		return
	}

	// Retrieve the REST data storage path
	// Returns /var/openchain/production/client/
	localStore := getRESTFilePath()
	restLogger.Info("Local data store for client loginToken: %s", localStore)

	// If the user is already logged in, return
	if _, err := os.Stat(localStore + "loginToken_" + loginSpec.EnrollId); err == nil {
		rw.WriteHeader(http.StatusOK)
		fmt.Fprintf(rw, "{\"OK\": \"User %s is already logged in.\"}", loginSpec.EnrollId)
		restLogger.Info("User '%s' is already logged in.\n", loginSpec.EnrollId)

		return
	}

	// User is not logged in, proceed with login
	restLogger.Info("Logging in user '%s' on REST interface...\n", loginSpec.EnrollId)

	loginResult, err := s.devops.Login(context.Background(), &loginSpec)

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
		restLogger.Info("Storing login token for user '%s'.\n", loginSpec.EnrollId)
		err = ioutil.WriteFile(localStore+"loginToken_"+loginSpec.EnrollId, []byte(loginSpec.EnrollId), 0755)
		if err != nil {
			rw.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(rw, "{\"Error\": \"Fatal error -- %s\"}", err)
			panic(fmt.Errorf("Fatal error when storing client login token: %s\n", err))
		}

		rw.WriteHeader(http.StatusOK)
		fmt.Fprintf(rw, "{\"OK\": \"Login successful for user '%s'.\"}", loginSpec.EnrollId)
		restLogger.Info("Login successful for user '%s'.\n", loginSpec.EnrollId)
	} else {
		loginErr := strings.Replace(string(loginResult.Msg), "\"", "'", -1)

		rw.WriteHeader(http.StatusUnauthorized)
		fmt.Fprintf(rw, "{\"Error\": \"%s\"}", loginErr)
		restLogger.Error(fmt.Sprintf("Error on client login: %s", loginErr))
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
		restLogger.Info("User '%s' is already logged in.\n", enrollmentID)
	} else {
		rw.WriteHeader(http.StatusUnauthorized)
		fmt.Fprintf(rw, "{\"Error\": \"User %s must log in.\"}", enrollmentID)
		restLogger.Info("User '%s' must log in.\n", enrollmentID)
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
		restLogger.Info("User '%s' is not logged in.\n", enrollmentID)

		return
	}

	// The user is logged in, delete the user's login token
	if err := os.RemoveAll(loginTok); err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(rw, "{\"Error\": \"Error trying to delete login token for user %s: %s\"}", enrollmentID, err)
		restLogger.Error(fmt.Sprintf("{\"Error\": \"Error trying to delete login token for user %s: %s\"}", enrollmentID, err))

		return
	}

	// The user is logged in, delete the user's cert and key directory
	if err := os.RemoveAll(cryptoDir); err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(rw, "{\"Error\": \"Error trying to delete login directory for user %s: %s\"}", enrollmentID, err)
		restLogger.Error(fmt.Sprintf("{\"Error\": \"Error trying to delete login directory for user %s: %s\"}", enrollmentID, err))

		return
	}

	rw.WriteHeader(http.StatusOK)
	fmt.Fprintf(rw, "{\"OK\": \"Deleted login token and directory for user %s.\"}", enrollmentID)
	restLogger.Info("Deleted login token and directory for user %s.\n", enrollmentID)

	return
}

// GetEnrollmentCert retrieves the enrollment certificate for a given user.
func (s *ServerOpenchainREST) GetEnrollmentCert(rw web.ResponseWriter, req *web.Request) {
	// Parse out the user enrollment ID
	enrollmentID := req.PathParams["id"]

	if restLogger.IsEnabledFor(logging.DEBUG) {
		restLogger.Debug("REST received enrollment certificate retrieval request for registrationID '%s'", enrollmentID)
	}

	// If security is enabled, initialize the crypto client
	if viper.GetBool("security.enabled") {
		if restLogger.IsEnabledFor(logging.DEBUG) {
			restLogger.Debug("Initializing secure client using context '%s'", enrollmentID)
		}

		// Initialize the security client
		sec, err := crypto.InitClient(enrollmentID, nil)
		if err != nil {
			rw.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(rw, "{\"Error\": \"%s\"}", err)
			restLogger.Error(fmt.Sprintf("{\"Error\": \"%s\"}", err))

			return
		}

		// Obtain the client CertificateHandler
		handler, err := sec.GetEnrollmentCertificateHandler()
		if err != nil {
			rw.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(rw, "{\"Error\": \"%s\"}", err)
			restLogger.Error(fmt.Sprintf("{\"Error\": \"%s\"}", err))

			return
		}

		// Certificate handler can not be hil
		if handler == nil {
			rw.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(rw, "{\"Error\": \"Error retrieving certificate handler.\"}")
			restLogger.Error("{\"Error\": \"Error retrieving certificate handler.\"}")

			return
		}

		// Obtain the DER encoded certificate
		certDER := handler.GetCertificate()

		// Confirm the retrieved enrollment certificate is not nil
		if certDER == nil {
			rw.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(rw, "{\"Error\": \"Enrollment certificate is nil.\"}")
			restLogger.Error("{\"Error\": \"Enrollment certificate is nil.\"}")

			return
		}

		// Confirm the retrieved enrollment certificate has non-zero length
		if len(certDER) == 0 {
			rw.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(rw, "{\"Error\": \"Enrollment certificate length is 0.\"}")
			restLogger.Error("{\"Error\": \"Enrollment certificate length is 0.\"}")

			return
		}

		// Transforms the DER encoded certificate to a PEM encoded certificate
		certPEM := utils.DERCertToPEM(certDER)

		// As the enrollment certificate contains \n characters, url encode it before outputting
		urlEncodedCert := url.QueryEscape(string(certPEM))

		// Close the security client
		crypto.CloseClient(sec)

		rw.WriteHeader(http.StatusOK)
		fmt.Fprintf(rw, "{\"OK\": \"%s\"}", urlEncodedCert)
		if restLogger.IsEnabledFor(logging.DEBUG) {
			restLogger.Debug("Sucessfully retrieved enrollment certificate for secure context '%s'", enrollmentID)
		}
	} else {
		// Security must be enabled to request enrollment certificates
		rw.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(rw, "{\"Error\": \"Security functionality must be enabled before requesting client certificates.\"}")
		restLogger.Error("{\"Error\": \"Security functionality must be enabled before requesting client certificates.\"}")

		return
	}
}

// GetTransactionCert retrieves the transaction certificate(s) for a given user.
func (s *ServerOpenchainREST) GetTransactionCert(rw web.ResponseWriter, req *web.Request) {
	// Parse out the user enrollment ID
	enrollmentID := req.PathParams["id"]

	if restLogger.IsEnabledFor(logging.DEBUG) {
		restLogger.Debug("REST received transaction certificate retrieval request for registrationID '%s'", enrollmentID)
	}

	// Parse out the count query parameter
	req.ParseForm()
	queryParams := req.Form

	// The default number of TCerts to retrieve is 1
	var count uint32 = 1

	// If the query parameter is present, examine the supplied value
	if queryParams["count"] != nil {
		// Convert string to uint. The parse function return the widest type (uint64)
		// Setting base to 32 allows you to subsequently cast the value to uint32
		qParam, err := strconv.ParseUint(queryParams["count"][0], 10, 32)

		// Check for count parameter being a non-negative integer
		if err != nil {
			rw.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(rw, "{\"Error\": \"Count query parameter must be a non-negative integer.\"}")
			restLogger.Error("{\"Error\": \"Count query parameter must be a non-negative integer.\"}")

			return
		}

		// If the query parameter is within the allowed range, record it
		if qParam > 0 && qParam <= 500 {
			count = uint32(qParam)
		}

		// Limit the number of TCerts retrieved to 500
		if qParam > 500 {
			count = 500
		}
	}

	// If security is enabled, initialize the crypto client
	if viper.GetBool("security.enabled") {
		if restLogger.IsEnabledFor(logging.DEBUG) {
			restLogger.Debug("Initializing secure client using context '%s'", enrollmentID)
		}

		// Initialize the security client
		sec, err := crypto.InitClient(enrollmentID, nil)
		if err != nil {
			rw.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(rw, "{\"Error\": \"%s\"}", err)
			restLogger.Error(fmt.Sprintf("{\"Error\": \"%s\"}", err))

			return
		}

		// Obtain the client CertificateHandler
		handler, err := sec.GetTCertificateHandlerNext()
		if err != nil {
			rw.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(rw, "{\"Error\": \"%s\"}", err)
			restLogger.Error(fmt.Sprintf("{\"Error\": \"%s\"}", err))

			return
		}

		// Certificate handler can not be hil
		if handler == nil {
			rw.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(rw, "{\"Error\": \"Error retrieving certificate handler.\"}")
			restLogger.Error("{\"Error\": \"Error retrieving certificate handler.\"}")

			return
		}

		// Retrieve the required number of TCerts
		tcertArray := make([]string, count)
		var i uint32
		for i = 0; i < count; i++ {
			// Obtain the DER encoded certificate
			certDER := handler.GetCertificate()

			// Confirm the retrieved enrollment certificate is not nil
			if certDER == nil {
				rw.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintf(rw, "{\"Error\": \"Transaction certificate is nil.\"}")
				restLogger.Error("{\"Error\": \"Transaction certificate is nil.\"}")

				return
			}

			// Confirm the retrieved enrollment certificate has non-zero length
			if len(certDER) == 0 {
				rw.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintf(rw, "{\"Error\": \"Transaction certificate length is 0.\"}")
				restLogger.Error("{\"Error\": \"Transaction certificate length is 0.\"}")

				return
			}

			// Transforms the DER encoded certificate to a PEM encoded certificate
			certPEM := utils.DERCertToPEM(certDER)

			// As the transaction certificate contains \n characters, url encode it before outputting
			urlEncodedCert := url.QueryEscape(string(certPEM))

			// Add the urlEncodedCert transaction certificate to the certificate array
			tcertArray[i] = urlEncodedCert
		}

		// Close the security client
		crypto.CloseClient(sec)

		// Construct a JSON formatted response
		jsonResponse, err := json.Marshal(tcertArray)
		if err != nil {
			rw.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(rw, "{\"Error\": \"%s\"}", err)
			restLogger.Error(fmt.Sprintf("{\"Error marshalling TCert array\": \"%s\"}", err))

			return
		}

		rw.WriteHeader(http.StatusOK)
		fmt.Fprintf(rw, "{\"OK\": %s}", string(jsonResponse))
		if restLogger.IsEnabledFor(logging.DEBUG) {
			restLogger.Debug("Sucessfully retrieved transaction certificates for secure context '%s'", enrollmentID)
		}
	} else {
		// Security must be enabled to request transaction certificates
		rw.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(rw, "{\"Error\": \"Security functionality must be enabled before requesting client certificates.\"}")
		restLogger.Error("{\"Error\": \"Security functionality must be enabled before requesting client certificates.\"}")

		return
	}
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

		// Check for error
		if err != nil {
			// Failure
			switch err {
			case oc.ErrNotFound:
				rw.WriteHeader(http.StatusNotFound)
			default:
				rw.WriteHeader(http.StatusInternalServerError)
			}
			fmt.Fprintf(rw, "{\"Error\": \"%s\"}", err)
		} else {
			// Success
			rw.WriteHeader(http.StatusOK)
			encoder := json.NewEncoder(rw)
			encoder.Encode(block)
		}
	}
}

// GetTransactionByUUID returns a transaction matching the specified UUID
func (s *ServerOpenchainREST) GetTransactionByUUID(rw web.ResponseWriter, req *web.Request) {
	// Parse out the transaction UUID
	txUUID := req.PathParams["uuid"]

	// Retrieve the transaction matching the UUID
	tx, err := s.server.GetTransactionByUUID(context.Background(), txUUID)

	// Check for Error
	if err != nil {
		switch err {
		case oc.ErrNotFound:
			rw.WriteHeader(http.StatusNotFound)
			fmt.Fprintf(rw, "{\"Error\": \"Transaction %s is not found.\"}", txUUID)
		default:
			rw.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(rw, "{\"Error\": \"Error retrieving transaction %s: %s.\"}", txUUID, err)
			restLogger.Error(fmt.Sprintf("{\"Error\": \"Error retrieving transaction %s: %s.\"}", txUUID, err))
		}
	} else {
		// Return existing transaction
		rw.WriteHeader(http.StatusOK)
		encoder := json.NewEncoder(rw)
		encoder.Encode(tx)
		restLogger.Info(fmt.Sprintf("Successfully retrieved transaction: %s", txUUID))
	}
}

// Deploy first builds the chaincode package and subsequently deploys it to the
// blockchain.
func (s *ServerOpenchainREST) Deploy(rw web.ResponseWriter, req *web.Request) {
	restLogger.Info("REST deploying chaincode...")

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
			restLogger.Error("{\"Error\": \"Payload must contain a ChaincodeSpec.\"}")
		} else {
			rw.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(rw, "{\"Error\": \"%s\"}", errVal)
			restLogger.Error(fmt.Sprintf("{\"Error\": \"%s\"}", errVal))
		}

		return
	}

	// Check that the ChaincodeID is not nil.
	if spec.ChaincodeID == nil {
		rw.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(rw, "{\"Error\": \"Payload must contain a ChaincodeID.\"}")
		restLogger.Error("{\"Error\": \"Payload must contain a ChaincodeID.\"}")

		return
	}

	// If the peer is running in development mode, confirm that the Chaincode name
	// is not left blank. If the peer is running in production mode, confirm that
	// the Chaincode path is not left blank. This is necessary as in development
	// mode, the chaincode is identified by name not by path during the deploy
	// process.
	if viper.GetString("chaincode.mode") == chaincode.DevModeUserRunsChaincode {
		// Check that the Chaincode name is not blank.
		if spec.ChaincodeID.Name == "" {
			rw.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(rw, "{\"Error\": \"Chaincode name may not be blank in development mode.\"}")
			restLogger.Error("{\"Error\": \"Chaincode name may not be blank in development mode.\"}")

			return
		}
	} else {
		// Check that the Chaincode path is not left blank.
		if spec.ChaincodeID.Path == "" {
			rw.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(rw, "{\"Error\": \"Chaincode path may not be blank.\"}")
			restLogger.Error("{\"Error\": \"Chaincode path may not be blank.\"}")

			return
		}
	}

	// If security is enabled, add client login token
	if viper.GetBool("security.enabled") {
		chaincodeUsr := spec.SecureContext
		if chaincodeUsr == "" {
			rw.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(rw, "{\"Error\": \"Must supply username for chaincode when security is enabled.\"}")
			restLogger.Error("{\"Error\": \"Must supply username for chaincode when security is enabled.\"}")

			return
		}

		// Retrieve the REST data storage path
		// Returns /var/openchain/production/client/
		localStore := getRESTFilePath()

		// Check if the user is logged in before sending transaction
		if _, err := os.Stat(localStore + "loginToken_" + chaincodeUsr); err == nil {
			restLogger.Info("Local user '%s' is already logged in. Retrieving login token.\n", chaincodeUsr)

			// Read in the login token
			token, err := ioutil.ReadFile(localStore + "loginToken_" + chaincodeUsr)
			if err != nil {
				rw.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintf(rw, "{\"Error\": \"Fatal error -- %s\"}", err)
				panic(fmt.Errorf("Fatal error when reading client login token: %s\n", err))
			}

			// Add the login token to the chaincodeSpec
			spec.SecureContext = string(token)

			// If privacy is enabled, mark chaincode as confidential
			if viper.GetBool("security.privacy") {
				spec.ConfidentialityLevel = pb.ConfidentialityLevel_CONFIDENTIAL
			}
		} else {
			// Check if the token is not there and fail
			if os.IsNotExist(err) {
				rw.WriteHeader(http.StatusUnauthorized)
				fmt.Fprintf(rw, "{\"Error\": \"User not logged in. Use the '/registrar' endpoint to obtain a security token.\"}")
				restLogger.Error("{\"Error\": \"User not logged in. Use the '/registrar' endpoint to obtain a security token.\"}")

				return
			}
			// Unexpected error
			rw.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(rw, "{\"Error\": \"Fatal error -- %s\"}", err)
			panic(fmt.Errorf("Fatal error when checking for client login token: %s\n", err))
		}
	}

	// Deploy the ChaincodeSpec
	chaincodeDeploymentSpec, err := s.devops.Deploy(context.Background(), &spec)
	if err != nil {
		// Replace " characters with '
		errVal := strings.Replace(err.Error(), "\"", "'", -1)

		rw.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(rw, "{\"Error\": \"%s\"}", errVal)
		restLogger.Error(fmt.Sprintf("{\"Error\": \"Deploying Chaincode -- %s\"}", errVal))

		return
	}

	// Clients will need the chaincode name in order to invoke or query it
	chainID := chaincodeDeploymentSpec.ChaincodeSpec.ChaincodeID.Name

	rw.WriteHeader(http.StatusOK)
	fmt.Fprintf(rw, "{\"OK\": \"Successfully deployed chainCode.\",\"message\":\""+chainID+"\"}")
	restLogger.Info("Successfuly deployed chainCode: " + chainID + ".\n")
}

// Invoke executes a specified function within a target Chaincode.
func (s *ServerOpenchainREST) Invoke(rw web.ResponseWriter, req *web.Request) {
	restLogger.Info("REST invoking chaincode...")

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
			restLogger.Error("{\"Error\": \"Payload must contain a ChaincodeInvocationSpec.\"}")
		} else {
			rw.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(rw, "{\"Error\": \"%s\"}", errVal)
			restLogger.Error(fmt.Sprintf("{\"Error\": \"%s\"}", errVal))
		}

		return
	}

	// Check that the ChaincodeSpec is not left blank.
	if spec.ChaincodeSpec == nil {
		rw.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(rw, "{\"Error\": \"Payload must contain a ChaincodeSpec.\"}")
		restLogger.Error("{\"Error\": \"Payload must contain a ChaincodeSpec.\"}")

		return
	}

	// Check that the ChaincodeID is not left blank.
	if spec.ChaincodeSpec.ChaincodeID == nil {
		rw.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(rw, "{\"Error\": \"Payload must contain a ChaincodeID.\"}")
		restLogger.Error("{\"Error\": \"Payload must contain a ChaincodeID.\"}")

		return
	}

	// Check that the Chaincode name is not blank.
	if spec.ChaincodeSpec.ChaincodeID.Name == "" {
		rw.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(rw, "{\"Error\": \"Chaincode name may not be blank.\"}")
		restLogger.Error("{\"Error\": \"Chaincode name may not be blank.\"}")

		return
	}

	// Check that the CtorMsg is not left blank.
	if (spec.ChaincodeSpec.CtorMsg == nil) || (spec.ChaincodeSpec.CtorMsg.Function == "") {
		rw.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(rw, "{\"Error\": \"Payload must contain a CtorMsg with a Chaincode function name.\"}")
		restLogger.Error("{\"Error\": \"Payload must contain a CtorMsg with a Chaincode function name.\"}")

		return
	}

	// If security is enabled, add client login token
	if viper.GetBool("security.enabled") {
		chaincodeUsr := spec.ChaincodeSpec.SecureContext
		if chaincodeUsr == "" {
			rw.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(rw, "{\"Error\": \"Must supply username for chaincode when security is enabled.\"}")
			restLogger.Error("{\"Error\": \"Must supply username for chaincode when security is enabled.\"}")

			return
		}

		// Retrieve the REST data storage path
		// Returns /var/openchain/production/client/
		localStore := getRESTFilePath()

		// Check if the user is logged in before sending transaction
		if _, err := os.Stat(localStore + "loginToken_" + chaincodeUsr); err == nil {
			restLogger.Info("Local user '%s' is already logged in. Retrieving login token.\n", chaincodeUsr)

			// Read in the login token
			token, err := ioutil.ReadFile(localStore + "loginToken_" + chaincodeUsr)
			if err != nil {
				rw.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintf(rw, "{\"Error\": \"Fatal error -- %s\"}", err)
				panic(fmt.Errorf("Fatal error when reading client login token: %s\n", err))
			}

			// Add the login token to the chaincodeSpec
			spec.ChaincodeSpec.SecureContext = string(token)

			// If privacy is enabled, mark chaincode as confidential
			if viper.GetBool("security.privacy") {
				spec.ChaincodeSpec.ConfidentialityLevel = pb.ConfidentialityLevel_CONFIDENTIAL
			}
		} else {
			// Check if the token is not there and fail
			if os.IsNotExist(err) {
				rw.WriteHeader(http.StatusUnauthorized)
				fmt.Fprintf(rw, "{\"Error\": \"User not logged in. Use the '/registrar' endpoint to obtain a security token.\"}")
				restLogger.Error("{\"Error\": \"User not logged in. Use the '/registrar' endpoint to obtain a security token.\"}")

				return
			}
			// Unexpected error
			rw.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(rw, "{\"Error\": \"Fatal error -- %s\"}", err)
			panic(fmt.Errorf("Fatal error when checking for client login token: %s\n", err))
		}
	}

	// Invoke the chainCode
	resp, err := s.devops.Invoke(context.Background(), &spec)
	if err != nil {
		// Replace " characters with '
		errVal := strings.Replace(err.Error(), "\"", "'", -1)

		rw.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(rw, "{\"Error\": \"%s\"}", errVal)
		restLogger.Error(fmt.Sprintf("{\"Error\": \"Invoking Chaincode -- %s\"}", errVal))

		return
	}

	// Clients will need the txuuid in order to track it after invocation
	txuuid := resp.Msg

	rw.WriteHeader(http.StatusOK)
	fmt.Fprintf(rw, "{\"OK\": \"Successfully invoked chainCode.\",\"message\": \"%s\"}", string(txuuid))
	restLogger.Info("Successfuly invoked chainCode with txuuid (%s)\n", string(txuuid))
}

// Query performs the requested query on the target Chaincode.
func (s *ServerOpenchainREST) Query(rw web.ResponseWriter, req *web.Request) {
	restLogger.Info("REST querying chaincode...")

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
			restLogger.Error("{\"Error\": \"Payload must contain a ChaincodeInvocationSpec.\"}")
		} else {
			rw.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(rw, "{\"Error\": \"%s\"}", errVal)
			restLogger.Error(fmt.Sprintf("{\"Error\": \"%s\"}", errVal))
		}

		return
	}

	// Check that the ChaincodeSpec is not left blank.
	if spec.ChaincodeSpec == nil {
		rw.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(rw, "{\"Error\": \"Payload must contain a ChaincodeSpec.\"}")
		restLogger.Error("{\"Error\": \"Payload must contain a ChaincodeSpec.\"}")

		return
	}

	// Check that the ChaincodeID is not left blank.
	if spec.ChaincodeSpec.ChaincodeID == nil {
		rw.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(rw, "{\"Error\": \"Payload must contain a ChaincodeID.\"}")
		restLogger.Error("{\"Error\": \"Payload must contain a ChaincodeID.\"}")

		return
	}

	// Check that the Chaincode name is not blank.
	if spec.ChaincodeSpec.ChaincodeID.Name == "" {
		rw.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(rw, "{\"Error\": \"Chaincode name may not be blank.\"}")
		restLogger.Error("{\"Error\": \"Chaincode name may not be blank.\"}")

		return
	}

	// Check that the CtorMsg is not left blank.
	if (spec.ChaincodeSpec.CtorMsg == nil) || (spec.ChaincodeSpec.CtorMsg.Function == "") {
		rw.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(rw, "{\"Error\": \"Payload must contain a CtorMsg with a Chaincode function name.\"}")
		restLogger.Error("{\"Error\": \"Payload must contain a CtorMsg with a Chaincode function name.\"}")

		return
	}

	// If security is enabled, add client login token
	if viper.GetBool("security.enabled") {
		chaincodeUsr := spec.ChaincodeSpec.SecureContext
		if chaincodeUsr == "" {
			rw.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(rw, "{\"Error\": \"Must supply username for chaincode when security is enabled.\"}")
			restLogger.Error("{\"Error\": \"Must supply username for chaincode when security is enabled.\"}")

			return
		}

		// Retrieve the REST data storage path
		// Returns /var/openchain/production/client/
		localStore := getRESTFilePath()

		// Check if the user is logged in before sending transaction
		if _, err := os.Stat(localStore + "loginToken_" + chaincodeUsr); err == nil {
			restLogger.Info("Local user '%s' is already logged in. Retrieving login token.\n", chaincodeUsr)

			// Read in the login token
			token, err := ioutil.ReadFile(localStore + "loginToken_" + chaincodeUsr)
			if err != nil {
				rw.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintf(rw, "{\"Error\": \"Fatal error -- %s\"}", err)
				panic(fmt.Errorf("Fatal error when reading client login token: %s\n", err))
			}

			// Add the login token to the chaincodeSpec
			spec.ChaincodeSpec.SecureContext = string(token)

			// If privacy is enabled, mark chaincode as confidential
			if viper.GetBool("security.privacy") {
				spec.ChaincodeSpec.ConfidentialityLevel = pb.ConfidentialityLevel_CONFIDENTIAL
			}
		} else {
			// Check if the token is not there and fail
			if os.IsNotExist(err) {
				rw.WriteHeader(http.StatusUnauthorized)
				fmt.Fprintf(rw, "{\"Error\": \"User not logged in. Use the '/registrar' endpoint to obtain a security token.\"}")
				restLogger.Error("{\"Error\": \"User not logged in. Use the '/registrar' endpoint to obtain a security token.\"}")

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
		restLogger.Error(fmt.Sprintf("{\"Error\": \"Querying Chaincode -- %s\"}", errVal))

		return
	}

	// Determine if the response received is JSON formatted
	if isJSON(string(resp.Msg)) {
		// Response is JSON formatted, return it as is
		rw.WriteHeader(http.StatusOK)
		fmt.Fprintf(rw, "{\"OK\": %s}", string(resp.Msg))
	} else {
		// Response is not JSON formatted, construct a JSON formatted response
		jsonResponse, err := json.Marshal(restResult{OK: string(resp.Msg)})
		if err != nil {
			rw.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(rw, "{\"Error\": \"%s\"}", err)
			restLogger.Error(fmt.Sprintf("{\"Error marshalling query response\": \"%s\"}", err))

			return
		}

		rw.WriteHeader(http.StatusOK)
		fmt.Fprintf(rw, string(jsonResponse))
	}
}

// GetPeers returns a list of all peer nodes currently connected to the target peer.
func (s *ServerOpenchainREST) GetPeers(rw web.ResponseWriter, req *web.Request) {
	peers, err := s.server.GetPeers(context.Background(), &google_protobuf.Empty{})

	encoder := json.NewEncoder(rw)

	// Check for error
	if err != nil {
		// Failure
		rw.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(rw, "{\"Error\": \"%s\"}", err)
		restLogger.Error(fmt.Sprintf("{\"Error\": \"Querying network peers -- %s\"}", err))
	} else {
		// Success
		rw.WriteHeader(http.StatusOK)
		encoder.Encode(peers)
	}
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
	restLogger.Info("Initializing the REST service...")
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
	router.Get("/registrar/:id/ecert", (*ServerOpenchainREST).GetEnrollmentCert)
	router.Get("/registrar/:id/tcert", (*ServerOpenchainREST).GetTransactionCert)

	router.Get("/chain", (*ServerOpenchainREST).GetBlockchainInfo)
	router.Get("/chain/blocks/:id", (*ServerOpenchainREST).GetBlockByNumber)

	router.Post("/devops/deploy", (*ServerOpenchainREST).Deploy)
	router.Post("/devops/invoke", (*ServerOpenchainREST).Invoke)
	router.Post("/devops/query", (*ServerOpenchainREST).Query)

	router.Get("/transactions/:uuid", (*ServerOpenchainREST).GetTransactionByUUID)

	router.Get("/network/peers", (*ServerOpenchainREST).GetPeers)

	// Add not found page
	router.NotFound((*ServerOpenchainREST).NotFound)

	// Start server
	err := http.ListenAndServe(viper.GetString("rest.address"), router)
	if err != nil {
		restLogger.Error(fmt.Sprintf("ListenAndServe: %s", err))
	}
}
