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

package obcpbft

import (
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"

	pb "github.com/openblockchain/obc-peer/protos"
)

func TestExecutorIdle(t *testing.T) {
	obcex := NewOBCExecutor(0, getConfig(), 30, nil, nil)
	done := make(chan struct{})
	go func() {
		obcex.BlockUntilIdle()
		done <- struct{}{}
	}()
	select {
	case <-done:
	case time.After(time.Second):
		t.Fatalf("Executor did not become idle within 1 second")
	}

}
