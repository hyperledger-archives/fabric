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
	"encoding/json"
	"fmt"
	"sync"
	pb "github.com/openblockchain/obc-peer/eventhub/protos"
)

//----Event Types -----
const (
	RegisterType = "register"
	TransactionType = "transaction"
)

//---- event hub framework ----

type eventProcessor struct {
	sync.RWMutex
	eventConsumers map[string] *handlerList

	//we could generalize this with mutiple channels each with its own size
	eventChannel chan *pb.EventHubMessage
}

func (ep *eventProcessor) start() {
	producerLogger.Error("event processor started")
	for {
		e := <- gEventProcessor.eventChannel
		var hl *handlerList
		eType := getMessageType(e)
		gEventProcessor.Lock()
		if hl,_ = gEventProcessor.eventConsumers[eType]; hl == nil {
			producerLogger.Error("Event of type %s does not exist", eType)
			gEventProcessor.Unlock()
			continue
		}
		hl.Lock()
		gEventProcessor.Unlock()

		for h,_ := range hl.handlers {
			if rType := h.responseType(eType); rType != pb.InterestedEvent_DONTSEND {
				//if EventHubMessage is already a generic message, producer must have converted
				if eType != "generic" {
					switch rType {
					case pb.InterestedEvent_JSON:
						if b, err := json.Marshal(e.Event); err != nil {
							fmt.Printf("could not marshall JSON for eObject %v\n", e.Event)
						} else {
							e.Event = &pb.EventHubMessage_GenericEvent{ &pb.GenericEvent{pb.GenericEvent_JSON, "json", b }}
						}
					case pb.InterestedEvent_NATIVE:
					}
				}
				if e.Event != nil {
					h.SendMessage(e)
				}
			}
		}
		hl.Unlock()
	}
}

var gEventProcessor *eventProcessor

type handlerList struct {
	sync.RWMutex
	handlers map[*handler]bool
}

func initializeEvents(bufferSize uint) {
	gEventProcessor = &eventProcessor { eventConsumers: make(map[string]*handlerList), eventChannel: make(chan *pb.EventHubMessage, bufferSize) }

	addInternalEventTypes()

	go gEventProcessor.start()
}

func getMessageType(e *pb.EventHubMessage) string {
	switch e.Event.(type) {
	case *pb.EventHubMessage_RegisterEvent : 
		return "register"
	case *pb.EventHubMessage_TransactionEvent : 
		return "transaction"
	case *pb.EventHubMessage_GenericEvent:
		return "generic"
	default:
		return ""
	}
}

//Send sends the event to interested consumers
func Send(e *pb.EventHubMessage) error {
	if gEventProcessor == nil {
		return nil
	}

	if e.Event == nil {
		producerLogger.Error("event not set")
		return fmt.Errorf("event not set")
	}

	gEventProcessor.eventChannel <- e

	return nil
}

//AddEventType supported event
func AddEventType(eventType string) error {
	gEventProcessor.Lock()
	producerLogger.Debug("registering %s", eventType)
	if _,ok := gEventProcessor.eventConsumers[eventType]; ok {
		gEventProcessor.Unlock()
		return fmt.Errorf("event type exists %s", eventType)
	}

	gEventProcessor.eventConsumers[eventType] = &handlerList { handlers: make(map[*handler]bool) }
	gEventProcessor.Unlock()

	return nil
}

//should be called at init time to register supported internal events
func addInternalEventTypes() {
	AddEventType(TransactionType)
	AddEventType(RegisterType)
}

//Unmarshall received event from client
func jsonToEventObject(jsonRep []byte, eObj interface{}) error {
	return json.Unmarshal(jsonRep, eObj)
}

func registerHandler(ie *pb.InterestedEvent, h *handler) error {
	producerLogger.Debug("registerHandler %s", ie.EventType)

	gEventProcessor.Lock()
	if hl,ok := gEventProcessor.eventConsumers[ie.EventType]; !ok {
		gEventProcessor.Unlock()
		return fmt.Errorf("event type %s does not exist", ie.EventType)
	} else if _,ok = hl.handlers[h]; ok {
		gEventProcessor.Unlock()
		return fmt.Errorf("handler already registered for  %s", ie.EventType)
	} else {
		hl.Lock()
		gEventProcessor.Unlock()
		hl.handlers[h] = true
		hl.Unlock()
	}

	return nil
}

func deRegisterHandler(ie *pb.InterestedEvent, h *handler) error {
	producerLogger.Debug("deRegisterHandler %s", ie.EventType)

	gEventProcessor.Lock()
	if hl,ok := gEventProcessor.eventConsumers[ie.EventType]; !ok {
		gEventProcessor.Unlock()
		return fmt.Errorf("event type %s does not exist", ie.EventType)
	} else if _,ok = hl.handlers[h]; !ok {
		gEventProcessor.Unlock()
		return fmt.Errorf("handler already deregistered for  %s", ie.EventType)
	} else {
		hl.Lock()
		gEventProcessor.Unlock()

		delete(hl.handlers, h)
		hl.Unlock()
	}

	return nil
}

