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
	"reflect"
	"sort"
	"time"
)

type orderedRequests []*Request

func (a *orderedRequests) Len() int {
	return len(*a)
}
func (a *orderedRequests) Swap(i, j int) {
	(*a)[i], (*a)[j] = (*a)[j], (*a)[i]
}
func (a *orderedRequests) Less(i, j int) bool {
	if (*a)[i].Timestamp == nil {
		// a[i] has no timestamp, handle it later, TODO, eventually this should be an error
		return false
	}

	if (*a)[j].Timestamp == nil {
		// a[j] has no timestamp, handle it later, TODO, eventually this should be an error
		return true
	}

	iTime := time.Unix((*a)[i].Timestamp.Seconds, int64((*a)[i].Timestamp.Nanos))
	jTime := time.Unix((*a)[j].Timestamp.Seconds, int64((*a)[j].Timestamp.Nanos))

	return jTime.After(iTime)
}

func (a *orderedRequests) add(request *Request) {
	for _, req := range *a {
		if reflect.DeepEqual(req, request) {
			return
		}
	}

	*a = append(*a, request)
	sort.Sort(a)
}

func (a *orderedRequests) adds(requests []*Request) {
	for _, req := range requests {
		a.add(req)
	}
}

func (a *orderedRequests) remove(request *Request) bool {
	if len(*a) == 0 {
		return false
	}
	defer sort.Sort(a)
	var lastReq *Request
	for i, req := range *a {
		(*a)[i] = lastReq
		if reflect.DeepEqual(req, request) {
			*a = (*a)[1:]
			return true
		}
		lastReq = req
	}
	// This isn't the most efficient, but this should be a degenerate case
	(*a)[0] = lastReq
	return false
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
	*a = nil
}

type requestStore struct {
	outstandingRequests *orderedRequests
	pendingRequests     *orderedRequests
}

// newRequestStore creates a new requestStore.
func newRequestStore() *requestStore {
	return &requestStore{
		outstandingRequests: &orderedRequests{},
		pendingRequests:     &orderedRequests{},
	}
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
	return len(*(rs.outstandingRequests)) > len(*(rs.pendingRequests))
}

// getNextNonPending returns up to the next n outstanding, but not pending requests
func (rs *requestStore) getNextNonPending(n int) []*Request {
	result := make([]*Request, n)
	i := 0
outer:
	for _, oreq := range *(rs.outstandingRequests) {
		if i == n {
			break
		}
		for _, preq := range *(rs.pendingRequests) {
			if reflect.DeepEqual(preq, oreq) {
				continue outer
			}
		}
		result[i] = oreq
		i++
	}

	return result[:i]
}
