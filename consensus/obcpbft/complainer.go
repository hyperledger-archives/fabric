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

package obcpbft

import (
	"time"

	"github.com/hyperledger/fabric/consensus/obcpbft/custodian"
)

// CustodyPair is a tuple of enqueued id and data object
type CustodyPair struct {
	Hash    string
	Request *Request
}

// complaintHandler represents a receiver of complaints
type complaintHandler interface {
	// Complain is called by the complainer to signal that custody
	// or complaint timeout was reached.  The second argument
	// specifies whether the timeout was a Custody timeout (false)
	// or Complaint timeout (true).
	Complain(string, *Request, bool)
}

// complainer provideds lifecycle management to ensure that Requests
// are not censored by a primary.  Initially Requests are put into the
// custody of the complainer using Custody().  When the Request has
// been processed successfully by the network, Success() signals to
// the complainer to drop custody of the Request.  If no success is
// signaled before the custody timeout expires, the complainer will
// signal this fact to the registered handler.  Likewise, Complaint()
// will register a Request for complaining.  Requests stay
// indefinitely in Custody and will repeatedly signal timeouts until
// they are removed using Success(); Requests registered via
// Complaint() only signal a timeout once.
type complainer struct {
	custody    *custodian.Custodian
	complaints *custodian.Custodian

	h complaintHandler
}

// newComplainer creates a new complainer.
func newComplainer(h complaintHandler, custodyTimeout time.Duration, complaintTimeout time.Duration) *complainer {
	c := &complainer{}
	c.custody = custodian.New(custodyTimeout, c.custodyTimeout)
	c.complaints = custodian.New(complaintTimeout, c.complaintTimeout)
	c.h = h
	return c
}

// Stop cleans up all outstanding goroutines of the complainer.
// Typically only used in tests.
func (c *complainer) Stop() {
	c.custody.Stop()
	c.complaints.Stop()
}

// Custody adds a Request into custody of the complainer.  When the
// custody timeout expires, the complaintHandler will be invoked with
// the bool argument set to false.  The Request stays in custody until
// Success() is called.
func (c *complainer) Custody(req *Request) string {
	hash := hashReq(req)
	c.custody.Register(hash, req)
	return hash
}

// custodyTimeout is the callback from the Custodian for requests that
// are in custody.
func (c *complainer) custodyTimeout(hash string, reqParam interface{}) {
	req := reqParam.(*Request)
	c.custody.Register(hash, req)
	c.h.Complain(hash, req, false)
}

// Complaint adds a Request to the complaint queue of the complainer.
// When the complaint timeout expires, the complaintHandler will be
// invoked with the bool argument set to true.  The Request is removed
// from the complaint queue once the timeout expires.
func (c *complainer) Complaint(req *Request) string {
	hash := hashReq(req)
	c.complaints.Register(hash, req)
	return hash
}

// complaintTimeout is the callback from the Custodian for requests
// that are in the complaint queue.
func (c *complainer) complaintTimeout(hash string, reqParam interface{}) {
	req := reqParam.(*Request)
	c.h.Complain(hash, req, true)
}

// Success removes a Request from both custody and complaint queues.
func (c *complainer) Success(req *Request) {
	hash := hashReq(req)
	c.SuccessHash(hash)
}

// SuccessHash is like Success, but takes directly the hash of the
// Request, as returned by hashReq.
func (c *complainer) SuccessHash(hash string) {
	c.custody.Remove(hash)
	c.complaints.Remove(hash)
}

// InCustody returns true if a request is currently in custody
func (c *complainer) InCustody(req *Request) bool {
	hash := hashReq(req)
	return c.custody.InCustody(hash)
}

// CustodyElements returns all requests currently in custody.
func (c *complainer) CustodyElements() []CustodyPair {
	var ret []CustodyPair
	for _, pair := range c.custody.Elements() {
		ret = append(ret, CustodyPair{pair.ID, pair.Data.(*Request)})
	}
	return ret
}

// Clear empties all requests and complaints from custody
func (c *complainer) Clear() {
	c.custody.RemoveAll()
	c.complaints.RemoveAll()
}

// Restart resets custody and complaint queues without calling into
// the complaintHandler.  The complaint queue is drained completely.
// The custody queue timeouts are reset.  Restart returns all requests
// that were in the complaint queue.
func (c *complainer) Restart() []CustodyPair {
	var reqs []CustodyPair

	for _, pair := range c.custody.RemoveAll() {
		c.custody.Register(pair.ID, pair.Data)
	}

	for _, pair := range c.complaints.RemoveAll() {
		reqs = append(reqs, CustodyPair{pair.ID, pair.Data.(*Request)})
	}

	return reqs
}
