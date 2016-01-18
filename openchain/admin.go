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
	"runtime"
	"time"

	"github.com/op/go-logging"
	"github.com/spf13/viper"
	"golang.org/x/net/context"

	google_protobuf "google/protobuf"

	pb "github.com/openblockchain/obc-peer/protos"
)

var log = logging.MustGetLogger("server")

// NewAdminServer creates and returns a Admin service instance.
func NewAdminServer() *ServerAdmin {
	s := new(ServerAdmin)
	return s
}

// ServerAdmin implementation of the Admin service for the Peer
type ServerAdmin struct {
}

func worker(id int, die chan struct{}) {
	for {
		select {
		case <-die:
			log.Debug("worker %d terminating", id)
			return
		default:
			log.Debug("%d is working...", id)
			runtime.Gosched()
		}
	}
}

// GetStatus reports the status of the server
func (*ServerAdmin) GetStatus(context.Context, *google_protobuf.Empty) (*pb.ServerStatus, error) {
	status := &pb.ServerStatus{Status: pb.ServerStatus_UNKNOWN}
	die := make(chan struct{})
	log.Debug("Creating %d workers", viper.GetInt("peer.workers"))
	for i := 0; i < viper.GetInt("peer.workers"); i++ {
		go worker(i, die)
	}
	runtime.Gosched()
	<-time.After(1 * time.Millisecond)
	close(die)
	log.Debug("returning status: %s", status)
	return status, nil
}

// StartServer starts the server
func (*ServerAdmin) StartServer(context.Context, *google_protobuf.Empty) (*pb.ServerStatus, error) {
	status := &pb.ServerStatus{Status: pb.ServerStatus_STARTED}
	log.Debug("returning status: %s", status)
	return status, nil
}

// StopServer stops the server
func (*ServerAdmin) StopServer(context.Context, *google_protobuf.Empty) (*pb.ServerStatus, error) {
	status := &pb.ServerStatus{Status: pb.ServerStatus_STOPPED}
	log.Debug("returning status: %s", status)
	return status, nil
}
