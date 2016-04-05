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
	"fmt"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/looplab/fsm"
	"github.com/spf13/viper"

	"github.com/hyperledger/fabric/core/ledger/statemgmt"
	pb "github.com/hyperledger/fabric/protos"
)

// Handler peer handler implementation.
type Handler struct {
	chatMutex                     sync.Mutex
	ToPeerEndpoint                *pb.PeerEndpoint
	Coordinator                   MessageHandlerCoordinator
	ChatStream                    ChatStream
	doneChan                      chan struct{}
	FSM                           *fsm.FSM
	initiatedStream               bool // Was the stream initiated within this Peer
	registered                    bool
	syncBlocksMutex               sync.Mutex
	syncBlocks                    chan *pb.SyncBlocks
	snapshotRequestHandler        *syncStateSnapshotRequestHandler
	syncStateDeltasRequestHandler *syncStateDeltasHandler
}

// NewPeerHandler returns a new Peer handler
// Is instance of HandlerFactory
func NewPeerHandler(coord MessageHandlerCoordinator, stream ChatStream, initiatedStream bool, nextHandler MessageHandler) (MessageHandler, error) {

	d := &Handler{
		ChatStream:      stream,
		initiatedStream: initiatedStream,
		Coordinator:     coord,
	}
	d.doneChan = make(chan struct{})

	d.snapshotRequestHandler = newSyncStateSnapshotRequestHandler()
	d.syncStateDeltasRequestHandler = newSyncStateDeltasHandler()
	d.FSM = fsm.NewFSM(
		"created",
		fsm.Events{
			{Name: pb.Message_DISC_HELLO.String(), Src: []string{"created"}, Dst: "established"},
			{Name: pb.Message_DISC_GET_PEERS.String(), Src: []string{"established"}, Dst: "established"},
			{Name: pb.Message_DISC_PEERS.String(), Src: []string{"established"}, Dst: "established"},
			{Name: pb.Message_SYNC_BLOCK_ADDED.String(), Src: []string{"established"}, Dst: "established"},
			{Name: pb.Message_SYNC_GET_BLOCKS.String(), Src: []string{"established"}, Dst: "established"},
			{Name: pb.Message_SYNC_BLOCKS.String(), Src: []string{"established"}, Dst: "established"},
			{Name: pb.Message_SYNC_STATE_GET_SNAPSHOT.String(), Src: []string{"established"}, Dst: "established"},
			{Name: pb.Message_SYNC_STATE_SNAPSHOT.String(), Src: []string{"established"}, Dst: "established"},
			{Name: pb.Message_SYNC_STATE_GET_DELTAS.String(), Src: []string{"established"}, Dst: "established"},
			{Name: pb.Message_SYNC_STATE_DELTAS.String(), Src: []string{"established"}, Dst: "established"},
		},
		fsm.Callbacks{
			"enter_state":                                                    func(e *fsm.Event) { d.enterState(e) },
			"before_" + pb.Message_DISC_HELLO.String():              func(e *fsm.Event) { d.beforeHello(e) },
			"before_" + pb.Message_DISC_GET_PEERS.String():          func(e *fsm.Event) { d.beforeGetPeers(e) },
			"before_" + pb.Message_DISC_PEERS.String():              func(e *fsm.Event) { d.beforePeers(e) },
			"before_" + pb.Message_SYNC_BLOCK_ADDED.String():        func(e *fsm.Event) { d.beforeBlockAdded(e) },
			"before_" + pb.Message_SYNC_GET_BLOCKS.String():         func(e *fsm.Event) { d.beforeSyncGetBlocks(e) },
			"before_" + pb.Message_SYNC_BLOCKS.String():             func(e *fsm.Event) { d.beforeSyncBlocks(e) },
			"before_" + pb.Message_SYNC_STATE_GET_SNAPSHOT.String(): func(e *fsm.Event) { d.beforeSyncStateGetSnapshot(e) },
			"before_" + pb.Message_SYNC_STATE_SNAPSHOT.String():     func(e *fsm.Event) { d.beforeSyncStateSnapshot(e) },
			"before_" + pb.Message_SYNC_STATE_GET_DELTAS.String():   func(e *fsm.Event) { d.beforeSyncStateGetDeltas(e) },
			"before_" + pb.Message_SYNC_STATE_DELTAS.String():       func(e *fsm.Event) { d.beforeSyncStateDeltas(e) },
		},
	)

	// If the stream was initiated from this Peer, send an Initial HELLO message
	if d.initiatedStream {
		// Send intiial Hello
		helloMessage, err := d.Coordinator.NewOpenchainDiscoveryHello()
		if err != nil {
			return nil, fmt.Errorf("Error getting new HelloMessage: %s", err)
		}
		if err := d.SendMessage(helloMessage); err != nil {
			return nil, fmt.Errorf("Error creating new Peer Handler, error returned sending %s: %s", pb.Message_DISC_HELLO, err)
		}
	}

	return d, nil
}

