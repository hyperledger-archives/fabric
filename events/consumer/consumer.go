/*
Copyright IBM Corp. 2016 All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

		 http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package consumer

import (
	"fmt"
	"io"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"github.com/hyperledger/fabric/core/comm"
	ehpb "github.com/hyperledger/fabric/protos"
)

//EventsClient holds the stream and adapter for consumer to work with
type EventsClient struct {
	peerAddress string
	stream      ehpb.Events_ChatClient
	adapter     EventAdapter
}

//NewEventsClient Returns a new grpc.ClientConn to the configured local PEER.
func NewEventsClient(peerAddress string, adapter EventAdapter) *EventsClient {
	return &EventsClient{peerAddress, nil, adapter}
}

//newEventsClientConnectionWithAddress Returns a new grpc.ClientConn to the configured local PEER.
func newEventsClientConnectionWithAddress(peerAddress string) (*grpc.ClientConn, error) {
	if comm.TLSEnabled() {
		return comm.NewClientConnectionWithAddress(peerAddress, true, true, comm.InitTLSForPeer())
	}
	return comm.NewClientConnectionWithAddress(peerAddress, true, false, nil)
}

// RegisterASync - registers interest in a event and doesn't wait for a response
func (ec *EventsClient) RegisterASync(ies []*ehpb.Interest) error {
	emsg := &ehpb.Event{Event: &ehpb.Event_Register{Register: &ehpb.Register{Events: ies}}}
	var err error
	if err = ec.stream.Send(emsg); err != nil {
		fmt.Printf("error on Register send %s\n", err)
	}
	return err
}

// Register - registers interest in a event
func (ec *EventsClient) Register(ies []*ehpb.Interest) error {
	var err error
	if err = ec.RegisterASync(ies); err != nil {
		return err
	}

	regChan := make(chan struct{})
	go func() {
		defer close(regChan)
		in, inerr := ec.stream.Recv()
		if inerr != nil {
			err = inerr
			return
		}
		switch in.Event.(type) {
		case *ehpb.Event_Register:
		case nil:
			err = fmt.Errorf("invalid nil object for register")
		default:
			err = fmt.Errorf("invalid registration object")
		}
	}()
	select {
	case <-regChan:
	case <-time.After(5 * time.Second):
		err = fmt.Errorf("timeout waiting for registration")
	}
	return err
}

// UnregisterASync - Unregisters interest in a event and doesn't wait for a response
func (ec *EventsClient) UnregisterASync(ies []*ehpb.Interest) error {
	emsg := &ehpb.Event{Event: &ehpb.Event_Unregister{Unregister: &ehpb.Unregister{Events: ies}}}
	var err error
	if err = ec.stream.Send(emsg); err != nil {
		fmt.Printf("error on unregister send %s\n", err)
	}

	return err
}

// Unregister - unregisters interest in a event
func (ec *EventsClient) Unregister(ies []*ehpb.Interest) error {
	var err error
	if err = ec.UnregisterASync(ies); err != nil {
		return err
	}

	regChan := make(chan struct{})
	go func() {
		defer close(regChan)
		in, inerr := ec.stream.Recv()
		if inerr != nil {
			err = inerr
			return
		}
		switch in.Event.(type) {
		case *ehpb.Event_Unregister:
		case nil:
			err = fmt.Errorf("invalid nil object for unregister")
		default:
			err = fmt.Errorf("invalid unregistration object")
		}
	}()
	select {
	case <-regChan:
	case <-time.After(5 * time.Second):
		err = fmt.Errorf("timeout waiting for unregistration")
	}
	return err
}

// Recv recieves next event - use when client has not called Start
func (ec *EventsClient) Recv() (*ehpb.Event, error) {
	in, err := ec.stream.Recv()
	if err == io.EOF {
		// read done.
		if ec.adapter != nil {
			ec.adapter.Disconnected(nil)
		}
		return nil, err
	}
	if err != nil {
		if ec.adapter != nil {
			ec.adapter.Disconnected(err)
		}
		return nil, err
	}
	return in, nil
}
func (ec *EventsClient) processEvents() error {
	defer ec.stream.CloseSend()
	for {
		in, err := ec.stream.Recv()
		if err == io.EOF {
			// read done.
			if ec.adapter != nil {
				ec.adapter.Disconnected(nil)
			}
			return nil
		}
		if err != nil {
			if ec.adapter != nil {
				ec.adapter.Disconnected(err)
			}
			return err
		}
		if ec.adapter != nil {
			cont, err := ec.adapter.Recv(in)
			if !cont {
				return err
			}
		}
	}
}

//Start establishes connection with Event hub and registers interested events with it
func (ec *EventsClient) Start() error {
	conn, err := newEventsClientConnectionWithAddress(ec.peerAddress)
	if err != nil {
		return fmt.Errorf("Could not create client conn to %s", ec.peerAddress)
	}

	ies, err := ec.adapter.GetInterestedEvents()
	if err != nil {
		return fmt.Errorf("error getting interested events:%s", err)
	}

	if len(ies) == 0 {
		return fmt.Errorf("must supply interested events")
	}

	serverClient := ehpb.NewEventsClient(conn)
	ec.stream, err = serverClient.Chat(context.Background())
	if err != nil {
		return fmt.Errorf("Could not create client conn to %s", ec.peerAddress)
	}

	if err = ec.Register(ies); err != nil {
		return err
	}

	go ec.processEvents()

	return nil
}

//Stop terminates connection with event hub
func (ec *EventsClient) Stop() error {
	if ec.stream == nil {
		// in case the steam/chat server has not been established earlier, we assume that it's closed, successfully
		return nil
	}
	return ec.stream.CloseSend()
}
