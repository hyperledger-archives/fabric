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
	"math/rand"
	"time"

	"github.com/spf13/viper"

)



type StaticDiscovery struct {
	rootNodes []string
	random *rand.Rand
	isValidator bool
}

func NewStaticDiscovery(isValidator bool) *StaticDiscovery {
	staticDisc := StaticDiscovery{}
	staticDisc.rootNodes   = strings.Split(viper.GetString("peer.discovery.rootnode"),",")
	staticDisc.random      = rand.New(rand.NewSource(time.Now().Unix()))
	staticDisc.isValidator = isValidator
	return &staticDisc
}

func (sd *StaticDiscovery) GetRootNode() (string, error) {
	if sd.isValidator {
		return sd.rootNodes[0], nil
	}
	return sd.rootNodes[sd.random.Int() % (len(sd.rootNodes))], nil
}