func (d *Handler) enterState(e *fsm.Event) {
	peerLogger.Debug("The Peer's bi-directional stream to %s is %s, from event %s\n", d.ToPeerEndpoint, e.Dst, e.Event)
}

func (d *Handler) deregister() error {
	var err error
	if d.registered {
		err = d.Coordinator.DeregisterHandler(d)
		//doneChan is created and waiting for registered handlers only
		d.doneChan <- struct{}{}
		d.registered = false
	}
	return err
}

// To return the PeerEndpoint this Handler is connected to.
func (d *Handler) To() (pb.PeerEndpoint, error) {
	if d.ToPeerEndpoint == nil {
		return pb.PeerEndpoint{}, fmt.Errorf("No peer endpoint for handler")
	}
	return *(d.ToPeerEndpoint), nil
}

// Stop stops this handler, which will trigger the Deregister from the MessageHandlerCoordinator.
func (d *Handler) Stop() error {
	// Deregister the handler
	err := d.deregister()
	if err != nil {
		return fmt.Errorf("Error stopping MessageHandler: %s", err)
	}
	return nil
}

func (d *Handler) beforeHello(e *fsm.Event) {
	peerLogger.Debug("Received %s, parsing out Peer identification", e.Event)
	// Parse out the PeerEndpoint information
	if _, ok := e.Args[0].(*pb.Message); !ok {
		e.Cancel(fmt.Errorf("Received unexpected message type"))
		return
	}
	msg := e.Args[0].(*pb.Message)

	helloMessage := &pb.HelloMessage{}
	err := proto.Unmarshal(msg.Payload, helloMessage)
	if err != nil {
		e.Cancel(fmt.Errorf("Error unmarshalling HelloMessage: %s", err))
		return
	}
	// Store the PeerEndpoint
	d.ToPeerEndpoint = helloMessage.PeerEndpoint
	peerLogger.Debug("Received %s from endpoint=%s", e.Event, helloMessage)

	// If security enabled, need to verify the signature on the hello message
	if viper.GetBool("security.enabled") {
		if err := d.Coordinator.GetSecHelper().Verify(helloMessage.PeerEndpoint.PkiID, msg.Signature, msg.Payload); err != nil {
			e.Cancel(fmt.Errorf("Error Verifying signature for received HelloMessage: %s", err))
			return
		}
		peerLogger.Debug("Verified signature for %s", e.Event)
	}

	if d.initiatedStream == false {
		// Did NOT intitiate the stream, need to send back HELLO
		peerLogger.Debug("Received %s, sending back %s", e.Event, pb.Message_DISC_HELLO.String())
		// Send back out PeerID information in a Hello
		helloMessage, err := d.Coordinator.NewOpenchainDiscoveryHello()
		if err != nil {
			e.Cancel(fmt.Errorf("Error getting new HelloMessage: %s", err))
			return
		}
		if err := d.SendMessage(helloMessage); err != nil {
			e.Cancel(fmt.Errorf("Error sending response to %s:  %s", e.Event, err))
			return
		}
	}
	// Register
	err = d.Coordinator.RegisterHandler(d)
	if err != nil {
		e.Cancel(fmt.Errorf("Error registering Handler: %s", err))
	} else {
		// Registered successfully
		d.registered = true
		go d.start()
	}
}

func (d *Handler) beforeGetPeers(e *fsm.Event) {
	peersMessage, err := d.Coordinator.GetPeers()
	if err != nil {
		e.Cancel(fmt.Errorf("Error Getting Peers: %s", err))
		return
	}
	data, err := proto.Marshal(peersMessage)
	if err != nil {
		e.Cancel(fmt.Errorf("Error Marshalling PeersMessage: %s", err))
		return
	}
	peerLogger.Debug("Sending back %s", pb.Message_DISC_PEERS.String())
	if err := d.SendMessage(&pb.Message{Type: pb.Message_DISC_PEERS, Payload: data}); err != nil {
		e.Cancel(err)
	}
}

