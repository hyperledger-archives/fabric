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

	"github.com/hyperledger/fabric/discovery"

	"testing"
	"github.com/spf13/viper"
)

func TestDiscovery_GetEmptyRootNode(t *testing.T) {
	discovery.SetDiscoveryService(NewStaticDiscovery(true))
	assertRootNode(t,"")

}

func TestDiscovery_GetSingleValidatingPeer(t *testing.T) {
	viper.Set("peer.discovery.rootnode","someHost")
	discovery.SetDiscoveryService(NewStaticDiscovery(true))
	assertRootNode(t,"someHost")
}

// Check that function always returns first one
func TestDiscovery_GetMultiValidatingPeer(t *testing.T) {
	viper.Set("peer.discovery.rootnode","a,b,c,d,e,f,g,h,i,j,k,l,m,n,o,p,q,r,s,t,u,v,w,x,y,z")
	discovery.SetDiscoveryService(NewStaticDiscovery(true))

	for i := 0; i < 100; i++ {
		assertRootNode(t,"a")
	}
}

// Check that function always returns first one
func TestDiscovery_GetMultiNonValidatingPeer(t *testing.T) {
	viper.Set("peer.discovery.rootnode","a,b,c,d,e")
	discovery.SetDiscoveryService(NewStaticDiscovery(false))
	assertRootNodeRandomValues(t,[]string{"a","b","c","d","e"})
}

func assertRootNode(t *testing.T,expected string) {
	rootNode, err := discovery.GetRootNode()
	if err != nil {
		t.Fail()
		t.Logf("Error getting rootnode:  %s", err)
	}
	if rootNode != expected {
		t.Fail()
		t.Logf("RootNode's value should be '%s'",expected)
	}
}


func assertRootNodeRandomValues(t *testing.T,expected []string) {

	rootNode, err := discovery.GetRootNode()
	if err != nil {
		t.Fail()
		t.Logf("Error getting rootnode:  %s", err)
	}

	if ! inArray(rootNode,expected) {
		t.Fail()
		t.Logf("RootNode's value should be one of '%v'",expected)

	}

	// Now test that a random value is sometimes returned
	for i := 0; i < 100; i++ {
		if val, err := discovery.GetRootNode(); err == nil && rootNode != val {
			return
		} else if err != nil {
			t.Fail()
			t.Logf("Error getting rootnode:  %s", err)
		}
	}
	t.Fail()
	t.Logf("returned value was always %s", rootNode)

}


func inArray(element string,array []string) bool {
	for _, val := range array {
		if val == element {
			return true;
		}
	}
	return false
}

