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

func TestDiscovery_GetEmptyRootNode(t *testing.T) {
	assertRandomRootNode(t, "", NewStaticDiscovery(""))
}

func TestDiscovery_GetEmptyRootNodes(t *testing.T) {
	discovery := NewStaticDiscovery("")
	rootNodes := discovery.GetRootNodes()
	if size := len(rootNodes); size != 1 || rootNodes[0] != "" {
		t.Fatalf("Needed input is not ['']")
	}
}

func TestDiscovery_GetSinglePeer(t *testing.T) {
	assertRandomRootNode(t, "someHost", NewStaticDiscovery("someHost"))
}

func TestDiscovery_GetAllPeers(t *testing.T) {
	s := "a,b,c,d,e,f,g,h,i,j,k,l,m,n,o,p,q,r,s,t,u,v,w,x,y,z"
	discovery := NewStaticDiscovery(s)
	rootNodes := discovery.GetRootNodes()

	expectedArrSize := strings.Count(s, ",") + 1
	if len(rootNodes) != expectedArrSize {
		t.Fatalf("Rootnodes length should have been %d but is %d", expectedArrSize, len(rootNodes))
		return
	}

	for _, rootNode := range rootNodes {
		if !strings.Contains(s, rootNode) {
			t.Fatalf("%s is not a rootNode of [%s]", rootNode, s)
		}
	}
}

func TestDiscovery_GetMulti(t *testing.T) {
	assertRootNodeRandomValues(t, []string{"a", "b", "c", "d", "e"}, NewStaticDiscovery("a,b,c,d,e"))
}

func assertRandomRootNode(t *testing.T, expected string, discovery d.Discovery) {
	rootNode := discovery.GetRandomNode()

	if rootNode != expected {
		t.Fatalf("RootNode's value should be '%s'", expected)
	}
}

func assertRootNodeRandomValues(t *testing.T, expected []string, discovery d.Discovery) {
	rootNode := discovery.GetRandomNode()

	if !inArray(rootNode, expected) {
		t.Fatalf("RootNode's value should be one of '%v'", expected)
	}

	// Now test that a random value is sometimes returned
	for i := 0; i < 100; i++ {
		if val := discovery.GetRandomNode(); rootNode != val {
			return
		}
	}
	t.Fatalf("returned value was always %s", rootNode)

}

func inArray(element string, array []string) bool {
	for _, val := range array {
		if val == element {
			return true
		}
	}
	return false
}