func (d *Handler) beforePeers(e *fsm.Event) {
	peerLogger.Debug("Received %s, grabbing peers message", e.Event)
	// Parse out the PeerEndpoint information
	if _, ok := e.Args[0].(*pb.Message); !ok {
		e.Cancel(fmt.Errorf("Received unexpected message type"))
		return
	}
	msg := e.Args[0].(*pb.Message)

	peersMessage := &pb.PeersMessage{}
	err := proto.Unmarshal(msg.Payload, peersMessage)
	if err != nil {
		e.Cancel(fmt.Errorf("Error unmarshalling PeersMessage: %s", err))
		return
	}

	peerLogger.Debug("Received PeersMessage with Peers: %s", peersMessage)
	d.Coordinator.PeersDiscovered(peersMessage)

	// // Can be used to demonstrate Broadcast function
	// if viper.GetString("peer.id") == "jdoe" {
	// 	d.Coordinator.Broadcast(&pb.Message{Type: pb.Message_UNDEFINED})
	// }

}

func (d *Handler) beforeBlockAdded(e *fsm.Event) {
	peerLogger.Debug("Received message: %s", e.Event)
	msg, ok := e.Args[0].(*pb.Message)
	if !ok {
		e.Cancel(fmt.Errorf("Received unexpected message type"))
		return
	}
	// Add the block and any delta state to the ledger
	_ = msg
}

func (d *Handler) when(stateToCheck string) bool {
	return d.FSM.Is(stateToCheck)
}

// HandleMessage handles the Openchain messages for the Peer.
func (d *Handler) HandleMessage(msg *pb.Message) error {
	peerLogger.Debug("Handling Message of type: %s ", msg.Type)
	if d.FSM.Cannot(msg.Type.String()) {
		return fmt.Errorf("Peer FSM cannot handle message (%s) with payload size (%d) while in state: %s", msg.Type.String(), len(msg.Payload), d.FSM.Current())
	}
	err := d.FSM.Event(msg.Type.String(), msg)
	if err != nil {
		if _, ok := err.(*fsm.NoTransitionError); !ok {
			// Only allow NoTransitionError's, all others are considered true error.
			return fmt.Errorf("Peer FSM failed while handling message (%s): current state: %s, error: %s", msg.Type.String(), d.FSM.Current(), err)
			//t.Error("expected only 'NoTransitionError'")
		}
	}
	return nil
}

// SendMessage sends a message to the remote PEER through the stream
func (d *Handler) SendMessage(msg *pb.Message) error {
	//make sure Sends are serialized. Also make sure everyone uses SendMessage
	//instead of calling Send directly on the grpc stream
	d.chatMutex.Lock()
	defer d.chatMutex.Unlock()
	peerLogger.Debug("Sending message to stream of type: %s ", msg.Type)
	err := d.ChatStream.Send(msg)
	if err != nil {
		return fmt.Errorf("Error Sending message through ChatStream: %s", err)
	}
	return nil
}

