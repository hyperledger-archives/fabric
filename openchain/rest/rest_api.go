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
	"log"
	"net/http"
	"strconv"

	"golang.org/x/net/context"

	"github.com/gocraft/web"
	"github.com/golang/protobuf/jsonpb"
	"github.com/spf13/viper"

	oc "github.com/openblockchain/obc-peer/openchain"
	pb "github.com/openblockchain/obc-peer/protos"
)

// serverOpenchain is a variable that will be holding the pointer to the
// underlying ServerOpenchain object. serverDevops is a variable that will be
// holding the pointer to the underlying Devops object. This is necessary due to
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
// header. Currently, it is set to "application/json".
func (s *ServerOpenchainREST) SetResponseType(rw web.ResponseWriter, req *web.Request, next web.NextMiddlewareFunc) {
	rw.Header().Set("Content-Type", "application/json")

	next(rw, req)
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
	var spec pb.ChainletSpec
	err := jsonpb.Unmarshal(req.Body, &spec)

	// Check for proper JSON syntax
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(rw, "{\"Error\": \"%s\"}", err)
		return
	}

	// Check for nil ChainletSpec
	if spec.ChainletID == nil {
		fmt.Fprintf(rw, "{\"Error\": \"Must specify ChainletSpec.\"}")
		return
	}

	// Build the ChainletSpec
	buildResult, err := s.devops.Build(context.Background(), &spec)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(rw, "{\"Error\": \"%s\"}", err)
		return
	}

	encoder := json.NewEncoder(rw)
	err = encoder.Encode(buildResult)
}

// Deploy first builds the docker container that holds the Chaincode
// and then deploys that container to the blockchain.
func (s *ServerOpenchainREST) Deploy(rw web.ResponseWriter, req *web.Request) {
	// Decode the incoming JSON payload
	var spec pb.ChainletSpec
	err := jsonpb.Unmarshal(req.Body, &spec)

	// Check for proper JSON syntax
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(rw, "{\"Error\": \"%s\"}", err)
		return
	}

	// Check for nil ChainletSpec
	if spec.ChainletID == nil {
		fmt.Fprintf(rw, "{\"Error\": \"Must specify ChainletSpec.\"}")
		return
	}

	// Deploy the ChainletSpec
	deployResult, err := s.devops.Deploy(context.Background(), &spec)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(rw, "{\"Error\": \"%s\"}", err)
		return
	}

	encoder := json.NewEncoder(rw)
	err = encoder.Encode(deployResult)
}

// Invoke executes a specified function within a target Chaincode and returns
// a result.
func (s *ServerOpenchainREST) Invoke(rw web.ResponseWriter, req *web.Request) {
	// Decode the incoming JSON payload
	var msg pb.ChainletMessage
	err := jsonpb.Unmarshal(req.Body, &msg)

	// Check for proper JSON syntax
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)

		// Client must supply payload
		if err == io.EOF {
			fmt.Fprintf(rw, "{\"Error\": \"Must provide ChaincodeMessage specification.\"}")
		} else {
			fmt.Fprintf(rw, "{\"Error\": \"%s\"}", err)
		}
		return
	}

	// Check for nil ChainletMessage
	if msg.Function == "" {
		fmt.Fprintf(rw, "{\"Error\": \"Must specify Chaincode function.\"}")
		return
	}

	// Invoke the Chaincode function
	//
	//  ...

	// Return Chaincode invocation result
	// 	encoder := json.NewEncoder(rw)
	// 	err = encoder.Encode(msg)
	rw.WriteHeader(http.StatusOK)
	fmt.Fprintf(rw, "{\"Result\": \"OK\"}")
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
	router := web.New(ServerOpenchainREST{})

	// Record the pointer to the underlying ServerOpenchain and Devops objects.
	serverOpenchain = server
	serverDevops = devops

	// Add middleware
	router.Middleware((*ServerOpenchainREST).SetOpenchainServer)
	router.Middleware((*ServerOpenchainREST).SetResponseType)

	// Add routes
	router.Get("/chain", (*ServerOpenchainREST).GetBlockchainInfo)
	router.Get("/chain/blocks/:id", (*ServerOpenchainREST).GetBlockByNumber)

	router.Get("/state/:chaincodeId/:key", (*ServerOpenchainREST).GetState)

	router.Post("/devops/build", (*ServerOpenchainREST).Build)
	router.Post("/devops/deploy", (*ServerOpenchainREST).Deploy)

	router.Post("/chaincode/:chaincodeId", (*ServerOpenchainREST).Invoke)

	// Add not found page
	router.NotFound((*ServerOpenchainREST).NotFound)

	// Start server
	err := http.ListenAndServe(viper.GetString("rest.address"), router)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
