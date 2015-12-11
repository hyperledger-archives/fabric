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

package crypto

import (
	"github.com/openblockchain/obc-peer/openchain/crypto/utils"
	"sync"
)

// Private Variables

var (
	// Map of initialized peers
	peers = make(map[string]Peer)

	// Sync
	peerMutex sync.Mutex
)

// Public Methods

// Register registers a client to the PKI infrastructure
func RegisterPeer(id string, pwd []byte, enrollID, enrollPWD string) error {
	peerMutex.Lock()
	defer peerMutex.Unlock()

	log.Info("Registering [%s] with id [%s]...", enrollID, id)

	if peers[id] != nil {
		log.Info("Registering [%s] with id [%s]...done. Already initialized.", enrollID, id)
		return nil
	}

	peer := new(peerImpl)
	if err := peer.register("peer", id, pwd, enrollID, enrollPWD); err != nil {
		log.Error("Failed registering [%s] with id [%s]: %s", enrollID, id, err)

		return err
	}
	err := peer.close()
	if err != nil {
		// It is not necessary to report this error to the caller
		log.Error("Registering [%s] with id [%s], failed closing: %s", enrollID, id, err)
	}

	log.Info("Registering [%s] with id [%s]...done!", enrollID, id)

	return nil
}

// Init initializes a client named name with password pwd
func InitPeeer(id string, pwd []byte) (Peer, error) {
	peerMutex.Lock()
	defer peerMutex.Unlock()

	log.Info("Initializing [%s]...", id)

	if peers[id] != nil {
		log.Info("Validator already initiliazied [%s].", id)

		return peers[id], nil
	}

	peer := new(peerImpl)
	if err := peer.init("peer", id, pwd); err != nil {
		log.Error("Failed initialization [%s]: %s", id, err)

		return nil, err
	}

	peers[id] = peer
	log.Info("Initializing [%s]...done!", id)

	return peer, nil
}

// Close releases all the resources allocated by clients
func ClosePeer(peer Peer) error {
	peerMutex.Lock()
	defer peerMutex.Unlock()

	return closePeerInternal(peer)
}

// CloseAll closes all the clients initialized so far
func CloseAllPeers() (bool, []error) {
	peerMutex.Lock()
	defer peerMutex.Unlock()

	log.Info("Closing all peers...")

	errs := make([]error, len(peers))
	for _, value := range peers {
		err := closePeerInternal(value)

		errs = append(errs, err)
	}

	log.Info("Closing all peers...done!")

	return len(errs) != 0, errs
}

// Private Methods

func closePeerInternal(peer Peer) error {
	id := peer.GetName()
	log.Info("Closing peer [%s]...", id)
	if _, ok := peers[id]; !ok {
		return utils.ErrInvalidReference
	}
	defer delete(peers, id)

	err := peers[id].(*peerImpl).close()

	log.Info("Closing peer [%s]...done! [%s]", id, err)

	return err
}
