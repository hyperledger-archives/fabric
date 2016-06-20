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
	"fmt"
	"sort"
	"strings"
)

type requestContainer struct {
	key string
	req *Request
}

type orderedRequests struct {
	order    []requestContainer
	presence map[string]struct{}
}

func (a *orderedRequests) Len() int {
	return len(a.order)
}
func (a *orderedRequests) Swap(i, j int) {
	a.order[i], a.order[j] = a.order[j], a.order[i]
}
func (a *orderedRequests) Less(i, j int) bool {
	return strings.Compare(a.order[i].key, a.order[j].key) < 0
}

func wrapRequest(req *Request) requestContainer {
	return requestContainer{
		key: fmt.Sprintf("%16x-%16x-%s", req.Timestamp.Seconds, req.Timestamp.Nanos, hashReq(req)),
		req: req,
	}
}

func (a *orderedRequests) has(key string) bool {
	_, ok := a.presence[key]
	return ok
}

func (a *orderedRequests) add(request *Request) {
	rc := wrapRequest(request)
	if !a.has(rc.key) {
		a.order = append(a.order, rc)
		a.presence[rc.key] = struct{}{}
		sort.Sort(a)
	}
}

func (a *orderedRequests) adds(requests []*Request) {
	for _, req := range requests {
		a.add(req)
	}
}

func (a *orderedRequests) remove(request *Request) bool {
	rc := wrapRequest(request)
	if !a.has(rc.key) {
		return false
	}
	for i, it := range a.order {
		if it.key != rc.key {
			continue
		}
		a.order = append(a.order[0:i], a.order[i+1:]...)
		delete(a.presence, rc.key)
		sort.Sort(a)
		return true
	}

	panic("orderedRequest inconsistent")
}

func (a *orderedRequests) removes(requests []*Request) bool {
	allSuccess := true
	for _, req := range requests {
		if !a.remove(req) {
			allSuccess = false
		}
	}

	return allSuccess
}

func (a *orderedRequests) empty() {
	a.order = nil
	a.presence = make(map[string]struct{})
}

type requestStore struct {
	outstandingRequests *orderedRequests
	pendingRequests     *orderedRequests
}

// newRequestStore creates a new requestStore.
func newRequestStore() *requestStore {
	rs := &requestStore{
		outstandingRequests: &orderedRequests{},
		pendingRequests:     &orderedRequests{},
	}
	// initialize data structures
	rs.outstandingRequests.empty()
	rs.pendingRequests.empty()

	return rs
}

// storeOutstanding adds a request to the outstanding request list
func (rs *requestStore) storeOutstanding(request *Request) {
	rs.outstandingRequests.add(request)
}

// storePending adds a request to the pending request list
func (rs *requestStore) storePending(request *Request) {
	rs.pendingRequests.add(request)
}

// storePending adds a slice of requests to the pending request list
func (rs *requestStore) storePendings(requests []*Request) {
	rs.pendingRequests.adds(requests)
}

// remove deletes the request from both the outstanding and pending lists, it returns whether it was found in each list respectively
func (rs *requestStore) remove(request *Request) (outstanding, pending bool) {
	outstanding = rs.outstandingRequests.remove(request)
	pending = rs.pendingRequests.remove(request)
	return
}

// getNextNonPending returns up to the next n outstanding, but not pending requests
func (rs *requestStore) hasNonPending() bool {
	return rs.outstandingRequests.Len() > rs.pendingRequests.Len()
}

// getNextNonPending returns up to the next n outstanding, but not pending requests
func (rs *requestStore) getNextNonPending(n int) (result []*Request) {
	for _, oreq := range rs.outstandingRequests.order {
		if rs.pendingRequests.has(oreq.key) {
			continue
		}
		result = append(result, oreq.req)
		if len(result) == n {
			break
		}
	}

	return result
}
