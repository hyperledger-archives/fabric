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
	"github.com/hyperledger/fabric/core/crypto/utils"
	"sync"
	"time"
)

// Private Variables

type clientEntry struct {
	client     Client
	counter    int64
	timestamp  time.Time
	longLiving bool
}

var (
	// Is client module initialized
	initialized bool
	done        chan struct{}

	// Map of initialized clients
	clients = make(map[string]clientEntry)

	// Sync
	clientMutex sync.Mutex
)

// Public Methods

// RegisterClient registers a client to the PKI infrastructure
func RegisterClient(name string, pwd []byte, enrollID, enrollPWD string) error {
	clientMutex.Lock()
	defer clientMutex.Unlock()

	// Init the client layer if necessary
	initClientLayer()

	log.Info("Registering client [%s] with name [%s]...", enrollID, name)

	if _, ok := clients[name]; ok {
		log.Info("Registering client [%s] with name [%s]...done. Already initialized.", enrollID, name)

		return nil
	}

	client := newClient()
	if err := client.register(name, pwd, enrollID, enrollPWD); err != nil {
		if err != utils.ErrAlreadyRegistered && err != utils.ErrAlreadyInitialized {
			log.Error("Failed registering client [%s] with name [%s] [%s].", enrollID, name, err)
			return err
		}
		log.Info("Registering client [%s] with name [%s]...done. Already registered or initiliazed.", enrollID, name)
	}
	err := client.close()
	if err != nil {
		// It is not necessary to report this error to the caller
		log.Warning("Registering client [%s] with name [%s]. Failed closing [%s].", enrollID, name, err)
	}

	log.Info("Registering client [%s] with name [%s]...done!", enrollID, name)

	return nil
}

// InitShortLivingClient initializes a short living client named 'name' using password 'pwd'.
// A short-living client is a client that is supposed to be initialize every time it is needed.
// The client is used (for a short period, namely to just invoke a single function) and then discarded immediately.
// There is no need to call CloseClient for the short-living instances, internal garbage collection is in place.
func InitShortLivingClient(name string, pwd []byte) (Client, error) {
	clientMutex.Lock()
	defer clientMutex.Unlock()

	// Init the client layer if necessary
	initClientLayer()

	log.Info("Initializing client [%s]...", name)

	if entry, ok := clients[name]; ok {
		log.Info("Client already initiliazied [%s]. Increasing counter from [%d]", name, clients[name].counter)
		entry.counter++
		entry.timestamp = time.Now()
		clients[name] = entry

		return clients[name].client, nil
	}

	client := newClient()
	if err := client.init(name, pwd); err != nil {
		log.Error("Failed client initialization [%s]: [%s].", name, err)

		return nil, err
	}

	clients[name] = clientEntry{client, 1, time.Now(), false}
	log.Info("Initializing client [%s]...done!", name)

	return client, nil
}

// InitClient initializes a long living client named 'name' using password 'pwd'.
// A short-living client is a client that is supposed to be initialize one and stay active for an indefinite period
// of time.
// Once the client is not needed anymore, CloseClient must be explicitly invoked. No internal garbage collection is
// applied to long-living instances.
func InitClient(name string, pwd []byte) (Client, error) {
	clientMutex.Lock()
	defer clientMutex.Unlock()

	// Init the client layer if necessary
	initClientLayer()

	log.Info("Initializing client [%s]...", name)

	if entry, ok := clients[name]; ok {
		log.Info("Client already initiliazied [%s]. Increasing counter from [%d]", name, clients[name].counter)
		entry.counter++
		entry.timestamp = time.Now()
		clients[name] = entry

		return clients[name].client, nil
	}

	client := newClient()
	if err := client.init(name, pwd); err != nil {
		log.Error("Failed client initialization [%s]: [%s].", name, err)

		return nil, err
	}

	clients[name] = clientEntry{client, 1, time.Now(), true}
	log.Info("Initializing client [%s]...done!", name)

	return client, nil
}

// CloseClient releases all the resources allocated by clients
func CloseClient(client Client) error {
	clientMutex.Lock()
	defer clientMutex.Unlock()

	return closeClientInternal(client, false)
}

// CloseAllClients closes all the clients (short and long-living) initialized so far
func CloseAllClients() (bool, []error) {
	clientMutex.Lock()
	defer clientMutex.Unlock()

	// Send the 'done' signal
	done <- struct{}{}

	// Close all the clients
	log.Info("Closing all clients...")

	errs := make([]error, len(clients))
	for _, value := range clients {
		err := closeClientInternal(value.client, true)

		errs = append(errs, err)
	}

	log.Info("Closing all clients...done!")

	// Reset
	initialized = false

	return len(errs) != 0, errs
}

// Private Methods

func newClient() *clientImpl {
	return &clientImpl{&nodeImpl{}, false, nil, nil, nil, nil}
}

func closeClientInternal(client Client, force bool) error {
	if client == nil {
		return utils.ErrNilArgument
	}

	name := client.GetName()
	log.Info("Closing client [%s]...", name)
	entry, ok := clients[name]
	if !ok {
		return utils.ErrInvalidReference
	}
	if entry.counter == 1 || force {
		defer delete(clients, name)
		err := clients[name].client.(*clientImpl).close()
		log.Debug("Closing client [%s]...cleanup! [%s].", name, utils.ErrToString(err))

		return err
	}

	// decrease counter
	entry.counter--
	clients[name] = entry
	log.Debug("Closing client [%s]...decreased counter at [%d].", name, clients[name].counter)

	return nil
}

func initClientLayer() {
	if !initialized {
		log.Debug("Initilize client layer...")
		done = make(chan struct{})

		// Start client instances cleaner
		go clientInstancesCleaner()

		initialized = true
		log.Debug("Initilize client layer...")
	} else {
		log.Debug("Client layer already initialized.")
	}

}

func clientInstancesCleaner() {
	log.Debug("Client Instanes cleaner starting...")

	var terminate bool = false
	for {
		select {
		case <-done:
			terminate = true

		case <-time.After(refreshTimePeriod * time.Minute):
			log.Debug("Time elpased, clean old client instances...")

			cleanOldClientInstances()
		}

		if terminate {
			break
		}
	}
}

func cleanOldClientInstances() {
	clientMutex.Lock()
	defer clientMutex.Unlock()

	now := time.Now()
	log.Debug("Cleaning old client instances [%s]...", now)

	for name, clientEntry := range clients {
		if clientEntry.longLiving {
			continue
		}

		elapsed := now.Sub(clientEntry.timestamp)

		log.Debug("Client Entry [%s], elapsed [%s]", name, elapsed)
		if elapsed.Seconds() >= 1 {
			// This entry must be removed
			log.Debug("Cleaning client [%s]...", name)

			err := closeClientInternal(clientEntry.client, true)
			if err != nil {
				log.Debug("Cleaning client [%s]...done!", name)
			} else {
				log.Error("Failed closing client (clientInstancesCleaner) [%s]", err)
			}
		}
	}
}
