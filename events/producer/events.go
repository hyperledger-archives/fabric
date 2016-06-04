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

package producer

import (
	//	"encoding/json"
	"fmt"
	"sync"
	"time"

	pb "github.com/hyperledger/fabric/protos"
)

//---- event hub framework ----

//handlerListi uses map to implement a set of handlers. use mutex to access
//the map. Note that we don't have lock/unlock wrapper methods as the lock
//of handler list has to be done under the eventProcessor lock. See
//registerHandler, deRegisterHandler. register/deRegister methods
//will be called only when a new consumer chat starts/ends respectively
//and the big lock should have no performance impact
//
type handlerList interface {
	//find() *handler
	add(ie *pb.Interest, h *handler) bool
	del(ie *pb.Interest, h *handler) bool
	foreach(ie *pb.Event, action func(h *handler))
}

type genericHandlerList struct {
	sync.RWMutex
	// this map used as a list - add/del/iterate
	handlers map[*handler]bool
}

type chaincodeHandlerList struct {
	sync.RWMutex
	// this map used as a list - add/del/iterate
	handlers map[string]map[string]*handler
}

func (hl *chaincodeHandlerList) add(ie *pb.Interest, h *handler) bool {
	hl.Lock()
	emap, ok := hl.handlers[ie.GetChainEvent().Uuid]
	if !ok {
		emap = make(map[string]*handler)
		hl.handlers[ie.GetChainEvent().Uuid] = emap
	}
	emap[ie.GetChainEvent().EventName] = h
	hl.Unlock()
	return true
}
func (hl *chaincodeHandlerList) del(ie *pb.Interest, h *handler) bool {
	hl.Lock()
	emap, ok := hl.handlers[ie.GetChainEvent().Uuid]
	if !ok {
		hl.Unlock()
		return false
	}
	delete(emap, ie.GetChainEvent().EventName)
	hl.Unlock()
	return true
}

func (hl *chaincodeHandlerList) foreach(e *pb.Event, action func(h *handler)) {
	hl.Lock()
	for _, h := range hl.handlers[e.GetChaincodeEvent().Uuid] {
		action(h)
	}
	hl.Unlock()
}

func (hl *genericHandlerList) add(ie *pb.Interest, h *handler) bool {
	hl.Lock()
	if _, ok := hl.handlers[h]; ok {
		hl.Unlock()
		return false
	}
	hl.handlers[h] = true
	hl.Unlock()
	return true
}
func (hl *genericHandlerList) del(ie *pb.Interest, h *handler) bool {
	hl.Lock()
	if _, ok := hl.handlers[h]; !ok {
		hl.Unlock()
		return false
	}
	delete(hl.handlers, h)
	hl.Unlock()
	return true
}

func (hl *genericHandlerList) foreach(e *pb.Event, action func(h *handler)) {
	hl.Lock()
	for h := range hl.handlers {
		action(h)
	}
	hl.Unlock()
}

//eventProcessor has a map of event type to handlers interested in that
//event type. start() kicks of the event processor where it waits for Events
//from producers. We could easily generalize the one event handling loop to one
//per handlerMap if necessary.
//
type eventProcessor struct {
	sync.RWMutex
	eventConsumers map[pb.EventType]handlerList

	//we could generalize this with mutiple channels each with its own size
	eventChannel chan *pb.Event

	//milliseconds timeout for producer to send an event.
	//if < 0, if buffer full, unblocks immediately and not send
	//if 0, if buffer full, will block and guarantee the event will be sent out
	//if > 0, if buffer full, blocks till timeout
	timeout int
}

//global eventProcessor singleton created by initializeEvents. Openchain producers
//send events simply over a reentrant static method
var gEventProcessor *eventProcessor

func (ep *eventProcessor) start() {
	producerLogger.Info("event processor started")
	for {
		//wait for event
		e := <-ep.eventChannel

		var hl handlerList
		eType := getMessageType(e)
		ep.Lock()
		if hl, _ = ep.eventConsumers[eType]; hl == nil {
			producerLogger.Error(fmt.Sprintf("Event of type %s does not exist", eType))
			ep.Unlock()
			continue
		}
		//lock the handler map lock
		ep.Unlock()

		hl.foreach(e, func(h *handler) {
			if e.Event != nil {
				h.SendMessage(e)
			}
		})

	}
}

