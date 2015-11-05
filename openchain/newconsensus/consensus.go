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

package newconsensus

import (
	"bytes"
	"fmt"
	"io"
	"reflect"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/openblockchain/obc-peer/openchain"
	"github.com/openblockchain/obc-peer/openchain/container"
	"github.com/openblockchain/obc-peer/openchain/ledger"
	"github.com/openblockchain/obc-peer/openchain/newconsensus/plugin"
	pb "github.com/openblockchain/obc-peer/protos"

	gp "google/protobuf"

	"github.com/op/go-logging"
	"github.com/spf13/viper"
	"golang.org/x/net/context"
)

// Package-level logger.
var Logger *logging.Logger

func init() {
	Logger = logging.MustGetLogger("consensus")
}

// =============================================================================
// Old definitions that are deprecrated and will be removed soon go here.
// =============================================================================

// ConsenterDeprecated is an interface for every consensus implementation.
type ConsenterDeprecated interface {
	GetParam(param string) (val string, err error)
	Recv(msg []byte) error
}

// CPI (Consensus Programming Interface) is to break the import cycle between
// consensus and consenter implementation.
type CPI interface {
	SetConsenter(c Consenter)
	HandleMsg(msg *pb.OpenchainMessage) error
	Broadcast(msg []byte) error
	ExecTXs(ctxt context.Context, xacts []*pb.Transaction) ([]byte, []error)
}

// =============================================================================
// Structure definitions go here.
// =============================================================================

// consensusLayer carries fields related to the consensus algorithm.
// - config: The link to the config file.
// - receiver: One of those is needed per connection. May hold the FSM that
// regulates the state transitions, data structures for incoming messages, etc.
// TODO: Replace `receiver` with `receivers` whose type is
// `map[string]messageHandler`.
// - plugin: General, plugin-specific properties that relate to the validator
// go here. E.g. a "leader" flag.
type consensusLayer struct {
	config   *viper.Viper
	receiver messageHandler
	plugin   plugin.ConsensusLayer
}

// streamReceiver implements the `messageHandler` interface, and therefore
// provides the means through which we send or receive a message from a stream.
// TODO: Some of this functionality will need to be moved to the `comm` layer.
// Keeping it here for now so that we can have a working prototype.
// - `parent`: Link to the parent `consensusLayer`.
// - `plugin`: For plugin-specific fields/structures that relate to the stream.
// For example, the FSM that sits on the receiving side of a stream and defines
// the acceptable states, triggers, and associated callbacks for a single link
// between two peers. Or data stores for a stream's messages.
// - `stream`: The stream on which this `streamReceiver` operates. TODO: I'd
// like this switched to type `*pb.Peer_ChatClient` but I'm switching to
// `PeerChatStream` to maintain compatibility with Jeff's `peer.go`.
type streamReceiver struct {
	parent Consenter
	plugin plugin.StreamReceiver
	stream openchain.PeerChatStream
}

// =============================================================================
// Interface definitions go here.
// =============================================================================

// Consenter is the interface implemented by: `consensusLayer`.
// - `Broadcast` is used to broadcast a message. A `streamReceiver` r will do
// `r.parent.Broadcast(msg)` which will in turn go through the map of
// established receivers stored in `r.parent` and do a
// `r.stream.SendMessage(msg)` on each one of them.
// - `ExecTX`: Execute TXs on the underlying VM.
// - `GetReceiver`: Attaches a `streamReceiver` for that stream to the
// validating peer's `consensusLayer` thus allowing them to transact on this
// link.
type Consenter interface {
	Broadcast(msg []byte) error
	ExecTXs(ctxt context.Context, txs []*pb.Transaction) ([]byte, []error)
	GetReceiver(stream openchain.PeerChatStream) messageHandler
}

// messageHandler is the interface implemented by: `streamReceiver`. Note that
// the `handleMessage` definition should be written by the plugin developer.
type messageHandler interface {
	getStream() openchain.PeerChatStream
	handleMessage(msg *pb.OpenchainMessage) error
	sendMessage(msg *pb.OpenchainMessage) error
}

// =============================================================================
// Interface implementations go here.
// =============================================================================