// start starts the Peer server function
func (d *Handler) start() error {
	discPeriod := viper.GetDuration("peer.discovery.period")
	tickChan := time.NewTicker(discPeriod).C
	peerLogger.Debug("Starting Peer discovery service")
	for {
		select {
		case <-tickChan:
			if err := d.SendMessage(&pb.Message{Type: pb.Message_DISC_GET_PEERS}); err != nil {
				peerLogger.Error(fmt.Sprintf("Error sending %s during handler discovery tick: %s", pb.Message_DISC_GET_PEERS, err))
			}
			// // TODO: For testing only, remove eventually.  Test the blocks transfer functionality.
			// syncBlocksChannel, _ := d.RequestBlocks(&pb.SyncBlockRange{Start: 0, End: 0})
			// go func() {
			// 	for {
			// 		// peerLogger.Debug("Sleeping for 1 second...")
			// 		// time.Sleep(1 * time.Second)
			// 		// peerLogger.Debug("Waking up and pulling from sync channel")
			// 		syncBlocks, ok := <-syncBlocksChannel
			// 		if !ok {
			// 			// Channel was closed
			// 			peerLogger.Debug("Channel closed for SyncBlocks")
			// 			break
			// 		} else {
			// 			peerLogger.Debug("Received SyncBlocks on channel with Range from %d to %d", syncBlocks.Range.Start, syncBlocks.Range.End)
			// 		}
			// 	}
			// }()

			// // TODO: For testing only, remove eventually.  Test the State Snapshot functionality
			// syncStateSnapshotChannel, _ := d.RequestStateSnapshot()
			// go func() {
			// 	for {
			// 		// peerLogger.Debug("Sleeping for 1 second...")
			// 		// time.Sleep(1 * time.Second)
			// 		// peerLogger.Debug("Waking up and pulling from sync channel")
			// 		syncStateSnapshot, ok := <-syncStateSnapshotChannel
			// 		if !ok {
			// 			// Channel was closed
			// 			peerLogger.Debug("Channel closed for SyncStateSnapshot")
			// 			break
			// 		} else {
			// 			peerLogger.Debug("Received SyncStateSnapshot on channel with block = %d, correlationId = %d, sequence = %d, len delta = %d", syncStateSnapshot.BlockNumber, syncStateSnapshot.Request.CorrelationId, syncStateSnapshot.Sequence, len(syncStateSnapshot.Delta))
			// 		}
			// 	}
			// }()

			// // TODO: For testing only, remove eventually.  Test the State Deltas functionality
			// syncStateDeltasChannel, _ := d.RequestStateDeltas(&pb.SyncBlockRange{Start: 0, End: 0})
			// go func() {
			// 	for {
			// 		// peerLogger.Debug("Sleeping for 1 second...")
			// 		// time.Sleep(1 * time.Second)
			// 		// peerLogger.Debug("Waking up and pulling from sync channel")
			// 		syncStateDeltas, ok := <-syncStateDeltasChannel
			// 		if !ok {
			// 			// Channel was closed
			// 			peerLogger.Debug("Channel closed for SyncStateDeltas")
			// 			break
			// 		} else {
			// 			peerLogger.Debug("Received SyncStateDeltas on channel with syncBlockRange = %d-%d, len delta = %d", syncStateDeltas.Range.Start, syncStateDeltas.Range.End, len(syncStateDeltas.Deltas))
			// 		}
			// 	}
			// }()
		case <-d.doneChan:
			peerLogger.Debug("Stopping discovery service")
			return nil
		}
	}
}

// RequestBlocks get the blocks from the other PeerEndpoint based upon supplied SyncBlockRange, will provide them through the returned channel.
// this will also stop writing any received blocks to channels created from Prior calls to RequestBlocks(..)
func (d *Handler) RequestBlocks(syncBlockRange *pb.SyncBlockRange) (<-chan *pb.SyncBlocks, error) {
	d.syncBlocksMutex.Lock()
	defer d.syncBlocksMutex.Unlock()

	if d.syncBlocks != nil {
		// close the previous one
		close(d.syncBlocks)
	}
	d.syncBlocks = make(chan *pb.SyncBlocks, viper.GetInt("peer.sync.blocks.channelSize"))

	// Marshal the SyncBlockRange as the payload
	syncBlockRangeBytes, err := proto.Marshal(syncBlockRange)
	if err != nil {
		return nil, fmt.Errorf("Error marshaling syncBlockRange during GetBlocks: %s", err)
	}
	peerLogger.Debug("Sending %s with Range %s", pb.Message_SYNC_GET_BLOCKS.String(), syncBlockRange)
	if err := d.SendMessage(&pb.Message{Type: pb.Message_SYNC_GET_BLOCKS, Payload: syncBlockRangeBytes}); err != nil {
		return nil, fmt.Errorf("Error sending %s during GetBlocks: %s", pb.Message_SYNC_GET_BLOCKS, err)
	}
	return d.syncBlocks, nil
}

func (d *Handler) beforeSyncGetBlocks(e *fsm.Event) {
	peerLogger.Debug("Received message: %s", e.Event)
	msg, ok := e.Args[0].(*pb.Message)
	if !ok {
		e.Cancel(fmt.Errorf("Received unexpected message type"))
		return
	}
	// Start a separate go FUNC to send the blocks per the SyncBlockRange payload
	syncBlockRange := &pb.SyncBlockRange{}
	err := proto.Unmarshal(msg.Payload, syncBlockRange)
	if err != nil {
		e.Cancel(fmt.Errorf("Error unmarshalling SyncBlockRange in GetBlocks: %s", err))
		return
	}

	go d.sendBlocks(syncBlockRange)
}

