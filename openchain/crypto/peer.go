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

// RegisterPeer registers a peer to the PKI infrastructure
func RegisterPeer(name string, pwd []byte, enrollID, enrollPWD string) error {
	peerMutex.Lock()
	defer peerMutex.Unlock()

	log.Info("Registering peer [%s] with id [%s]...", enrollID, name)

	if peers[name] != nil {
		log.Info("Registering peer [%s] with id [%s]...done. Already initialized.", enrollID, name)

		return nil
	}

	peer := new(peerImpl)
	if err := peer.register("peer", name, pwd, enrollID, enrollPWD); err != nil {
		if err != utils.ErrAlreadyRegistered && err != utils.ErrAlreadyInitialized  {
			log.Error("Failed registering peer [%s] with id [%s] [%s].", enrollID, name, err)
			return err
		}
		log.Info("Registering peer [%s] with id [%s]...done. Already registered or initiliazed.", enrollID, name)
	}
	err := peer.close()
	if err != nil {
		// It is not necessary to report this error to the caller
		log.Warning("Registering peer [%s] with id [%s]. Failed closing [%s].", enrollID, name, err)
	}

	log.Info("Registering peer [%s] with id [%s]...done!", enrollID, name)

	return nil
}

// InitPeer initializes a peer named name with password pwd
func InitPeer(name string, pwd []byte) (Peer, error) {
	peerMutex.Lock()
	defer peerMutex.Unlock()

	log.Info("Initializing peer [%s]...", name)

	if peers[name] != nil {
		log.Info("Peer already initiliazied [%s].", name)

		return peers[name], nil
	}

	peer := new(peerImpl)
	if err := peer.init("peer", name, pwd); err != nil {
		log.Error("Failed peer initialization [%s]: [%s]", name, err)

		return nil, err
	}

	peers[name] = peer
	log.Info("Initializing peer [%s]...done!", name)

	return peer, nil
}

// ClosePeer releases all the resources allocated by peers
func ClosePeer(peer Peer) error {
	peerMutex.Lock()
	defer peerMutex.Unlock()

	return closePeerInternal(peer)
}

// CloseAllPeers closes all the peers initialized so far
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

	log.Info("Closing peer [%s]...done! [%s].", id, err)

	return err
}
