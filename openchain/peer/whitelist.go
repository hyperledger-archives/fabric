/*
Licensed to the Apache Software Foundation (ASF) under one
or more contributor license agreements.  See the NOTICE file
distributed with this work for additional information
regarding copyright ownership.  The ASF licenses this file
to you under the Apache License, Version 2.0 (the
"License"); you may not use this file except in compliance
with the License.  You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing,
software distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
KIND, either express or implied.  See the License for the
specific language governing permissions and limitations
under the License.
*/

package peer

import (
	"fmt"
	"sort"
	"strings"

	"github.com/openblockchain/obc-peer/openchain/util"
	pb "github.com/openblockchain/obc-peer/protos"

	"github.com/golang/protobuf/proto"
)

const whitelistFile = "/tmp/whitelist.dat"

// Gatekeeper is used to manage the list of validating peers a validator should connect to
type Gatekeeper interface {
	CheckWhitelistExists() (size int)
	LoadWhitelist() (err error)
	SaveWhitelist() (err error)
	GetWhitelist() (whitelistedMap map[string]int, sortedKeys []string, sortedValues []*pb.PeerID)
	GetWhitelistCap() (cap int)
	SetWhitelistCap(cap int)
}

// CheckWhitelistExists returns the length (number of entries) of this peer's whitelist
func (p *PeerImpl) CheckWhitelistExists() (size int) {
	return len(p.whitelist.SortedKeys)
}

// LoadWhitelist loads the validator's whitelist of validating peers from disk into memory
// Sets p.whitelist and p.whitelistedMap
func (p *PeerImpl) LoadWhitelist() (err error) {
	data, err := util.LoadFromDisk(whitelistFile)
	if err != nil {
		return err
	}

	err = proto.Unmarshal(data, p.whitelist)
	if err != nil {
		return fmt.Errorf("Unable to unmarshal whitelist: %v", err)
	}

	peerLogger.Debug("Whitelist now reads: %+v", p.whitelist)
	p.whitelistedMap = createMapFromWhitelist(p.whitelist.SortedKeys)

	return err
}

// SaveWhitelist stores to disk the list of validating peers this validator should connect to
// Is called when the handlerMap is locked by RegisterHandler
func (p *PeerImpl) SaveWhitelist() (err error) {
	// filter the handlerMap structure for connected VPs
	vpMap := filterHandlers(p.handlerMap.m)

	// TODO Fix *potential* race condition; *may* need to SetWhitelistCap()
	//      before RegisterHandler() registers the (N-1)-th connection
	if p.whitelist.Persisted || (p.whitelist.Cap == -1) || (int32(len(vpMap)) < (p.whitelist.Cap - 1)) {
		return nil
	}

	peerLogger.Debug("Handler map now has %d VP connections, time to persist the whitelist...", len(vpMap))

	// add self to map
	ownEP, _ := p.GetPeerEndpoint()
	vpMap = addSelfToMap(ownEP, vpMap)

	// derive sorted lists
	p.whitelist.SortedKeys, p.whitelist.SortedValues = sortWhitelist(vpMap)

	// then save to memory
	peerLogger.Debug("Whitelist (w. self, sorted) now reads: %+v", p.whitelist)

	// build whitelisted map
	p.whitelistedMap = createMapFromWhitelist(p.whitelist.SortedKeys)

	p.whitelist.Persisted = true

	// marshal whitelist
	data, err := proto.Marshal(p.whitelist)
	if err != nil {
		return fmt.Errorf("Unable to marshal whitelist: %v", err)
	}

	// save to disk
	err = util.SaveToDisk(whitelistFile, data)
	if err != nil {
		return err
	}

	return nil
}

// GetWhitelist retrieves the map and sorted list of whitelisted peer keys
func (p *PeerImpl) GetWhitelist() (whitelistedMap map[string]int, sortedKeys []string, sortedValues []*pb.PeerID) {
	whitelistedMap = p.whitelistedMap
	sortedKeys = p.whitelist.SortedKeys
	sortedValues = p.whitelist.SortedValues
	return
}

// GetWhitelistCap allows the consensus plugin to get the expected number of maximum validators on the network
func (p *PeerImpl) GetWhitelistCap() (cap int) {
	return int(p.whitelist.Cap)
}

// SetWhitelistCap allows the consensus plugin to set the expected number of maximum validators on the network
func (p *PeerImpl) SetWhitelistCap(cap int) {
	p.whitelist.Cap = int32(cap)
	peerLogger.Debug("Whitelist cap set to: %d", p.whitelist.Cap)
}

// filterHandlers filters this peer's handlerMap for connected validating peers
// Is called by SaveWhitelist
func filterHandlers(handlerMap map[pb.PeerID]MessageHandler) (vpMap map[string]*pb.PeerID) {
	vpMap = make(map[string]*pb.PeerID)

	for k, v := range handlerMap {
		ep, err := v.To()
		if err != nil {
			peerLogger.Debug("Error retrieving endpoint for handler %v", k)
			continue
		}
		if ep.Type == pb.PeerEndpoint_VALIDATOR {
			temp := k
			vpMap[ep.GetID().Name] = &temp
		}
	}

	return
}

// addSelfToList adds this peer's key and *PeerID to the map of validating peers
func addSelfToMap(ep *pb.PeerEndpoint, vpMap map[string]*pb.PeerID) map[string]*pb.PeerID {
	vpMap[ep.GetID().Name] = ep.GetID()
	return vpMap
}

// sortWhitelist takes a map, sorts the keys alphabetically, and returns the corresponding values in a separate list as well
func sortWhitelist(vpMap map[string]*pb.PeerID) (sortedKeys []string, sortedValues []*pb.PeerID) {
	sortedKeys = make([]string, len(vpMap))
	sortedValues = make([]*pb.PeerID, len(vpMap))

	// get the keys
	i := 0
	for key := range vpMap {
		sortedKeys[i] = key
		i++
	}

	// sort the keys
	sort.Strings(sortedKeys)

	// sort the values
	for i, key := range sortedKeys {
		for k, v := range vpMap {
			if strings.Compare(key, k) == 0 {
				temp := v
				sortedValues[i] = temp
			}
		}
	}

	return
}

// createMapFromWhitelist builds a map holding whitelisted keys for easy 'comma OK' checks
func createMapFromWhitelist(sortedKeys []string) (whitelistedMap map[string]int) {
	whitelistedMap = make(map[string]int, len(sortedKeys))

	for i, key := range sortedKeys {
		whitelistedMap[key] = i
	}

	peerLogger.Debug("Whitelisted map now reads: %+v", whitelistedMap)
	return
}
