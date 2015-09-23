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

package openchain

import (
	"testing"

	"golang.org/x/net/context"

	pb "hub.jazz.net/openchain-peer/protos"
)

func TestDevops_Build_NilSpec(t *testing.T) {
	devopsServer := NewDevopsServer()

	_, err := devopsServer.Build(context.Background(), nil)
	if err == nil {
		t.Fail()
		t.Log("Expected error in Devops.Build call with 'nil' spec:")
	}
	t.Logf("Got expected err: %s", err)
	//performHandshake(t, peerClientConn)
}

func TestDevops_Build(t *testing.T) {
	devopsServer := NewDevopsServer()

	// Build the spec
	spec := &pb.ChainletSpec{}

	buildResult, err := devopsServer.Build(context.Background(), spec)
	if err != nil {
		t.Fail()
		t.Logf("Error in Devops.Build call: %s", err)
	}
	t.Logf("Build result = %s", buildResult)
	//performHandshake(t, peerClientConn)
}