func (d *Handler) beforeSyncBlocks(e *fsm.Event) {
	peerLogger.Debug("Received message: %s", e.Event)
	msg, ok := e.Args[0].(*pb.Message)
	if !ok {
		e.Cancel(fmt.Errorf("Received unexpected message type"))
		return
	}
	// Forward the received SyncBlocks to the channel
	syncBlocks := &pb.SyncBlocks{}
	err := proto.Unmarshal(msg.Payload, syncBlocks)
	if err != nil {
		e.Cancel(fmt.Errorf("Error unmarshalling SyncBlocks in beforeSyncBlocks: %s", err))
		return
	}

	peerLogger.Debug("Sending block onto channel for start = %d and end = %d", syncBlocks.Range.Start, syncBlocks.Range.End)

	// Send the message onto the channel, allow for the fact that channel may be closed on send attempt.
	defer func() {
		if x := recover(); x != nil {
			peerLogger.Error(fmt.Sprintf("Error sending syncBlocks to channel: %v", x))
		}
	}()
	// Use non-blocking send, will WARN if missed message.
	select {
	case d.syncBlocks <- syncBlocks:
	default:
		peerLogger.Warning("Did NOT send SyncBlocks message to channel for range: %d - %d", syncBlocks.Range.Start, syncBlocks.Range.End)
	}
}

// sendBlocks sends the blocks based upon the supplied SyncBlockRange over the stream.
func (d *Handler) sendBlocks(syncBlockRange *pb.SyncBlockRange) {
	peerLogger.Debug("Sending blocks %d-%d", syncBlockRange.Start, syncBlockRange.End)
	var blockNums []uint64
	if syncBlockRange.Start > syncBlockRange.End {
		// Send in reverse order
		// note that i is a uint so decrementing i below 0 results in an underflow (i becomes uint.MaxValue). Always stop after i == 0
		for i := syncBlockRange.Start; i >= syncBlockRange.End && i <= syncBlockRange.Start; i-- {
			blockNums = append(blockNums, i)
		}
	} else {
		//
		for i := syncBlockRange.Start; i <= syncBlockRange.End; i++ {
			peerLogger.Debug("Appending to blockNums: %d", i)
			blockNums = append(blockNums, i)
		}
	}
	for _, currBlockNum := range blockNums {
		// Get the Block from
		block, err := d.Coordinator.GetBlockByNumber(currBlockNum)
		if err != nil {
			peerLogger.Error(fmt.Sprintf("Error sending blockNum %d: %s", currBlockNum, err))
			break
		}
		// Encode a SyncBlocks into the payload
		syncBlocks := &pb.SyncBlocks{Range: &pb.SyncBlockRange{Start: currBlockNum, End: currBlockNum}, Blocks: []*pb.Block{block}}
		syncBlocksBytes, err := proto.Marshal(syncBlocks)
		if err != nil {
			peerLogger.Error(fmt.Sprintf("Error marshalling syncBlocks for BlockNum = %d: %s", currBlockNum, err))
			break
		}
		if err := d.SendMessage(&pb.Message{Type: pb.Message_SYNC_BLOCKS, Payload: syncBlocksBytes}); err != nil {
			peerLogger.Error(fmt.Sprintf("Error sending blockNum %d: %s", currBlockNum, err))
			break
		}
	}
}

// ----------------------------------------------------------------------------
//
//  State sync Snapshot functionality
//
//
// ----------------------------------------------------------------------------

// RequestStateSnapshot request the state snapshot deltas from the other PeerEndpoint, will provide them through the returned channel.
// this will also stop writing any received syncStateSnapshot(s) to channels created from Prior calls to RequestStateSnapshot()
func (d *Handler) RequestStateSnapshot() (<-chan *pb.SyncStateSnapshot, error) {
	d.snapshotRequestHandler.Lock()
	defer d.snapshotRequestHandler.Unlock()
	// Reset the handler
	d.snapshotRequestHandler.reset()

	// Create the syncStateSnapshotRequest
	syncStateSnapshotRequest := d.snapshotRequestHandler.createRequest()
	syncStateSnapshotRequestBytes, err := proto.Marshal(syncStateSnapshotRequest)
	if err != nil {
		return nil, fmt.Errorf("Error marshaling syncStateSnapshotRequest during GetStateSnapshot: %s", err)
	}
	peerLogger.Debug("Sending %s with syncStateSnapshotRequest = %s", pb.Message_SYNC_STATE_GET_SNAPSHOT.String(), syncStateSnapshotRequest)
	if err := d.SendMessage(&pb.Message{Type: pb.Message_SYNC_STATE_GET_SNAPSHOT, Payload: syncStateSnapshotRequestBytes}); err != nil {
		return nil, fmt.Errorf("Error sending %s during GetStateSnapshot: %s", pb.Message_SYNC_STATE_GET_SNAPSHOT, err)
	}

	return d.snapshotRequestHandler.channel, nil
}

