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

package core

import (
	"math/rand"
	"sync"
	"time"
)

// DiscoveryImpl is an implementation of Discovery
type DiscoveryImpl struct {
	sync.RWMutex
	nodes  map[string]bool
	seq    []string
	random *rand.Rand
}

// NewDiscoveryImpl is a constructor of a Discovery implementation
func NewDiscoveryImpl(addresses []string) *DiscoveryImpl {
	di := DiscoveryImpl{}
	di.nodes = make(map[string]bool)
	di.seq = addresses
	for _, address := range di.seq {
		di.nodes[address] = true
	}
	di.random = rand.New(rand.NewSource(time.Now().Unix()))
	return &di
}

// AddNode adds an address to the discovery list
func (di *DiscoveryImpl) AddNode(address string) bool {
	di.Lock()
	defer di.Unlock()
	devopsLogger.Debugf(">>> About to add %v to %v", address, di.seq)
	if _, ok := di.nodes[address]; !ok {
		di.seq = append(di.seq, address)
		di.nodes[address] = true
	}
	return di.nodes[address]
}

// RemoveNode removes an address from the discovery list
func (di *DiscoveryImpl) RemoveNode(address string) bool {
	di.Lock()
	defer di.Unlock()
	if _, ok := di.nodes[address]; ok {
		di.nodes[address] = false
	}
	return !di.nodes[address]
}

// GetAllNodes returns an array of all addresses saved in the discovery list
func (di *DiscoveryImpl) GetAllNodes() []string {
	di.Lock()
	defer di.Unlock()
	var addresses []string
	for address, valid := range di.nodes {
		if valid {
			addresses = append(addresses, address) // TODO Expensive, don't quite like it
		}
	}
	return addresses
}

// GetRandomNode returns a random node
func (di *DiscoveryImpl) GetRandomNode() string {
	di.Lock()
	defer di.Unlock()
	var randomNode string
	for {
		randomNode = di.seq[di.random.Intn(len(di.nodes))]
		if di.nodes[randomNode] {
			break
		}
	}
	return randomNode
}

// FindNode returns true if its address is stored in the discovery list
func (di *DiscoveryImpl) FindNode(address string) bool {
	di.Lock()
	defer di.Unlock()
	_, ok := di.nodes[address]
	return ok
}
