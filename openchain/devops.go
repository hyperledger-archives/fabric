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
	"errors"

	"github.com/op/go-logging"
	"golang.org/x/net/context"

	google_protobuf "google/protobuf"

	pb "hub.jazz.net/openchain-peer/protos"
)

var devops_logger = logging.MustGetLogger("devops")

func NewDevopsServer() *devops {
	d := new(devops)
	return d
}

type devops struct {
}

func (*devops) Build(context context.Context, spec *pb.ChainletSpec) (*pb.BuildResult, error) {

	if spec == nil {
		return nil, errors.New("Error in Build, expected code specification, nil received")
	}
	result := &pb.BuildResult{Status: pb.BuildResult_SUCCESS}
	devops_logger.Debug("returning build result: %s", result)
	return result, nil
}

func (*devops) Deploy(context.Context, *google_protobuf.Empty) (*pb.DevopsResponse, error) {
	//status := &pb.BuildResult{Status: pb.BuildResult_SUCCESS}
	//devops_logger.Debug("returning status: %s", status)
	return nil, nil
}
