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

// StaticDiscovery is an implementation of Discovery
type StaticDiscovery struct {
	rootNodes []string
	random    *rand.Rand
}

// NewStaticDiscovery is a constructor of a Discovery implementation
// Accepts as a parameter the root node configuration, which is a single node,
// or a comma separated list of nodes with no spaces
func NewStaticDiscovery(rootNodesString string) *StaticDiscovery {
	sd := StaticDiscovery{}
	sd.rootNodes = strings.Split(rootNodesString, ",")
	sd.random = rand.New(rand.NewSource(time.Now().Unix()))
	return &sd
}

// GetRandomNode returns a random root node out of the nodes the discovery was initialized with
func (sd *StaticDiscovery) GetRandomNode() string {
	return sd.rootNodes[sd.random.Intn(len(sd.rootNodes))]
}

// GetRootNodes returns an array of all the nodes it was initialized with
func (sd *StaticDiscovery) GetRootNodes() []string {
	return append([]string{}, sd.rootNodes...)
}
