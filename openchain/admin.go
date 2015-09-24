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

func NewAdminServer() *serverAdmin {
	s := new(serverAdmin)
	return s
}

type serverAdmin struct {
}

func worker(id int, die chan bool) {
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

func (*serverAdmin) GetStatus(context.Context, *google_protobuf.Empty) (*pb.ServerStatus, error) {
	status := &pb.ServerStatus{Status: pb.ServerStatus_UNKNOWN}
	die := make(chan bool)
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

func (*serverAdmin) StartServer(context.Context, *google_protobuf.Empty) (*pb.ServerStatus, error) {
	status := &pb.ServerStatus{Status: pb.ServerStatus_STARTED}
	log.Debug("returning status: %s", status)
	return status, nil
}

func (*serverAdmin) StopServer(context.Context, *google_protobuf.Empty) (*pb.ServerStatus, error) {
	status := &pb.ServerStatus{Status: pb.ServerStatus_STOPPED}
	log.Debug("returning status: %s", status)
	return status, nil
}
