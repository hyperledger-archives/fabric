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
	"log"
	"net/http"
	"strconv"

	"golang.org/x/net/context"

	"github.com/gocraft/web"
	"github.com/spf13/viper"

	oc "github.com/openblockchain/obc-peer/openchain"
	pb "github.com/openblockchain/obc-peer/protos"
)

// serverOpenchain is a variable that will be holding the pointer to the
// underlying ServerOpenchain object. This is necessary due to how gocraft/web
// implements context initialization.
var serverOpenchain *oc.ServerOpenchain

// ServerOpenchainREST defines the Openchain REST service object. It exposes
// the methods available on the ServerOpenchain service through a REST API.
type ServerOpenchainREST struct {
	server *oc.ServerOpenchain
}

// SetOpenchainServer is a middleware function that sets the pointer to the
// underlying ServerOpenchain object.
func (s *ServerOpenchainREST) SetOpenchainServer(rw web.ResponseWriter, req *web.Request, next web.NextMiddlewareFunc) {
	s.server = serverOpenchain

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

// GetState returns the value for the specified chaincode ID and key.
func (s *ServerOpenchainREST) GetState(rw web.ResponseWriter, req *web.Request) {

	// Parse out the chaincode id
	chaincodeID := req.PathParams["chaincodeId"]
	key := req.PathParams["key"]

	// Retrieve Block from blockchain
	val, err := s.server.GetState(context.Background(), chaincodeID, key)

	encoder := json.NewEncoder(rw)

	// Check for error
	if err != nil {
		// Failure
		rw.WriteHeader(400)
		fmt.Fprintf(rw, "{\"Error\": \"%s\"}", err)
	} else {
		// Success
		rw.WriteHeader(200)
		encoder.Encode(val)
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
func StartOpenchainRESTServer(server *oc.ServerOpenchain) {
	// Initialize the REST service object
	router := web.New(ServerOpenchainREST{})

	// Record the pointer to the underlying ServerOpenchain object.
	serverOpenchain = server

	// Add middleware
	router.Middleware((*ServerOpenchainREST).SetOpenchainServer)
	router.Middleware((*ServerOpenchainREST).SetResponseType)

	// Add routes
	router.Get("/chain", (*ServerOpenchainREST).GetBlockchainInfo)
	router.Get("/chain/blocks/:id", (*ServerOpenchainREST).GetBlockByNumber)
	router.Get("/state/:chaincodeId/:key", (*ServerOpenchainREST).GetState)

	// Add not found page
	router.NotFound((*ServerOpenchainREST).NotFound)

	// Start server
	err := http.ListenAndServe(viper.GetString("rest.address"), router)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