/*func oldaction(h *handler) {
	if rType := h.responseType(eType); rType != pb.Interest_DONTSEND {
		//if Message is already a generic message, producer must have already converted
		if eType != "generic" {
			switch rType {
			case pb.Interest_JSON:
				if b, err := json.Marshal(e.Event); err != nil {
					producerLogger.Error(fmt.Sprintf("could not marshall JSON for eObject %v(%s)", e.Event, eType))
				} else {
					e.Event = &pb.Event_Generic{Generic: &pb.Generic{EventType: eType, Payload: b}}
				}
			case pb.Interest_PROTOBUF:
			}
		}
		if e.Event != nil {
			h.SendMessage(e)
		}
	}
}*/

//initialize and start
func initializeEvents(bufferSize uint, tout int) {
	if gEventProcessor != nil {
		panic("should not be called twice")
	}

	gEventProcessor = &eventProcessor{eventConsumers: make(map[pb.EventType]handlerList), eventChannel: make(chan *pb.Event, bufferSize), timeout: tout}

	addInternalEventTypes()

	//start the event processor
	go gEventProcessor.start()
}

//AddEventType supported event
func AddEventType(eventType pb.EventType) error {
	gEventProcessor.Lock()
	producerLogger.Debug("registering %s", pb.EventType_name[int32(eventType)])
	if _, ok := gEventProcessor.eventConsumers[eventType]; ok {
		gEventProcessor.Unlock()
		return fmt.Errorf("event type exists %s", pb.EventType_name[int32(eventType)])
	}

	switch eventType   {
	case pb.EventType_BLOCK:
		gEventProcessor.eventConsumers[eventType] = &genericHandlerList{handlers: make(map[*handler]bool)}
	case pb.EventType_CHAINCODE:
		gEventProcessor.eventConsumers[eventType] = &chaincodeHandlerList{handlers: make(map[string]map[string]*handler)}
	}
	gEventProcessor.Unlock()

	return nil
}

func registerHandler(ie *pb.Interest, h *handler) error {
	producerLogger.Debug("registerHandler %s", ie.EventType)

	gEventProcessor.Lock()
	if hl, ok := gEventProcessor.eventConsumers[ie.EventType]; !ok {
		gEventProcessor.Unlock()
		return fmt.Errorf("event type %s does not exist", ie.EventType)
	} else if ok = hl.add(ie, h); !ok {
		gEventProcessor.Unlock()
		return fmt.Errorf("handler already registered for  %s", ie.EventType)
	}

	gEventProcessor.Unlock()
	return nil
}

func deRegisterHandler(ie *pb.Interest, h *handler) error {
	producerLogger.Debug("deRegisterHandler %s", ie.EventType)

	gEventProcessor.Lock()
	if hl, ok := gEventProcessor.eventConsumers[ie.EventType]; !ok {
		gEventProcessor.Unlock()
		return fmt.Errorf("event type %s does not exist", ie.EventType)
	} else if ok = hl.del(ie, h); !ok {
		gEventProcessor.Unlock()
		return fmt.Errorf("handler already deregistered for  %s", ie.EventType)
	}

	return nil
}

//------------- producer API's -------------------------------

//Send sends the event to interested consumers
func Send(e *pb.Event) error {
	if e.Event == nil {
		producerLogger.Error("event not set")
		return fmt.Errorf("event not set")
	}

	if gEventProcessor == nil {
		return nil
	}

	if gEventProcessor.timeout < 0 {
		select {
		case gEventProcessor.eventChannel <- e:
		default:
			return fmt.Errorf("could not send the blocking event")
		}
	} else if gEventProcessor.timeout == 0 {
		gEventProcessor.eventChannel <- e
	} else {
		select {
		case gEventProcessor.eventChannel <- e:
		case <-time.After(time.Duration(gEventProcessor.timeout) * time.Millisecond):
			return fmt.Errorf("could not send the blocking event")
		}
	}

	return nil
}
