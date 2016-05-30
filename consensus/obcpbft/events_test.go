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

import (
	"testing"
	"time"
)

// mockEventID is equivalent to MIN_INT to prevent collisions
type mockEvent struct{}

type mockReceiver struct {
	processEventImpl func(event interface{}) interface{}
}

func (mr *mockReceiver) processEvent(event interface{}) interface{} {
	if mr.processEventImpl != nil {
		return mr.processEventImpl(event)
	}
	return nil
}

func newMockManager(processEvent func(event interface{}) interface{}) eventManager {
	return newEventManagerImpl(&mockReceiver{
		processEventImpl: processEvent,
	})
}

// Starts an event timer, waits for the event to be delivered
func TestEventTimerStart(t *testing.T) {
	events := make(chan interface{})
	mr := newMockManager(func(event interface{}) interface{} {
		events <- event
		return nil
	})
	mr.start()
	defer mr.halt()
	timer := newEventTimer(mr)
	defer timer.halt()
	me := &mockEvent{}
	timer.reset(time.Millisecond, me)

	select {
	case e := <-events:
		if e != me {
			t.Fatalf("Received wrong output from event timer")
		}
	case <-time.After(time.Second):
		t.Fatalf("Timed out waiting for event to fire")
	}
}

// Starts an event timer, resets it twice, expects second output
func TestEventTimerHardReset(t *testing.T) {
	events := make(chan interface{})
	mr := newMockManager(func(event interface{}) interface{} {
		events <- event
		return nil
	})
	timer := newEventTimer(mr)
	defer timer.halt()
	me1 := &mockEvent{}
	me2 := &mockEvent{}
	timer.reset(time.Millisecond, me1)
	timer.reset(time.Millisecond, me2)

	mr.start()
	defer mr.halt()

	select {
	case e := <-events:
		if e != me2 {
			t.Fatalf("Received wrong output from event timer")
		}
	case <-time.After(time.Second):
		t.Fatalf("Timed out waiting for event to fire")
	}
}

// Starts an event timer, soft resets it twice, expects first output
func TestEventTimerSoftReset(t *testing.T) {
	events := make(chan interface{})
	mr := newMockManager(func(event interface{}) interface{} {
		events <- event
		return nil
	})
	timer := newEventTimer(mr)
	defer timer.halt()
	me1 := &mockEvent{}
	me2 := &mockEvent{}
	timer.softReset(time.Millisecond, me1)
	timer.softReset(time.Millisecond, me2)

	mr.start()
	defer mr.halt()

	select {
	case e := <-events:
		if e != me1 {
			t.Fatalf("Received wrong output from event timer")
		}
	case <-time.After(time.Second):
		t.Fatalf("Timed out waiting for event to fire")
	}
}

// Starts an event timer, then stops it before delivery is possible, should not receive event
func TestEventTimerStop(t *testing.T) {
	events := make(chan interface{})
	mr := newMockManager(func(event interface{}) interface{} {
		events <- event
		return nil
	})
	timer := newEventTimer(mr)
	defer timer.halt()
	me := &mockEvent{}
	timer.reset(time.Millisecond, me)
	time.Sleep(100 * time.Millisecond) // Allow the timer to fire
	timer.stop()

	mr.start()
	defer mr.halt()

	select {
	case <-events:
		t.Fatalf("Received event output from event timer")
	case <-time.After(100 * time.Millisecond):
		// All good
	}
}

// Replies to an event with a different event, should process both
func TestEventManagerLoop(t *testing.T) {
	success := make(chan struct{})
	m2 := &mockEvent{}
	mr := newMockManager(func(event interface{}) interface{} {
		if event != m2 {
			return m2
		}
		success <- struct{}{}
		return nil
	})
	mr.start()
	defer mr.halt()

	mr.queue() <- &mockEvent{}

	select {
	case <-success:
		// All good
	case <-time.After(2 * time.Second):
		t.Fatalf("Did not succeed processing second event")
	}
}
