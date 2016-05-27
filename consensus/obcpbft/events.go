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

package obcpbft

import "time"

// eventReceiver is a consumer of events, processEvent will be called serially
// as events arrive
type eventReceiver interface {
	// processEvent delivers an event to the eventReceiver, if it returns non-nil, the return is the next processed event
	processEvent(e interface{}) interface{}
}

// ------------------------------------------------------------
//
// Threaded object
//
// ------------------------------------------------------------

// threaded holds an exit channel to allow threads to break from a select
type threaded struct {
	exit chan struct{}
}

// halt tells the threaded object's thread to exit
func (t *threaded) halt() {
	select {
	case <-t.exit:
		logger.Warning("Attempted to halt a threaded object twice")
	default:
		close(t.exit)
	}
}

// ------------------------------------------------------------
//
// Event Manager
//
// ------------------------------------------------------------

// eventManager provides a serialized interface for submitting events to
// an eventReceiver on the other side of the queue
type eventManager interface {
	inject(interface{})        // A temporary interface to allow the event manager thread to skip the queue
	queue() chan<- interface{} // Get a write-only reference to the queue, to submit events
	start()                    // Starts the eventManager thread TODO, these thread management things should probably go away
	halt()                     // Stops the eventManager thread
}

// eventManagerImpl is an implementation of eventManger
type eventManagerImpl struct {
	threaded
	receiver eventReceiver
	events   chan interface{}
}

// newEventManager creates an instance of eventManagerImpl
func newEventManagerImpl(er eventReceiver) eventManager {
	return &eventManagerImpl{
		receiver: er,
		events:   make(chan interface{}),
		threaded: threaded{make(chan struct{})},
	}
}

// start creates the go routine necessary to deliver events
func (em *eventManagerImpl) start() {
	go em.eventLoop()
}

// queue returns a write only reference to the event queue
func (em *eventManagerImpl) queue() chan<- interface{} {
	return em.events
}

// sendEvent performs the event loop on a receiver to completion
func sendEvent(receiver eventReceiver, event interface{}) {
	next := event
	for {
		// If an event returns something non-nil, then process it as a new event
		next = receiver.processEvent(next)
		if next == nil {
			break
		}
	}
}

// inject can only safely be called by the eventManager thread itself, it skips the queue
func (em *eventManagerImpl) inject(event interface{}) {
	sendEvent(em.receiver, event)
}

// eventLoop is where the event thread loops, delivering events
func (em *eventManagerImpl) eventLoop() {
	for {
		select {
		case next := <-em.events:
			em.inject(next)
		case <-em.exit:
			logger.Debug("eventLoop told to exit")
			return
		}
	}
}

// ------------------------------------------------------------
//
// Event Timer
//
// ------------------------------------------------------------

// eventTimer is an interface for managing time driven events
// the special contract eventTimer gives which a traditional golang
// timer does not, is that if the event thread calls stop, or reset
// then even if the timer has already fired, the event will not be
// delivered to the event queue
type eventTimer interface {
	softReset(duration time.Duration, event interface{}) // start a new countdown, only if one is not already started
	reset(duration time.Duration, event interface{})     // start a new countdown, clear any pending events
	stop()                                               // stop the countdown, clear any pending events
	halt()                                               // Stops the eventTimer thread
}

// eventTimerFactory abstracts the creation of eventTimers, as they may
// need to be mocked for testing
type eventTimerFactory interface {
	createTimer() eventTimer // Creates an eventTimer which is stopped
}

// eventTimerFactoryImpl implements the eventTimerFactory
type eventTimerFactoryImpl struct {
	manager eventManager // The eventManager to use in constructing the event timers
}

// newEventTimerFactoryImpl creates a new eventTimerFactory for the given eventManager
func newEventTimerFactoryImpl(manager eventManager) eventTimerFactory {
	return &eventTimerFactoryImpl{manager}
}

// createTimer creates a new timer which deliver events to the eventManager for this factory
func (etf *eventTimerFactoryImpl) createTimer() eventTimer {
	return newEventTimer(etf.manager)
}

// timerStart is used to deliver the start request to the eventTimer thread
type timerStart struct {
	hard     bool          // Whether to reset the timer if it is running
	event    interface{}   // What event to push onto the event queue
	duration time.Duration // How long to wait before sending the event
}

