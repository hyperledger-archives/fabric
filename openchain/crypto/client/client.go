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
	clients map[string]crypto.Client = make(map[string]crypto.Client)

	// Sync
	mutex sync.Mutex
)

// Log

var log = logging.MustGetLogger("CRYPTO.CLIENT")

// Public Methods

func Register(id string, pwd []byte, enrollID, enrollPWD string) error {
	mutex.Lock()
	defer mutex.Unlock()

	log.Info("Registering [%s] with id [%s]...", enrollID, id)

	if clients[id] != nil {
		log.Info("Registering [%s] with id [%s]...done. Already initialized.", enrollID, id)
		return nil
	}

	client := new(clientImpl)
	if err := client.Register(id, pwd, enrollID, enrollPWD); err != nil {
		log.Error("Failed registering [%s] with id [%s]: %s", enrollID, id, err)

		return err
	}
	err := client.Close()
	if err != nil {
		// It is not necessary to report this error to the caller
		log.Error("Registering [%s] with id [%s], failed closing: %s", enrollID, id, err)
	}

	log.Info("Registering [%s] with id [%s]...done!", enrollID, id)

	return nil
}

func Init(id string, pwd []byte) (crypto.Client, error) {
	mutex.Lock()
	defer mutex.Unlock()

	log.Info("Initializing [%s]...", id)

	if clients[id] != nil {
		log.Info("Client already initiliazied [%s].", id)

		return clients[id], nil
	}

	client := new(clientImpl)
	if err := client.Init(id, pwd); err != nil {
		log.Error("Failed initialization [%s]: %s", id, err)

		return nil, err
	}

	clients[id] = client
	log.Info("Initializing [%s]...done!", id)

	return client, nil
}

func Close(client crypto.Client) error {
	mutex.Lock()
	defer mutex.Unlock()

	return closeInternal(client)
}

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
