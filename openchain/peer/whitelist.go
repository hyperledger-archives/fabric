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

	"github.com/openblockchain/obc-peer/openchain/util"
	pb "github.com/openblockchain/obc-peer/protos"

	"github.com/golang/protobuf/proto"
)

const whitelistFile = "/tmp/whitelist.dat"

// Gatekeeper is used to manage the list of validating peers a validator should connect to
type Gatekeeper interface {
	CheckWhitelistExists() (size int, err error)
	LoadWhitelist() (err error)
	SaveWhitelist() error
	GetWhitelist() (whitelistedMap map[pb.PeerID]int, sortedValues []*pb.PeerID, err error)
	GetWhitelistCap() (cap int, err error)
	SetWhitelistCap(cap int) error
}

// CheckWhitelistExists returns the length (number of entries) of this peer's whitelist
func (p *PeerImpl) CheckWhitelistExists() (size int, err error) {
	return len(p.whitelist.SortedKeys), nil
}

// LoadWhitelist loads the validator's whitelist of validating peers from disk into memory
// Sets p.whitelist and p.whitelistedMap
func (p *PeerImpl) LoadWhitelist() error {
	data, err := util.LoadFromDisk(whitelistFile)
	if err != nil {
		return err
	}

	err = proto.Unmarshal(data, p.whitelist)
	if err != nil {
		return fmt.Errorf("Unable to unmarshal whitelist: %v", err)
	}

	peerLogger.Debug("Whitelist now reads: %+v", p.whitelist)
	p.whitelistedMap = createMapFromWhitelist(p.whitelist)

	return err
}

// SaveWhitelist stores to disk the list of validating peers this validator should connect to
// Is called when the handlerMap is locked by RegisterHandler
func (p *PeerImpl) SaveWhitelist() error {
	// filter the handlerMap structure for connected VPs
	handlerMapVP := filterHandlerMapForVP(p.handlerMap.m)

	// TODO Fix *potential* race condition; *may* need to SetWhitelistCap()
	//      before RegisterHandler() registers the (N-1)-th connection
	if (p.whitelist.Cap == -1) || (int32(len(handlerMapVP)) < (p.whitelist.Cap - 1)) {
		return nil
	}

	peerLogger.Debug("Handler map now has %d VP connections, time to persist the whitelist...", len(handlerMapVP))

	// order the filtered map to generate IDs
	ownEndpoint, _ := p.GetPeerEndpoint()
	p.whitelist.SortedKeys, p.whitelist.SortedValues = sortHandlerMapForVP(handlerMapVP, ownEndpoint.GetID())
	peerLogger.Debug("Whitelist reads: %+v", p.whitelist)
	p.whitelistedMap = createMapFromWhitelist(p.whitelist)

	data, err := proto.Marshal(p.whitelist)
	if err != nil {
		return fmt.Errorf("Unable to marshal whitelist: %v", err)
	}

	err = util.SaveToDisk(whitelistFile, data)
	if err != nil {
		return err
	}

	return nil
}

// GetWhitelist retrieves the sorted list of PeerID.Names and PeerIDs of the validating peers in the whitelist
func (p *PeerImpl) GetWhitelist() (whitelistedMap map[pb.PeerID]int, sortedValues []*pb.PeerID, err error) {
	whitelistedMap = p.whitelistedMap
	sortedValues = p.whitelist.GetSortedValues()
	return
}

// GetWhitelistCap allows the consensus plugin to get the expected number of maximum validators on the network
func (p *PeerImpl) GetWhitelistCap() (cap int, err error) {
	return int(p.whitelist.Cap), nil
}

// SetWhitelistCap allows the consensus plugin to set the expected number of maximum validators on the network
func (p *PeerImpl) SetWhitelistCap(cap int) error {
	p.whitelist.Cap = int32(cap)
	peerLogger.Debug("Whitelist cap set to: %d", p.whitelist.Cap)
	return nil
}

// filterHandlerMapForVP filters this peer's handlerMap for connected validating peers
// Is called by SaveWhitelist
func filterHandlerMapForVP(inMap map[pb.PeerID]MessageHandler) (outMap map[*pb.PeerID]*MessageHandler) {
	outMap = make(map[*pb.PeerID]*MessageHandler)
	for k, v := range inMap {
		ep, err := v.To()
		if err != nil {
			peerLogger.Debug("Error retrieving endpoint for handler %v", k)
			continue
		}
		peerLogger.Debug("Current endpoint under inspection: %+v", ep)
		if ep.Type == pb.PeerEndpoint_VALIDATOR {
			key := k
			value := v
			outMap[&key] = &value
		}
	}
	return
}

// sortHandlerMapForVP adds our own PeerID to the list and returns an alphabetically sorted slice of VP keys
func sortHandlerMapForVP(inMap map[*pb.PeerID]*MessageHandler, ownPeerID *pb.PeerID) (sortedKeys []string, sortedValues []*pb.PeerID) {
	sortedKeys = make([]string, len(inMap)+1)
	sortedValues = make([]*pb.PeerID, len(inMap)+1)

	i := 0
	for k := range inMap {
		sortedKeys[i] = k.Name
		i++
	}
	sortedKeys[i] = ownPeerID.Name
	sort.Strings(sortedKeys)

	for k, v := range sortedKeys {
		if v == ownPeerID.Name {
			sortedValues[k] = ownPeerID
		} else {
			for j := range inMap {
				if v == j.Name {
					temp := j
					sortedValues[k] = temp
				}
			}
		}
	}

	peerLogger.Debug("Sorted keys: %+v", sortedKeys)
	peerLogger.Debug("Sorted values: %+v", sortedValues)

	return
}

// createMapFromWhitelist builds a map holding whitelisted pb.PeerIDs for easy 'comma OK' checks
func createMapFromWhitelist(inList *pb.Whitelist) (outMap map[pb.PeerID]int) {
	outMap = make(map[pb.PeerID]int)
	for k, v := range inList.GetSortedValues() {
		temp := v
		outMap[*temp] = k
	}

	peerLogger.Debug("Whitelisted map now reads: %+v", outMap)
	return
}
