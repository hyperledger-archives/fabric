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

package peer

import (
	"sync"

	pb "github.com/hyperledger/fabric/protos"
)

//-----------------------------------------------------------------------------
//
// Sync State Snapshot Handler
//
//-----------------------------------------------------------------------------

type syncStateSnapshotRequestHandler struct {
	sync.Mutex
	correlationID uint64
	channel       chan *pb.SyncStateSnapshot
}

func (srh *syncStateSnapshotRequestHandler) reset() {
	close(srh.channel)
	srh.channel = makeStateSnapshotChannel()
	srh.correlationID++
}

func (srh *syncStateSnapshotRequestHandler) shouldHandle(syncStateSnapshot *pb.SyncStateSnapshot) bool {
	return syncStateSnapshot.Request.CorrelationId == srh.correlationID
}

func (srh *syncStateSnapshotRequestHandler) createRequest() *pb.SyncStateSnapshotRequest {
	return &pb.SyncStateSnapshotRequest{CorrelationId: srh.correlationID}
}

func makeStateSnapshotChannel() chan *pb.SyncStateSnapshot {
	return make(chan *pb.SyncStateSnapshot, SyncStateSnapshotChannelSize())
}

func newSyncStateSnapshotRequestHandler() *syncStateSnapshotRequestHandler {
	return &syncStateSnapshotRequestHandler{channel: makeStateSnapshotChannel()}
}

//-----------------------------------------------------------------------------
//
// Sync State Deltas Handler
//
//-----------------------------------------------------------------------------

type syncStateDeltasHandler struct {
	sync.Mutex
	channel chan *pb.SyncStateDeltas
}

func (ssdh *syncStateDeltasHandler) reset() {
	close(ssdh.channel)
	ssdh.channel = makeSyncStateDeltasChannel()
}

func (ssdh *syncStateDeltasHandler) createRequest(syncBlockRange *pb.SyncBlockRange) *pb.SyncStateDeltasRequest {
	return &pb.SyncStateDeltasRequest{Range: syncBlockRange}
}

func makeSyncStateDeltasChannel() chan *pb.SyncStateDeltas {
	return make(chan *pb.SyncStateDeltas, SyncStateDeltasChannelSize())
}

func newSyncStateDeltasHandler() *syncStateDeltasHandler {
	return &syncStateDeltasHandler{channel: makeSyncStateDeltasChannel()}
}
