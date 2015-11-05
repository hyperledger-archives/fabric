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

package main

import (
	"errors"
	"fmt"

	"golang.org/x/net/context"

	"github.com/openblockchain/obc-peer/openchain/chaincode"
	pb "github.com/openblockchain/obc-peer/protos"
)

type SimpletChainlet struct {
}

func (t *SimpletChainlet) Run(chainletSupportClient pb.ChainletSupportClient) error {
	status, err := chainletSupportClient.GetExecutionContext(context.Background(), &pb.ChainletRequestContext{})
	if err != nil {
		return errors.New(fmt.Sprintf("Error getting execution context: %s\n", err))
	}
	fmt.Printf("Current status: %v  err: %v\n", status, err)
	return nil
}

func main() {
	err := chaincode.Start(new(SimpletChainlet))
	if err != nil {
		fmt.Printf("Error starting Simple chaincode: %s", err)
	}
}
