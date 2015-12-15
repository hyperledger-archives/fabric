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

	pb "github.com/openblockchain/obc-peer/protos"
)

// Handler peer handler implementation.
type Handler struct {
	ToPeerEndpoint  *pb.PeerEndpoint
	Coordinator     MessageHandlerCoordinator
	ChatStream      ChatStream
	doneChan        chan bool
	FSM             *fsm.FSM
	initiatedStream bool // Was the stream initiated within this Peer
	registered      bool
	syncBlocksMutex sync.Mutex
	syncBlocks      chan *pb.SyncBlocks
}

// NewPeerHandler returns a new Peer handler
func NewPeerHandler(coord MessageHandlerCoordinator, stream ChatStream, initiatedStream bool, nextHandler MessageHandler) (MessageHandler, error) {

	d := &Handler{
		ChatStream:      stream,
		initiatedStream: initiatedStream,
		Coordinator:     coord,
	}
	d.doneChan = make(chan bool)

	d.FSM = fsm.NewFSM(
		"created",
		fsm.Events{
			{Name: pb.OpenchainMessage_DISC_HELLO.String(), Src: []string{"created"}, Dst: "established"},
			{Name: pb.OpenchainMessage_DISC_GET_PEERS.String(), Src: []string{"established"}, Dst: "established"},
			{Name: pb.OpenchainMessage_DISC_PEERS.String(), Src: []string{"established"}, Dst: "established"},
			{Name: pb.OpenchainMessage_SYNC_BLOCK_ADDED.String(), Src: []string{"established"}, Dst: "established"},
			{Name: pb.OpenchainMessage_SYNC_GET_BLOCKS.String(), Src: []string{"established"}, Dst: "established"},
			{Name: pb.OpenchainMessage_SYNC_BLOCKS.String(), Src: []string{"established"}, Dst: "established"},
		},
		fsm.Callbacks{
			"enter_state":                                             func(e *fsm.Event) { d.enterState(e) },
			"before_" + pb.OpenchainMessage_DISC_HELLO.String():       func(e *fsm.Event) { d.beforeHello(e) },
			"before_" + pb.OpenchainMessage_DISC_GET_PEERS.String():   func(e *fsm.Event) { d.beforeGetPeers(e) },
			"before_" + pb.OpenchainMessage_DISC_PEERS.String():       func(e *fsm.Event) { d.beforePeers(e) },
			"before_" + pb.OpenchainMessage_SYNC_BLOCK_ADDED.String(): func(e *fsm.Event) { d.beforeBlockAdded(e) },
			"before_" + pb.OpenchainMessage_SYNC_GET_BLOCKS.String():  func(e *fsm.Event) { d.beforeSyncGetBlocks(e) },
			"before_" + pb.OpenchainMessage_SYNC_BLOCKS.String():      func(e *fsm.Event) { d.beforeSyncBlocks(e) },
		},
	)

	// If the stream was initiated from this Peer, send an Initial HELLO message
	if d.initiatedStream {
		// Send intiial Hello
		helloMessage, err := d.Coordinator.NewOpenchainDiscoveryHello()
		if err != nil {
			return nil, fmt.Errorf("Error getting new HelloMessage: %s", err)
		}
		if err := d.ChatStream.Send(helloMessage); err != nil {
			return nil, fmt.Errorf("Error creating new Peer Handler, error returned sending %s: %s", pb.OpenchainMessage_DISC_HELLO, err)
		}
	}

	return d, nil
}

func (d *Handler) enterState(e *fsm.Event) {
	peerLogger.Debug("The Peer's bi-directional stream to %s is %s, from event %s\n", d.ToPeerEndpoint, e.Dst, e.Event)
}

func (d *Handler) deregister() error {
	if d.registered {
		return d.Coordinator.DeregisterHandler(d)
	}
	return nil
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
	d.doneChan <- true
	d.registered = false
	if err != nil {
		return fmt.Errorf("Error stopping MessageHandler: %s", err)
	}
	return nil
}

