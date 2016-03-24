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
)

const whitelistFile = "/tmp/whitelist.dat"

// Holds the PeerIDs of whitelisted validating peers
type whitelist struct {
	cap        int
	handlerMap map[*pb.PeerID]*MessageHandler
	index      []string
	order      []*pb.PeerID
}

// Gatekeeper is used to manage the list of validating peers a validator should connect to
type Gatekeeper interface {
	CheckWhitelistExists() (size int, err error)
	LoadWhitelist() (err error)
	SaveWhitelist() error
	GetWhitelistKeys() (index []string, order []*pb.PeerID, err error)
	GetWhitelistCap() (cap int, err error)
	SetWhitelistCap(cap int) error
}

// CheckWhitelistExists returns the length (number of entries) of this peer's whitelist
func (p *PeerImpl) CheckWhitelistExists() (size int, err error) {
	return len(p.whitelist.handlerMap), nil
}

// LoadWhitelist loads the validator's whitelist of validating peers from disk into memory
func (p *PeerImpl) LoadWhitelist() error {
	list := new(whitelist)
	err := util.LoadFromDisk(whitelistFile, list)
	if err != nil {
		return fmt.Errorf("Unable to read whitelist from disk: %v", err)
	}

	peerLogger.Debug("Whitelist (from disk) now reads: %+v", list)
	p.whitelist = list

	return err
}

// SaveWhitelist stores to disk the list of validating peers this validator should connect to
// Is called when the handlerMap is locked by RegisterHandler
func (p *PeerImpl) SaveWhitelist() error {
	// filter the handlerMap structure for connected VPs
	handlerMapVP := filterHandlerMapForVP(p.handlerMap.m)

	// TODO Fix potential race condition; *may* need to SetWhitelistCap()
	//      before RegisterHandler() registers the (N-1)-th connection
	if (p.whitelist.cap == -1) || (len(handlerMapVP) < (p.whitelist.cap - 1)) {
		return nil
	}

	// order the filtered map to generate IDs
	p.whitelist.index, p.whitelist.order = sortHandlerMapForVP(handlerMapVP)

	// set in memory
	p.whitelist.handlerMap = handlerMapVP
	peerLogger.Debug("Whitelist (from memory) reads: %+v", p.whitelist)

	// save to disk
	err := util.SaveToDisk(whitelistFile, p.whitelist)
	if err != nil {
		return fmt.Errorf("Unable to save whitelist to disk: %v", err)
	}

	return nil
}

// GetWhitelistKeys retrieves the sorted list of Peer.ID.Names of the validating peers in the whitelist
func (p *PeerImpl) GetWhitelistKeys() (index []string, order []*pb.PeerID, err error) {
	index = p.whitelist.index
	order = p.whitelist.order
	return
}

// GetWhitelistCap allows the consensus plugin to get the expected number of maximum validators on the network
func (p *PeerImpl) GetWhitelistCap() (cap int, err error) {
	return p.whitelist.cap, nil
}

// SetWhitelistCap allows the consensus plugin to set the expected number of maximum validators on the network
func (p *PeerImpl) SetWhitelistCap(cap int) error {
	p.whitelist.cap = cap
	peerLogger.Debug("Whitelist cap set to: %d", cap)
	return nil
}

// filterHandlerMapForVP filters this peer's handlerMap for connected validating peers
// Is called by SaveWhitelist
func filterHandlerMapForVP(in map[pb.PeerID]MessageHandler) (out map[*pb.PeerID]*MessageHandler) {
	for k, v := range in {
		ep, err := v.To()
		if err != nil {
			peerLogger.Debug("Error retrieving endpoint for handler %v", k)
			continue
		}
		if ep.Type == pb.PeerEndpoint_VALIDATOR {
			out[&k] = &v
		}
	}
	return
}

// sortHandlerMapForVP returns an alphabetically sorted slice of VP keys
func sortHandlerMapForVP(in map[*pb.PeerID]*MessageHandler) (index []string, order []*pb.PeerID) {
	s1 := make([]string, len(in))
	s2 := make([]*pb.PeerID, len(in))

	i := 0
	for k := range in {
		s1[i] = k.Name
		i++
	}
	sort.Strings(s1)

	for k, v := range s1 {
		for j := range in {
			if j.Name == v {
				s2[k] = j
			}
		}
	}

	index = s1
	order = s2

	return
}