// Broadcast is used to broadcast a payload as a message of CONSENSUS type. The
// way things work now, this call will probably be initiated by a
// `streamReceiver`. So, `streamReceiver` `r` will do `r.parent.Broadcast(msg)`
// which will in turn go through the map of established receivers stored in
// `r.parent` and do a `r.stream.SendM(msg)` on each one of them. (Note: This is
// a TODO). For now, there's only one `streamReceiver` per `consensusLayer`.
func (layer *consensusLayer) Broadcast(payload []byte) error {

	if Logger.IsEnabledFor(logging.DEBUG) {
		Logger.Debug("Broadcasting a message.")
	}

	// Wrap as message of type OpenchainMessage_CONSENSUS.
	msgTime := &gp.Timestamp{Seconds: time.Now().Unix(), Nanos: 0}
	msg := &pb.OpenchainMessage{Type: pb.OpenchainMessage_CONSENSUS,
		Timestamp: msgTime,
		Payload:   payload,
	}

	/* TODO: Fix this using reflection.
	if layer.receiver.(streamReceiver) == (streamReceiver{}) {
		return fmt.Error("No stream connections established, cannot broadcast.")
	} */

	stream := layer.receiver.getStream()
	stream.Send(msg)

	if Logger.IsEnabledFor(logging.DEBUG) {
		Logger.Debug("Message broadcasted.")
	}

	return nil
}

// ExecTXs will execute all the transactions listed in the `txs` array
// one-by-one. If all the executions are successful, it returns the candidate
// global state hash, and nil error array.
func (layer *consensusLayer) ExecTXs(ctxt context.Context, txs []*pb.Transaction) ([]byte, []error) {

	if Logger.IsEnabledFor(logging.DEBUG) {
		Logger.Debug("Executing the transactions.")
	}

	// +1 is state hash.
	errors := make([]error, len(txs)+1)
	for i, t := range txs {
		// Add "function" as an argument to be passed.
		newArgs := make([]string, len(t.Args)+1)
		newArgs[0] = t.Function
		copy(newArgs[1:len(t.Args)+1], t.Args)
		// Is there a payload to be passed to the container?
		var buf *bytes.Buffer
		if t.Payload != nil {
			buf = bytes.NewBuffer(t.Payload)
		}
		cds := &pb.ChainletDeploymentSpec{}
		errors[i] = proto.Unmarshal(t.Payload, cds)
		if errors[i] != nil {
			continue
		}
		// Create start request...
		var req container.VMCReqIntf
		vmName, bErr := container.BuildVMName(cds.ChainletSpec)
		if bErr != nil {
			errors[i] = bErr
			continue
		}
		if t.Type == pb.Transaction_CHAINLET_NEW {
			var targz io.Reader = bytes.NewBuffer(cds.CodePackage)
			req = container.CreateImageReq{ID: vmName, Args: newArgs, Reader: targz}
		} else if t.Type == pb.Transaction_CHAINLET_EXECUTE {
			req = container.StartImageReq{ID: vmName, Args: newArgs, Instream: buf}
		} else {
			errors[i] = fmt.Errorf("Invalid transaction type: %s", t.Type.String())
		}
		// ...and execute it. `err` will be nil if successful
		_, errors[i] = container.VMCProcess(ctxt, container.DOCKER, req)
	}

	// TODO: Error processing goes here. For now, assume everything worked fine.

	if Logger.IsEnabledFor(logging.DEBUG) {
		Logger.Debug("Executed the transactions.")
	}

	// Calculate candidate global state hash.
	var stateHash []byte

	_, hashErr := ledger.GetLedger()
	if hashErr == nil {
		// TODO: stateHash, hashErr = ledger.GetTempStateHash()
	}
	errors[len(errors)-1] = hashErr

	return stateHash, errors
}

// GetReceiver attaches a `streamReceiver` for that stream to the peer's
// `consensusLayer` thus allowing them to transact on this link.
func (layer *consensusLayer) GetReceiver(stream openchain.PeerChatStream) messageHandler {

	if Logger.IsEnabledFor(logging.DEBUG) {
		Logger.Debug("Looking for an existing receiver for the stream.")
	}

	// TODO: If we haven't established a receiver for that stream, create one.
	/* if layer.receiver == (streamReceiver{}) {
		if Logger.IsEnabledFor(logging.DEBUG) {
			Logger.Debug("None found.")
		}
		layer.receiver = newReceiver(layer, stream)
	} else {
		if Logger.IsEnabledFor(logging.DEBUG) {
			Logger.Debug("Found an existing receiver for the stream. Returning it.")
		}
	} */

	// TODO: For now, always return a new receiver.
	layer.receiver = newReceiver(layer, stream)

	return layer.receiver
}

// getStream is used to retrieve the stream a `streamReceiver` refers to.
func (receiver *streamReceiver) getStream() openchain.PeerChatStream {

	if Logger.IsEnabledFor(logging.DEBUG) {
		Logger.Debug("Getting the receiver's stream.")
	}

	// streamPointer := reflect.ValueOf(receiver).Elem().FieldByName("stream").Addr()

	return receiver.stream
}

