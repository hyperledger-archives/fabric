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
	"github.com/op/go-logging"
	"github.com/openblockchain/obc-peer/openchain/crypto"
	"github.com/openblockchain/obc-peer/openchain/crypto/utils"
	"sync"
)

// Private Variables

var (
	// Map of initialized peers
	peers = make(map[string]crypto.Peer)

	// Sync
	mutex sync.Mutex
)

// Log

var log = logging.MustGetLogger("CRYPTO.PEER")

// Public Methods

// Register registers a client to the PKI infrastructure
func Register(id string, pwd []byte, enrollID, enrollPWD string) error {
	mutex.Lock()
	defer mutex.Unlock()

	log.Info("Registering [%s] with id [%s]...", enrollID, id)

	if peers[id] != nil {
		log.Info("Registering [%s] with id [%s]...done. Already initialized.", enrollID, id)
		return nil
	}

	peer := new(peerImpl)
	if err := peer.Register(id, pwd, enrollID, enrollPWD); err != nil {
		log.Error("Failed registering [%s] with id [%s]: %s", enrollID, id, err)

		return err
	}
	err := peer.Close()
	if err != nil {
		// It is not necessary to report this error to the caller
		log.Error("Registering [%s] with id [%s], failed closing: %s", enrollID, id, err)
	}

	log.Info("Registering [%s] with id [%s]...done!", enrollID, id)

	return nil
}

// Init initializes a client named name with password pwd
func Init(id string, pwd []byte) (crypto.Peer, error) {
	mutex.Lock()
	defer mutex.Unlock()

	log.Info("Initializing [%s]...", id)

	if peers[id] != nil {
		log.Info("Validator already initiliazied [%s].", id)

		return peers[id], nil
	}

	peer := new(peerImpl)
	if err := peer.Init(id, pwd); err != nil {
		log.Error("Failed initialization [%s]: %s", id, err)

		return nil, err
	}

	peers[id] = peer
	log.Info("Initializing [%s]...done!", id)

	return peer, nil
}

// Close releases all the resources allocated by clients
func Close(peer crypto.Peer) error {
	mutex.Lock()
	defer mutex.Unlock()

	return closeInternal(peer)
}

// CloseAll closes all the clients initialized so far
func CloseAll() (bool, []error) {
	mutex.Lock()
	defer mutex.Unlock()

	log.Info("Closing all peers...")

	errs := make([]error, len(peers))
	for _, value := range peers {
		err := closeInternal(value)

		errs = append(errs, err)
	}

	log.Info("Closing all peers...done!")

	return len(errs) != 0, errs
}

// Private Methods

func closeInternal(peer crypto.Peer) error {
	id := peer.GetName()
	log.Info("Closing peer [%s]...", id)
	if _, ok := peers[id]; !ok {
		return utils.ErrInvalidReference
	}
	defer delete(peers, id)

	err := peers[id].(*peerImpl).Close()

	log.Info("Closing peer [%s]...done! [%s]", id, err)

	return err
}
