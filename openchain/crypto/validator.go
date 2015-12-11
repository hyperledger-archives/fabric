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
	// Map of initialized validators
	validators = make(map[string]Peer)

	// Sync
	mutex sync.Mutex
)

// Public Methods

// Register registers a client to the PKI infrastructure
func RegisterValidator(id string, pwd []byte, enrollID, enrollPWD string) error {
	mutex.Lock()
	defer mutex.Unlock()

	log.Info("Registering [%s] with id [%s]...", enrollID, id)

	if validators[id] != nil {
		log.Info("Registering [%s] with id [%s]...done. Already initialized.", enrollID, id)
		return nil
	}

	validator := new(validatorImpl)
	if err := validator.register(id, pwd, enrollID, enrollPWD); err != nil {
		log.Error("Failed registering [%s] with id [%s]: %s", enrollID, id, err)

		return err
	}
	err := validator.close()
	if err != nil {
		// It is not necessary to report this error to the caller
		log.Error("Registering [%s] with id [%s], failed closing: %s", enrollID, id, err)
	}

	log.Info("Registering [%s] with id [%s]...done!", enrollID, id)

	return nil
}

// Init initializes a client named name with password pwd
func InitValidator(id string, pwd []byte) (Peer, error) {
	mutex.Lock()
	defer mutex.Unlock()

	log.Info("Initializing [%s]...", id)

	if validators[id] != nil {
		log.Info("Validator already initiliazied [%s].", id)

		return validators[id], nil
	}

	validator := new(validatorImpl)
	if err := validator.init(id, pwd); err != nil {
		log.Error("Failed initialization [%s]: %s", id, err)

		return nil, err
	}

	validators[id] = validator
	log.Info("Initializing [%s]...done!", id)

	return validator, nil
}

// Close releases all the resources allocated by clients
func CloseValidator(peer Peer) error {
	mutex.Lock()
	defer mutex.Unlock()

	return closeValidatorInternal(peer)
}

// CloseAll closes all the clients initialized so far
func CloseAllValidators() (bool, []error) {
	mutex.Lock()
	defer mutex.Unlock()

	log.Info("Closing all validators...")

	errs := make([]error, len(validators))
	for _, value := range validators {
		err := closeValidatorInternal(value)

		errs = append(errs, err)
	}

	log.Info("Closing all validators...done!")

	return len(errs) != 0, errs
}

// Private Methods

func closeValidatorInternal(peer Peer) error {
	id := peer.GetName()
	log.Info("Closing validator [%s]...", id)
	if _, ok := validators[id]; !ok {
		return utils.ErrInvalidReference
	}
	defer delete(validators, id)

	err := validators[id].(*validatorImpl).close()

	log.Info("Closing validator [%s]...done! [%s]", id, err)

	return err
}