// beforeSyncStateGetSnapshot triggers the sending of State Snapshot deltas to remote Peer.
func (d *Handler) beforeSyncStateGetSnapshot(e *fsm.Event) {
	peerLogger.Debug("Received message: %s", e.Event)
	msg, ok := e.Args[0].(*pb.Message)
	if !ok {
		e.Cancel(fmt.Errorf("Received unexpected message type"))
		return
	}
	// Unmarshall the sync State snapshot request
	syncStateSnapshotRequest := &pb.SyncStateSnapshotRequest{}
	err := proto.Unmarshal(msg.Payload, syncStateSnapshotRequest)
	if err != nil {
		e.Cancel(fmt.Errorf("Error unmarshalling SyncStateSnapshotRequest in beforeSyncStateGetSnapshot: %s", err))
		return
	}

	// Start a separate go FUNC to send the State snapshot
	go d.sendStateSnapshot(syncStateSnapshotRequest)
}

// beforeSyncStateSnapshot will write the State Snapshot deltas to the respective channel.
func (d *Handler) beforeSyncStateSnapshot(e *fsm.Event) {
	peerLogger.Debug("Received message: %s", e.Event)
	msg, ok := e.Args[0].(*pb.Message)
	if !ok {
		e.Cancel(fmt.Errorf("Received unexpected message type"))
		return
	}
	// Forward the received syncStateSnapshot to the channel
	syncStateSnapshot := &pb.SyncStateSnapshot{}
	err := proto.Unmarshal(msg.Payload, syncStateSnapshot)
	if err != nil {
		e.Cancel(fmt.Errorf("Error unmarshalling syncStateSnapshot in beforeSyncStateSnapshot: %s", err))
		return
	}

	// Send the message onto the channel, allow for the fact that channel may be closed on send attempt.
	defer func() {
		if x := recover(); x != nil {
			peerLogger.Error(fmt.Sprintf("Error sending syncStateSnapshot to channel: %v", x))
		}
	}()
	// Use non-blocking send, will WARN and close channel if missed message.
	d.snapshotRequestHandler.Lock()
	defer d.snapshotRequestHandler.Unlock()
	// Make sure the correlationID matches
	if d.snapshotRequestHandler.shouldHandle(syncStateSnapshot) {
		select {
		case d.snapshotRequestHandler.channel <- syncStateSnapshot:
		default:
			// Was not able to write to the channel, in which case the Snapshot stream is incomplete, and must be discarded, closing the channel
			// without sending the terminating message which would have had an empty byte slice.
			peerLogger.Warning("Did NOT send SyncStateSnapshot message to channel for correlationId = %d, sequence = %d, closing channel as the message has been discarded", syncStateSnapshot.Request.CorrelationId, syncStateSnapshot.Sequence)
			d.snapshotRequestHandler.reset()
		}
	} else {
		//Ignore the message, does not match the current correlationId
		peerLogger.Warning("Ignoring SyncStateSnapshot message with correlationId = %d, sequence = %d, as current correlationId = %d", syncStateSnapshot.Request.CorrelationId, syncStateSnapshot.Sequence, d.snapshotRequestHandler.correlationID)
	}
}