// eventTimerImpl is an implementation of eventTimer
type eventTimerImpl struct {
	threaded                   // Gives us the exit chan
	timerChan <-chan time.Time // When non-nil, counts down to preparing to do the event
	startChan chan *timerStart // Channel to deliver the timer start events to the service go routine
	stopChan  chan struct{}    // Channel to deliver the timer stop events to the service go routine
	manager   eventManager     // The event manager to deliver the event to after timer expiration
}

// newEventTimer creates a new instance of eventTimerImpl
func newEventTimer(manager eventManager) eventTimer {
	et := &eventTimerImpl{
		startChan: make(chan *timerStart),
		stopChan:  make(chan struct{}),
		threaded:  threaded{make(chan struct{})},
		manager:   manager,
	}
	go et.loop()
	return et
}

// softReset tells the timer to start a new countdown, only if it is not currently counting down
// this will not clear any pending events
func (et *eventTimerImpl) softReset(timeout time.Duration, event interface{}) {
	et.startChan <- &timerStart{
		duration: timeout,
		event:    event,
		hard:     true,
	}
}

// reset tells the timer to start counting down from a new timeout, this also clears any pending events
func (et *eventTimerImpl) reset(timeout time.Duration, event interface{}) {
	et.startChan <- &timerStart{
		duration: timeout,
		event:    event,
		hard:     false,
	}
}

// stop tells the timer to stop, and not to deliver any pending events
func (et *eventTimerImpl) stop() {
	et.stopChan <- struct{}{}
}

// loop is where the timer thread lives, looping
func (et *eventTimerImpl) loop() {
	var eventDestChan chan<- interface{}
	var event interface{}

	for {
		// A little state machine, relying on the fact that nil channels will block on read/write indefinitely

		select {
		case start := <-et.startChan:
			if et.timerChan != nil {
				if start.hard {
					logger.Debug("Resetting a running timer")
				} else {
					continue
				}
			}
			logger.Debug("Starting timer")
			et.timerChan = time.After(start.duration)
			if eventDestChan != nil {
				logger.Debug("Timer cleared pending event")
			}
			event = start.event
			eventDestChan = nil
		case <-et.stopChan:
			if et.timerChan == nil && eventDestChan == nil {
				logger.Warning("Attempting to stop an unfired idle timer")
			}
			et.timerChan = nil
			logger.Debug("Stopping timer for")
			if eventDestChan != nil {
				logger.Debug("Timer for cleared pending event")
			}
			eventDestChan = nil
			event = nil
		case <-et.timerChan:
			logger.Debug("Event timer fired")
			et.timerChan = nil
			eventDestChan = et.manager.queue()
		case eventDestChan <- event:
			logger.Debug("Timer event delivered")
			eventDestChan = nil
		case <-et.exit:
			logger.Debug("Halting timer")
			return
		}
	}
}

// ------------------------------------------------------------
//
// Event Types
//
// ------------------------------------------------------------

// workEvent is a temporary type, to inject work
type workEvent func()

// viewChangeTimerEvent is sent when the view change timer expires
type viewChangeTimerEvent struct{}

// execDoneEvent is sent when an execution completes
type execDoneEvent struct{}

// stateUpdatedEvent is sent when state transfer completes
type stateUpdatedEvent checkpointMessage

// stateUpdatingEvent is sent when state transfer is initiated
type stateUpdatingEvent checkpointMessage

// pbftMessageEvent is sent when a consensus messages is received to be sent to pbft
type pbftMessageEvent pbftMessage

// batchMessageEvent is sent when a consensus messages is received to be sent to pbft
type batchMessageEvent batchMessage

// batchTimerEvent is sent when the batch timer expires
type batchTimerEvent struct{}

// viewChangedEvent is sent when the view change timer expires
type viewChangedEvent struct{}

// complaintEvent is sent when custody has a complaint
type complaintEvent custodyInfo

// batchExecEvent is sent when a batch execution should take place
type batchExecEvent execInfo

// returnRequestEvent is sent by pbft when we are forwarded a request
type returnRequestEvent *Request
