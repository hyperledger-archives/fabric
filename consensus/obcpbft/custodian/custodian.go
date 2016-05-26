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

package custodian

import (
	"sync"
	"time"

	"github.com/op/go-logging"
)

var logger *logging.Logger // package-level logger

func init() {
	logger = logging.MustGetLogger("consensus/obcpbft/custodian")
}

type custody struct {
	id       string
	data     interface{}
	deadline time.Time
	canceled bool
}

// Custodian provides a timeout service for objects.  The timeout is
// the same for all enqueued objects.  Order is retained.
// When a timeout expires, the object is re-registered automatically
// so that the timeout fires once every timeout duration until removed
type Custodian struct {
	lock     sync.Mutex
	timeout  time.Duration
	timer    *time.Timer
	notifyCb CustodyNotify
	stopCh   chan struct{}
	requests map[string]*custody
	seq      []*custody
}

// CustodyPair is a tuple of enqueued id and data object
type CustodyPair struct {
	ID   string
	Data interface{}
}

// CustodyNotify is the callback function type as called by the
// Custodian when the timeout expires
type CustodyNotify func(id string, data interface{})

// New creates a new Custodian.  Timeout specifies the timeout of the
// Custodian, notifyCb specifies the callback function that is called
// by the Custodian when a timeout expires.
func New(timeout time.Duration, notifyCb CustodyNotify) *Custodian {
	c := &Custodian{
		timeout:  timeout,
		notifyCb: notifyCb,
		requests: make(map[string]*custody),
	}
	c.timer = time.NewTimer(time.Hour)
	c.timer.Stop()
	c.stopCh = make(chan struct{})
	return c
}

// Stop closes down all Custodian activity.  Only used for tests.
func (c *Custodian) Stop() {
	close(c.stopCh)
}

// Register enqueues a new object to the custodian.  The data object
// is referred to by id.
func (c *Custodian) Register(id string, data interface{}) {
	obj := &custody{
		id:       id,
		data:     data,
		deadline: time.Now().Add(c.timeout),
	}
	logger.Debug("Registering %s into custody with timeout %v", id, obj.deadline)
	c.lock.Lock()
	defer c.lock.Unlock()
	c.requests[obj.id] = obj
	c.seq = append(c.seq, obj)
	if len(c.seq) == 1 {
		c.resetTimer()
	}
}

// Remove removes an object from custody.  No callback will be invoked
// on this object anymore.
func (c *Custodian) Remove(id string) bool {
	c.lock.Lock()
	defer c.lock.Unlock()
	logger.Debug("Removing %s from custody", id)
	obj, ok := c.requests[id]
	if ok {
		delete(c.requests, id)
		logger.Debug("Canceling %s", id)
		obj.canceled = true
		obj.data = nil
	}
	return ok
}

// Elements returns all objects that are currently under custody.
func (c *Custodian) Elements() []CustodyPair {
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.syncElements()
}

func (c *Custodian) syncElements() []CustodyPair {
	var m []CustodyPair
	for _, obj := range c.seq {
		if obj.canceled {
			continue
		}
		m = append(m, CustodyPair{obj.id, obj.data})
	}
	return m
}

// RemoveAll deletes all objects from custody.  No callbacks will be
// invoked.  RemoveAll returns all objects that have been under
// custody.
func (c *Custodian) RemoveAll() []CustodyPair {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.timer.Stop()
	m := c.syncElements()
	c.requests = make(map[string]*custody)
	c.seq = nil
	return m
}

// resetTimer must be called with lock held
func (c *Custodian) resetTimer() {
	if len(c.seq) == 0 {
		c.timer.Stop()
		return
	}
	next := c.seq[0]
	diff := next.deadline.Sub(time.Now())
	if diff < 0 {
		diff = 0
	}
	c.timer.Reset(diff)
	logger.Debug("Resetting timer to %v for req %s", diff, next.id)
	go c.notifyRoutine()
}

func (c *Custodian) notifyRoutine() {
	select {
	case <-c.timer.C:
		logger.Debug("Custodian timer expired")
		break
	case <-c.stopCh:
		c.stopCh = nil
		return
	}
	c.lock.Lock()

	var expired *CustodyPair

	if len(c.seq) == 0 {
		return
	}

	obj := c.seq[0]

	if obj.deadline.After(time.Now()) {
		// Timers are always in order in seq
		logger.Debug("Timer expired, but first timer in the future")
		return
	}

	delete(c.requests, obj.id)
	c.seq = c.seq[1:]

	if !obj.canceled {
		logger.Debug("Found %s was not expired", obj.id)
		expired = &CustodyPair{obj.id, obj.data}
	} else {
		c.resetTimer()
		logger.Debug("Found %s was already expired", obj.id)
	}

	c.lock.Unlock()

	if expired != nil {
		logger.Debug("Determined timer expiration was for %s", expired.ID)

		// re-Register the expired request so that it remains in the store until manually removed
		c.Register(expired.ID, expired.Data)

		c.notifyCb(expired.ID, expired.Data)
	} else {
		logger.Debug("Timer expired, but first timer had already been canceled")
	}
}
