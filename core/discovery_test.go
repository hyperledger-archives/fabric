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
	"strings"
	"testing"

	d "github.com/hyperledger/fabric/discovery"
)

func TestDiscovery_AddFindNode(t *testing.T) {
	discovery := NewDiscoveryImpl("")
	res := discovery.AddNode("foo")
	if !res || !discovery.FindNode("foo") {
		t.Fatal("Unable to add a node to the discovery list")
	}
}

func TestDiscovery_RemoveNode(t *testing.T) {
	discovery := NewDiscoveryImpl("")
	_ = discovery.AddNode("foo")
	if !discovery.RemoveNode("foo") || len(discovery.GetAllNodes()) != 0 {
		t.Fatalf("Unable to remove a node from the discovery list")
	}
}

func TestDiscovery_GetAllNodes(t *testing.T) {
	s := "a,b,c,d,e,f,g,h,i,j,k,l,m,n,o,p,q,r,s,t,u,v,w,x,y,z"
	discovery := NewDiscoveryImpl(s)
	nodes := discovery.GetAllNodes()

	expectedArrSize := strings.Count(s, ",") + 1
	if len(nodes) != expectedArrSize {
		t.Fatalf("Nodes list length should have been %d but is %d", expectedArrSize, len(nodes))
		return
	}

	for _, node := range nodes {
		if !strings.Contains(s, node) {
			t.Fatalf("%s is not a node in [%s]", node, s)
		}
	}
}

func TestDiscovery_ZeroNodes(t *testing.T) {
	discovery := NewDiscoveryImpl("")
	nodes := discovery.GetAllNodes()
	if len(nodes) != 0 {
		t.Fatalf("Expected empty list, size is %d instead", len(nodes))
	}
}

func TestDiscovery_RandomNodes(t *testing.T) {
	assertNodeRandomValues(t, []string{"a", "b", "c", "d", "e"}, NewDiscoveryImpl("a,b,c,d,e"))
}

func assertNodeRandomValues(t *testing.T, expected []string, discovery d.Discovery) {
	node := discovery.GetRandomNode()

	if !inArray(node, expected) {
		t.Fatalf("Node's value should be one of '%v'", expected)
	}

	// Now test that a random value is sometimes returned
	for i := 0; i < 100; i++ {
		if val := discovery.GetRandomNode(); node != val {
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