// handleMessage points to the definition written by the plugin developer.
func (receiver *streamReceiver) handleMessage(msg *pb.OpenchainMessage) error {

	if Logger.IsEnabledFor(logging.DEBUG) {
		Logger.Debug("Passing the message to the plugin's messageHandler.")
	}

	pluginPointer := reflect.ValueOf(receiver).Elem().FieldByName("plugin").Addr().Interface().(plugin.StreamReceiver)

	return plugin.HandleMessage(pluginPointer, msg)
}

// sendMessage invokes the Send() method on the stream (type: PeerChatStream).
func (receiver *streamReceiver) sendMessage(msg *pb.OpenchainMessage) error {

	if Logger.IsEnabledFor(logging.DEBUG) {
		Logger.Debug("Sending the message on the stream.")
	}

	err := receiver.stream.Send(msg)

	if err != nil {
		return fmt.Errorf("Error when sending message to the stream.")
	}

	if Logger.IsEnabledFor(logging.DEBUG) {
		Logger.Debug("Message sent.")
	}

	return nil
}

// =============================================================================
// Constructors go here.
// =============================================================================

// NewLayer generates the `consensusLayer` that is attached to a peer to turn
// them into a validating peer.
func NewLayer() Consenter {

	if Logger.IsEnabledFor(logging.DEBUG) {
		Logger.Debug("Creating the consensus layer for the peer.")
	}

	layer := &consensusLayer{}

	if Logger.IsEnabledFor(logging.DEBUG) {
		Logger.Debug("Linking the consensus layer to its config file.")
	}

	layer.config = viper.New()
	layer.config.SetConfigName("config")
	layer.config.AddConfigPath("./plugin/")
	err := layer.config.ReadInConfig()
	if err != nil {
		panic(fmt.Errorf("Fatal error reading consensus algo config: %s", err))
	}

	// Don't worry about setting the `receiver` field.
	// Will be set by `GetReceiver(stream)`.

	// Pointing to the plugin developer's constructor.
	layer.plugin = plugin.NewLayerMember()

	return layer
}

// newReceiver creates a new `streamReceiver`.
func newReceiver(parent Consenter, stream openchain.PeerChatStream) *streamReceiver {

	if Logger.IsEnabledFor(logging.DEBUG) {
		Logger.Debug("Creating a new receiver for the stream.")
	}

	receiver := &streamReceiver{parent: parent, stream: stream}

	// Pointing to the developer's own implementation in the `plugin` package.
	receiver.plugin = plugin.NewReceiverMember()

	if Logger.IsEnabledFor(logging.DEBUG) {
		Logger.Debug("Receiver has been created.")
	}

	return receiver
}

// =============================================================================
// Misc. functions go here.
// =============================================================================

// Adding this function here as an example of transacting with a specific peer.
// It may make sense to have the VP `go chatWithPeer(currentLeader)` upon
// up so that we have the link with the leader established and good to go.
func (layer *consensusLayer) chatWithPeer(peerAddress string) error {

	var ans error

	connection, err := openchain.NewPeerClientConnectionWithAddress(peerAddress)
	if err != nil {
		return fmt.Errorf("Connecting to peer at address %s failed: %s", peerAddress, err)
	}

	// Variable name taken from the PB generated files. Not intuitive, I know.
	peerClient := pb.NewPeerClient(connection)
	// stream's type is pb.Peer_ChatClient
	stream, err := peerClient.Chat(context.Background())

	// Retrieve agent for that stream.
	agent := layer.GetReceiver(stream)

	if err != nil {
		return fmt.Errorf("Error chatting with peer at address %s: %s", peerAddress, err)
	}

	// Never forget.
	defer stream.CloseSend()

	// Send a 'hello' message.
	stream.Send(&pb.OpenchainMessage{Type: pb.OpenchainMessage_DISC_HELLO})

	// Now wait.
	waitc := make(chan struct{})

	// Launch goroutine to receive reply and let the FSM take over.
	go func() {
		for {
			// Receive incoming messages.
			in, err := stream.Recv()
			if err != nil {
				ans = fmt.Errorf("Error receiving from stream with peer at address %s: %s", peerAddress, err)
				close(waitc)
				return
			}
			// Pass the message to the stream agent.
			err = agent.handleMessage(in)
			if err != nil {
				ans = fmt.Errorf("Error handling message received from peer at address %s: %s", peerAddress, err)
				close(waitc)
				return
			}
		}
	}()

	<-waitc
	return ans
}
