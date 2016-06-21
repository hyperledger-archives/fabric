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
	"strings"
	"time"
)

// DiscoveryImpl is an implementation of Discovery
type DiscoveryImpl struct {
	nodes  []string
	random *rand.Rand
}

// NewDiscoveryImpl is a constructor of a Discovery implementation
// Accepts as a parameter a single node, or a comma-separated list of nodes with no spaces
func NewDiscoveryImpl(nodes string) *DiscoveryImpl {
	di := DiscoveryImpl{}
	di.nodes = strings.Split(nodes, ",")
	di.random = rand.New(rand.NewSource(time.Now().Unix()))
	return &di
}

// AddNode adds a node to the peer's discovery list
func (di *DiscoveryImpl) AddNode(node string) []string {
	di.nodes = append(di.nodes, node)
	return di.GetAllNodes()
}

// GetAllNodes returns an array of all stored nodes
func (di *DiscoveryImpl) GetAllNodes() []string {
	return di.nodes
}

// GetRandomNode returns a random bootstrap node
func (di *DiscoveryImpl) GetRandomNode() string {
	return di.nodes[di.random.Intn(len(di.nodes))]
}
