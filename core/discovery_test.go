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

func TestZeroNodes(t *testing.T) {
	disc := NewDiscoveryImpl()
	nodes := disc.GetAllNodes()
	if len(nodes) != 0 {
		t.Fatalf("Expected empty list, got a list of size %d instead", len(nodes))
	}
}

func TestAddFindNode(t *testing.T) {
	disc := NewDiscoveryImpl()
	res := disc.AddNode("foo")
	if !res || !disc.FindNode("foo") {
		t.Fatal("Unable to add a node to the discovery list")
	}
}

func TestRemoveNode(t *testing.T) {
	disc := NewDiscoveryImpl()
	_ = disc.AddNode("foo")
	if !disc.RemoveNode("foo") || len(disc.GetAllNodes()) != 0 {
		t.Fatalf("Unable to remove a node from the discovery list")
	}
}

func TestGetAllNodes(t *testing.T) {
	initList := []string{"a", "b", "c", "d"}
	disc := NewDiscoveryImpl()
	for i := range initList {
		_ = disc.AddNode(initList[i])
	}
	nodes := disc.GetAllNodes()

	expected := len(initList)
	actual := len(nodes)
	if actual != expected {
		t.Fatalf("Nodes list length should have been %d but is %d", expected, actual)
		return
	}

	for _, node := range nodes {
		if !inArray(node, initList) {
			t.Fatalf("%s is found in the discovery list but not in the initial list %v", node, initList)
		}
	}
}

func TestRandomNodes(t *testing.T) {
	initList := []string{"a", "b", "c", "d"}
	disc := NewDiscoveryImpl()
	for i := range initList {
		_ = disc.AddNode(initList[i])
	}
	assertNodeRandomValues(t, initList, disc)
}

func inArray(element string, array []string) bool {
	for _, val := range array {
		if val == element {
			return true
		}
	}
	return false
}

func assertNodeRandomValues(t *testing.T, expected []string, disc d.Discovery) {
	node := disc.GetRandomNode()

	if !inArray(node, expected) {
		t.Fatalf("%s is found in the discovery list but not in the initial list %v", node, expected)
	}

	// Now test that a random value is sometimes returned
	for i := 0; i < 100; i++ {
		if val := disc.GetRandomNode(); node != val {
			return
		}
	}
	t.Fatalf("Returned value was always %s", node)
}