// sendBlocks sends the blocks based upon the supplied SyncBlockRange over the stream.
func (d *Handler) sendStateSnapshot(syncStateSnapshotRequest *pb.SyncStateSnapshotRequest) {
	peerLogger.Debug("Sending state snapshot with correlationId = %d", syncStateSnapshotRequest.CorrelationId)

	snapshot, err := d.Coordinator.GetStateSnapshot()
	if err != nil {
		peerLogger.Error(fmt.Sprintf("Error getting snapshot: %s", err))
		return
	}
	defer snapshot.Release()

	// Iterate over the state deltas and send to requestor
	currBlockNumber := snapshot.GetBlockNumber()
	var sequence uint64
	// Loop through and send the Deltas
	for i := 0; snapshot.Next(); i++ {
		delta := statemgmt.NewStateDelta()
		k, v := snapshot.GetRawKeyValue()
		cID, kID := statemgmt.DecodeCompositeKey(k)
		delta.Set(cID, kID, v, nil)

		deltaAsBytes := delta.Marshal()
		// Encode a SyncStateSnapsot into the payload
		sequence = uint64(i)
		syncStateSnapshot := &pb.SyncStateSnapshot{Delta: deltaAsBytes, Sequence: sequence, BlockNumber: currBlockNumber, Request: syncStateSnapshotRequest}

		syncStateSnapshotBytes, err := proto.Marshal(syncStateSnapshot)
		if err != nil {
			peerLogger.Error(fmt.Sprintf("Error marshalling syncStateSnapsot for BlockNum = %d: %s", currBlockNumber, err))
			break
		}
		if err := d.SendMessage(&pb.Message{Type: pb.Message_SYNC_STATE_SNAPSHOT, Payload: syncStateSnapshotBytes}); err != nil {
			peerLogger.Error(fmt.Sprintf("Error sending syncStateSnapsot for BlockNum = %d: %s", currBlockNumber, err))
			break
		}
	}

	// Now send the terminating message
	syncStateSnapshot := &pb.SyncStateSnapshot{Delta: []byte{}, Sequence: sequence + 1, BlockNumber: currBlockNumber, Request: syncStateSnapshotRequest}
	syncStateSnapshotBytes, err := proto.Marshal(syncStateSnapshot)
	if err != nil {
		peerLogger.Error(fmt.Sprintf("Error marshalling terminating syncStateSnapsot message for correlationId = %d, BlockNum = %d: %s", syncStateSnapshotRequest.CorrelationId, currBlockNumber, err))
		return
	}
	if err := d.SendMessage(&pb.Message{Type: pb.Message_SYNC_STATE_SNAPSHOT, Payload: syncStateSnapshotBytes}); err != nil {
		peerLogger.Error(fmt.Sprintf("Error sending terminating syncStateSnapsot for correlationId = %d, BlockNum = %d: %s", syncStateSnapshotRequest.CorrelationId, currBlockNumber, err))
		return
	}

}

// ----------------------------------------------------------------------------
//
//  State sync Deltas functionality
//
//
// ----------------------------------------------------------------------------

// RequestStateDeltas get the state snapshot deltas from the other PeerEndpoint, will provide them through the returned channel.
// this will also stop writing any received syncStateSnapshot(s) to channels created from Prior calls to GetStateSnapshot()
func (d *Handler) RequestStateDeltas(syncBlockRange *pb.SyncBlockRange) (<-chan *pb.SyncStateDeltas, error) {
	d.syncStateDeltasRequestHandler.Lock()
	defer d.syncStateDeltasRequestHandler.Unlock()
	// Reset the handler
	d.syncStateDeltasRequestHandler.reset()

	// Create the syncStateSnapshotRequest
	syncStateDeltasRequest := d.syncStateDeltasRequestHandler.createRequest(syncBlockRange)
	syncStateDeltasRequestBytes, err := proto.Marshal(syncStateDeltasRequest)
	if err != nil {
		return nil, fmt.Errorf("Error marshaling syncStateDeltasRequest during RequestStateDeltas: %s", err)
	}
	peerLogger.Debug("Sending %s with syncStateDeltasRequest = %s", pb.Message_SYNC_STATE_GET_DELTAS.String(), syncStateDeltasRequest)
	if err := d.SendMessage(&pb.Message{Type: pb.Message_SYNC_STATE_GET_DELTAS, Payload: syncStateDeltasRequestBytes}); err != nil {
		return nil, fmt.Errorf("Error sending %s during RequestStateDeltas: %s", pb.Message_SYNC_STATE_GET_DELTAS, err)
	}

	return d.syncStateDeltasRequestHandler.channel, nil
}

// beforeSyncStateGetDeltas triggers the sending of Get SyncStateDeltas to remote Peer.
func (d *Handler) beforeSyncStateGetDeltas(e *fsm.Event) {
	peerLogger.Debug("Received message: %s", e.Event)
	msg, ok := e.Args[0].(*pb.Message)
	if !ok {
		e.Cancel(fmt.Errorf("Received unexpected message type"))
		return
	}
	// Unmarshall the sync State deltas request
	syncStateDeltasRequest := &pb.SyncStateDeltasRequest{}
	err := proto.Unmarshal(msg.Payload, syncStateDeltasRequest)
	if err != nil {
		e.Cancel(fmt.Errorf("Error unmarshalling SyncStateDeltasRequest in beforeSyncStateGetDeltas: %s", err))
		return
	}

	// Start a separate go FUNC to send the State Deltas
	go d.sendStateDeltas(syncStateDeltasRequest)
}

