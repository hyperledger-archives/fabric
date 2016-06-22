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
	"testing"

	d "github.com/hyperledger/fabric/discovery"
)

func TestAddFindNode(t *testing.T) {
	disc := NewDiscoveryImpl([]string{})
	res := disc.AddNode("foo")
	if !res || !disc.FindNode("foo") {
		t.Fatal("Unable to add a node to the discovery list")
	}
}

func TestRemoveNode(t *testing.T) {
	disc := NewDiscoveryImpl([]string{})
	_ = disc.AddNode("foo")
	if !disc.RemoveNode("foo") || len(disc.GetAllNodes()) != 0 {
		t.Fatalf("Unable to remove a node from the discovery list")
	}
}

func TestGetAllNodes(t *testing.T) {
	init := []string{"a", "b", "c", "d"}
	disc := NewDiscoveryImpl(init)
	nodes := disc.GetAllNodes()

	expected := len(init)
	actual := len(nodes)
	if actual != expected {
		t.Fatalf("Nodes list length should have been %d but is %d", expected, actual)
		return
	}

	for _, node := range nodes {
		if !inArray(node, init) {
			t.Fatalf("%s is not a node in %v", node, init)
		}
	}
}

func TestZeroNodes(t *testing.T) {
	disc := NewDiscoveryImpl([]string{})
	nodes := disc.GetAllNodes()
	if len(nodes) != 0 {
		t.Fatalf("Expected empty list, size is %d instead", len(nodes))
	}
}

func TestRandomNodes(t *testing.T) {
	assertNodeRandomValues(t, []string{"a", "b", "c", "d", "e"}, NewDiscoveryImpl([]string{"a", "b", "c", "d", "e"}))
}

func assertNodeRandomValues(t *testing.T, expected []string, disc d.Discovery) {
	node := disc.GetRandomNode()

	if !inArray(node, expected) {
		t.Fatalf("Node's value should be one of '%v'", expected)
	}

	// Now test that a random value is sometimes returned
	for i := 0; i < 100; i++ {
		if val := disc.GetRandomNode(); node != val {
			return
		}
	}
	t.Fatalf("Returned value was always %s", node)
}

func inArray(element string, array []string) bool {
	for _, val := range array {
		if val == element {
			return true
		}
	}
	return false
}
