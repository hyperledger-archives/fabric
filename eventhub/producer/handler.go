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

package producer

import (
	"fmt"

	pb "github.com/openblockchain/obc-peer/eventhub/protos"
)

type handler struct {
	ChatStream      pb.EventHub_ChatServer
	doneChan        chan bool
	registered      bool
	interestedEvents map[string]*pb.InterestedEvent
}

func newEventHubHandler(stream pb.EventHub_ChatServer) (*handler,error) {
	d := &handler{
		ChatStream:      stream,
	}
	d.doneChan = make(chan bool)

	return d, nil
}

// Stop stops this handler
func (d *handler) Stop() error {
	d.deregister()
	d.doneChan <- true
	d.registered = false
	return nil
}

func (d *handler) register(iEvents []*pb.InterestedEvent) error {
        //TODO add the handler to the map for the interested events
	//if successfully done, continue....
	d.interestedEvents = make(map[string]*pb.InterestedEvent)
	for _,v := range iEvents {
		if ie,ok := d.interestedEvents[v.EventType]; ok {
			producerLogger.Error("event %s already registered", v.EventType)
			ie.ResponseType = v.ResponseType
			continue
		}
		if err := registerHandler(v, d); err != nil {
			producerLogger.Error("could not register %s", v)
			continue
		}
			
		d.interestedEvents[v.EventType] = v
	}
	return nil
}

func (d *handler) deregister() {
	for k,v := range d.interestedEvents {
		var ie *pb.InterestedEvent
		var ok bool
		if ie,ok = d.interestedEvents[k]; !ok {
			continue
		}
		if err := deRegisterHandler(v, d); err != nil {
			producerLogger.Error("could not register %s", k)
			continue
		}
		delete(d.interestedEvents, ie.EventType)
	}
}

func (d *handler) responseType(eventType string) pb.InterestedEvent_ResponseType {
	rType := pb.InterestedEvent_DONTSEND
	if d.registered {
		if ie, _ := d.interestedEvents[eventType]; ie != nil {
			rType = ie.ResponseType
		}
	}
	return rType
}

// HandleMessage handles the Openchain messages for the Peer.
func (d *handler) HandleMessage(msg *pb.EventHubMessage) error {
	producerLogger.Debug("Handling EventHubMessage")
	eventsObj := msg.GetRegisterEvent()
	if eventsObj == nil {
		return fmt.Errorf("Invalid object from consumer %v", msg.GetEvent())
	}
	
	if err := d.register(eventsObj.Events); err != nil {
		return fmt.Errorf("Could not register events %s", err)
	}

	//Send can unblock and make client start sending. We have to register before
	//we send on the chat stream
	d.registered = true

	//TODO return supported events.. for now just return the received msg
	if err := d.ChatStream.Send(msg); err != nil {
		return fmt.Errorf("Error sending response to %v:  %s", msg, err)
	} else {
		d.registered = true
	}
	return nil
}

// SendMessage sends a message to the remote PEER through the stream
func (d *handler) SendMessage(msg *pb.EventHubMessage) error {
	err := d.ChatStream.Send(msg)
	if err != nil {
		return fmt.Errorf("Error Sending message through ChatStream: %s", err)
	}
	return nil
}