// sendBlocks sends the blocks based upon the supplied SyncBlockRange over the stream.
func (d *Handler) sendStateDeltas(syncStateDeltasRequest *pb.SyncStateDeltasRequest) {
	peerLogger.Debug("Sending state deltas for block range %d-%d", syncStateDeltasRequest.Range.Start, syncStateDeltasRequest.Range.End)
	var blockNums []uint64
	syncBlockRange := syncStateDeltasRequest.Range
	if syncBlockRange.Start > syncBlockRange.End {
		// Send in reverse order
		for i := syncBlockRange.Start; i >= syncBlockRange.End; i-- {
			blockNums = append(blockNums, i)
		}
	} else {
		//
		for i := syncBlockRange.Start; i <= syncBlockRange.End; i++ {
			peerLogger.Debug("Appending to blockNums: %d", i)
			blockNums = append(blockNums, i)
		}
	}
	for _, currBlockNum := range blockNums {
		// Get the state deltas for Block from coordinator
		stateDelta, err := d.Coordinator.GetStateDelta(currBlockNum)
		if err != nil {
			peerLogger.Error(fmt.Sprintf("Error sending stateDelta for blockNum %d: %s", currBlockNum, err))
			break
		}
		if stateDelta == nil {
			peerLogger.Warning(fmt.Sprintf("Requested to send a stateDelta for blockNum %d which has been discarded", currBlockNum))
			break
		}
		// Encode a SyncStateDeltas into the payload
		stateDeltaBytes := stateDelta.Marshal()
		syncStateDeltas := &pb.SyncStateDeltas{Range: &pb.SyncBlockRange{Start: currBlockNum, End: currBlockNum}, Deltas: [][]byte{stateDeltaBytes}}
		syncStateDeltasBytes, err := proto.Marshal(syncStateDeltas)
		if err != nil {
			peerLogger.Error(fmt.Sprintf("Error marshalling syncStateDeltas for BlockNum = %d: %s", currBlockNum, err))
			break
		}
		if err := d.SendMessage(&pb.Message{Type: pb.Message_SYNC_STATE_DELTAS, Payload: syncStateDeltasBytes}); err != nil {
			peerLogger.Error(fmt.Sprintf("Error sending stateDeltas for blockNum %d: %s", currBlockNum, err))
			break
		}
	}
}

func (d *Handler) beforeSyncStateDeltas(e *fsm.Event) {
	peerLogger.Debug("Received message: %s", e.Event)
	msg, ok := e.Args[0].(*pb.Message)
	if !ok {
		e.Cancel(fmt.Errorf("Received unexpected message type"))
		return
	}
	// Forward the received SyncStateDeltas to the channel
	syncStateDeltas := &pb.SyncStateDeltas{}
	err := proto.Unmarshal(msg.Payload, syncStateDeltas)
	if err != nil {
		e.Cancel(fmt.Errorf("Error unmarshalling SyncStateDeltas in beforeSyncStateDeltas: %s", err))
		return
	}
	peerLogger.Debug("Sending state delta onto channel for start = %d and end = %d", syncStateDeltas.Range.Start, syncStateDeltas.Range.End)

	// Send the message onto the channel, allow for the fact that channel may be closed on send attempt.
	defer func() {
		if x := recover(); x != nil {
			peerLogger.Error(fmt.Sprintf("Error sending syncStateDeltas to channel: %v", x))
		}
	}()

	// Use non-blocking send, will WARN and close channel if missed message.
	d.syncStateDeltasRequestHandler.Lock()
	defer d.syncStateDeltasRequestHandler.Unlock()
	select {
	case d.syncStateDeltasRequestHandler.channel <- syncStateDeltas:
	default:
		// Was not able to write to the channel, in which case the SyncStateDeltasRequest stream is incomplete, and must be discarded, closing the channel
		peerLogger.Warning("Did NOT send SyncStateDeltas message to channel for block range %d-%d, closing channel as the message has been discarded", syncStateDeltas.Range.Start, syncStateDeltas.Range.End)
		d.syncStateDeltasRequestHandler.reset()
	}

}
