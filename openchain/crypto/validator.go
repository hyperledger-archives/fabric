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

// RegisterValidator registers a client to the PKI infrastructure
func RegisterValidator(id string, pwd []byte, enrollID, enrollPWD string) error {
	mutex.Lock()
	defer mutex.Unlock()

	log.Info("Registering [", enrollID, "] with id [", id, "]...")

	if validators[id] != nil {
		log.Info("Registering [", enrollID, "] with id [", id, "]...done. Already initialized.")
		return nil
	}

	validator := new(validatorImpl)
	if err := validator.register(id, pwd, enrollID, enrollPWD); err != nil {
		log.Error("Failed registering [", enrollID, "] with id [", id, "] [%s].", err.Error())

		if err != utils.ErrAlreadyRegistered && err != utils.ErrAlreadyInitialized  {
			return err
		}
	}
	err := validator.close()
	if err != nil {
		// It is not necessary to report this error to the caller
		log.Error("Registering [", enrollID, "] with id [", id, "]. Failed closing [%s].", err.Error())
	}

	log.Info("Registering [", enrollID, "] with id [", id, "]...done!")

	return nil
}

// InitValidator initializes a client named name with password pwd
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
		log.Error("Failed initialization [%s]: ", id, err)

		return nil, err
	}

	validators[id] = validator
	log.Info("Initializing [%s]...done!", id)

	return validator, nil
}

// CloseValidator releases all the resources allocated by the validator
func CloseValidator(peer Peer) error {
	mutex.Lock()
	defer mutex.Unlock()

	return closeValidatorInternal(peer)
}

// CloseAllValidators closes all the validators initialized so far
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

	log.Info("Closing validator [%s]...done! [%s].", id, err)

	return err
}
