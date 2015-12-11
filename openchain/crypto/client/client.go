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

package client

import (
	"github.com/op/go-logging"
	"github.com/openblockchain/obc-peer/openchain/crypto"
	"github.com/openblockchain/obc-peer/openchain/crypto/utils"
	"sync"
)

// Private Variables

var (
	// Map of initialized clients
	clients = make(map[string]crypto.Client)

	// Sync
	mutex sync.Mutex
)

// Log

var log = logging.MustGetLogger("CRYPTO.CLIENT")

// Public Methods

// Register registers a client to the PKI infrastructure
func Register(name string, pwd []byte, enrollID, enrollPWD string) error {
	mutex.Lock()
	defer mutex.Unlock()

	log.Info("Registering [%s] with id [%s]...", enrollID, name)

	if clients[name] != nil {
		log.Info("Registering [%s] with id [%s]...done. Already initialized.", enrollID, name)
		return nil
	}

	client := new(clientImpl)
	if err := client.Register(name, pwd, enrollID, enrollPWD); err != nil {
		log.Error("Failed registering [%s] with id [%s]: %s", enrollID, name, err)

		return err
	}
	err := client.Close()
	if err != nil {
		// It is not necessary to report this error to the caller
		log.Error("Registering [%s] with id [%s], failed closing: %s", enrollID, name, err)
	}

	log.Info("Registering [%s] with id [%s]...done!", enrollID, name)

	return nil
}

// Init initializes a client named name with password pwd
func Init(name string, pwd []byte) (crypto.Client, error) {
	mutex.Lock()
	defer mutex.Unlock()

	log.Info("Initializing [%s]...", name)

	if clients[name] != nil {
		log.Info("Client already initiliazied [%s].", name)

		return clients[name], nil
	}

	client := new(clientImpl)
	if err := client.Init(name, pwd); err != nil {
		log.Error("Failed initialization [%s]: %s", name, err)

		return nil, err
	}

	clients[name] = client
	log.Info("Initializing [%s]...done!", name)

	return client, nil
}

// Close releases all the resources allocated by clients
func Close(client crypto.Client) error {
	mutex.Lock()
	defer mutex.Unlock()

	return closeInternal(client)
}

// CloseAll closes all the clients initialized so far
func CloseAll() (bool, []error) {
	mutex.Lock()
	defer mutex.Unlock()

	log.Info("Closing all clients...")

	errs := make([]error, len(clients))
	for _, value := range clients {
		err := closeInternal(value)

		errs = append(errs, err)
	}

	log.Info("Closing all clients...done!")

	return len(errs) != 0, errs
}

// Private Methods

func closeInternal(client crypto.Client) error {
	id := client.GetName()
	log.Info("Closing client [%s]...", id)
	if _, ok := clients[id]; !ok {
		return utils.ErrInvalidReference
	}
	defer delete(clients, id)

	err := clients[id].(*clientImpl).Close()

	log.Info("Closing client [%s]...done! [%s]", id, err)

	return err
}