func (d *Handler) beforeHello(e *fsm.Event) {
	peerLogger.Debug("Received %s, parsing out Peer identification", e.Event)
	// Parse out the PeerEndpoint information
	if _, ok := e.Args[0].(*pb.OpenchainMessage); !ok {
		e.Cancel(fmt.Errorf("Received unexpected message type"))
		return
	}
	msg := e.Args[0].(*pb.OpenchainMessage)

	helloMessage := &pb.HelloMessage{}
	err := proto.Unmarshal(msg.Payload, helloMessage)
	if err != nil {
		e.Cancel(fmt.Errorf("Error unmarshalling HelloMessage: %s", err))
		return
	}
	// Store the PeerEndpoint
	d.ToPeerEndpoint = helloMessage.PeerEndpoint
	peerLogger.Debug("Received %s from endpoint=%s", e.Event, helloMessage)
	if d.initiatedStream == false {
		// Did NOT intitiate the stream, need to send back HELLO
		peerLogger.Debug("Received %s, sending back %s", e.Event, pb.OpenchainMessage_DISC_HELLO.String())
		// Send back out PeerID information in a Hello
		helloMessage, err := d.Coordinator.NewOpenchainDiscoveryHello()
		if err != nil {
			e.Cancel(fmt.Errorf("Error getting new HelloMessage: %s", err))
			return
		}
		if err := d.ChatStream.Send(helloMessage); err != nil {
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
	peerLogger.Debug("Sending back %s", pb.OpenchainMessage_DISC_PEERS.String())
	if err := d.ChatStream.Send(&pb.OpenchainMessage{Type: pb.OpenchainMessage_DISC_PEERS, Payload: data}); err != nil {
		e.Cancel(err)
	}
}

func (d *Handler) beforePeers(e *fsm.Event) {
	peerLogger.Debug("Received %s, grabbing peers message", e.Event)
	// Parse out the PeerEndpoint information
	if _, ok := e.Args[0].(*pb.OpenchainMessage); !ok {
		e.Cancel(fmt.Errorf("Received unexpected message type"))
		return
	}
	msg := e.Args[0].(*pb.OpenchainMessage)

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
	// 	d.Coordinator.Broadcast(&pb.OpenchainMessage{Type: pb.OpenchainMessage_UNDEFINED})
	// }

}

func (d *Handler) beforeBlockAdded(e *fsm.Event) {
	peerLogger.Debug("Received message: %s", e.Event)
	msg, ok := e.Args[0].(*pb.OpenchainMessage)
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
func (d *Handler) HandleMessage(msg *pb.OpenchainMessage) error {
	peerLogger.Debug("Handling OpenchainMessage of type: %s ", msg.Type)
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
func (d *Handler) SendMessage(msg *pb.OpenchainMessage) error {
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
			if err := d.ChatStream.Send(&pb.OpenchainMessage{Type: pb.OpenchainMessage_DISC_GET_PEERS}); err != nil {
				peerLogger.Error(fmt.Sprintf("Error sending %s during handler discovery tick: %s", pb.OpenchainMessage_DISC_GET_PEERS, err))
			}
			// // TODO: For testing only, remove
			// syncBlocksChannel, _ := d.GetBlocks(&pb.SyncBlockRange{Start: 0, End: 0})
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
			// syncBlock := <-syncBlocksChannel
			// peerLogger.Debug("Received syncBlock with Range: %s, and block count: %s", syncBlock.Range, len(syncBlock.Blocks))
		case <-d.doneChan:
			peerLogger.Debug("Stopping discovery service")
			return nil
		}
	}
}

// GetBlocks get the blocks from the other PeerEndpoint based upon supplied SyncBlockRange, will provide them through the returned channel.
// this will also stop writing any received blocks to channels created from Prior calls to GetBlocks(..)
func (d *Handler) GetBlocks(syncBlockRange *pb.SyncBlockRange) (<-chan *pb.SyncBlocks, error) {
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
	peerLogger.Debug("Sending %s with Range %s", pb.OpenchainMessage_SYNC_GET_BLOCKS.String(), syncBlockRange)
	if err := d.ChatStream.Send(&pb.OpenchainMessage{Type: pb.OpenchainMessage_SYNC_GET_BLOCKS, Payload: syncBlockRangeBytes}); err != nil {
		return nil, fmt.Errorf("Error sending %s during GetBlocks: %s", pb.OpenchainMessage_SYNC_GET_BLOCKS, err)
	}
	return d.syncBlocks, nil
}

func (d *Handler) beforeSyncGetBlocks(e *fsm.Event) {
	peerLogger.Debug("Received message: %s", e.Event)
	msg, ok := e.Args[0].(*pb.OpenchainMessage)
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
	msg, ok := e.Args[0].(*pb.OpenchainMessage)
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
	peerLogger.Debug("TODO: send received syncBlocks for start = %d and end = %d message to channel", syncBlocks.Range.Start, syncBlocks.Range.End)

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
		if err := d.SendMessage(&pb.OpenchainMessage{Type: pb.OpenchainMessage_SYNC_BLOCKS, Payload: syncBlocksBytes}); err != nil {
			peerLogger.Error(fmt.Sprintf("Error sending blockNum %d: %s", currBlockNum, err))
			break
		}
	}
}
