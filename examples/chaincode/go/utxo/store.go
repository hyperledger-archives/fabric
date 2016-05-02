/*
Copyright IBM Corp. 2016 All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

		 http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"fmt"

	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric/examples/chaincode/go/utxo/util"
	"github.com/hyperledger/fabric/core/chaincode/shim"
)

type Store struct {
	stub *shim.ChaincodeStub
}

func MakeChaincodeStore(stub *shim.ChaincodeStub) util.Store {
	store := &Store{}
	store.stub = stub
	return store
}

func keyToString(key *util.Key) string {
	return key.TxHashAsHex + ":" + string(key.TxIndex)
}

func (s *Store) GetState(key util.Key) (*util.TX_TXOUT, bool, error) {
	keyToFetch := keyToString(&key)
	data, err := s.stub.GetState(keyToFetch)
	if err != nil {
		return nil, false, fmt.Errorf("Error getting state from stub:  %s", err)
	}
	if data == nil {
		return nil, false, nil
	}
	// Value found, unmarshal
	var value = &util.TX_TXOUT{}
	err = proto.Unmarshal(data, value)
	if err != nil {
		return nil, false, fmt.Errorf("Error unmarshalling value:  %s", err)
	}
	return value, true, nil
}

func (s *Store) DelState(key util.Key) error {
	return s.stub.DelState(keyToString(&key))
}

func (s *Store) PutState(key util.Key, value *util.TX_TXOUT) error {
	data, err := proto.Marshal(value)
	if err != nil {
		return fmt.Errorf("Error marshalling value to bytes:  %s", err)
	}
	return s.stub.PutState(keyToString(&key), data)
}

func (s *Store) GetTran(key string) ([]byte, bool, error) {
	data, err := s.stub.GetState(key)
	if err != nil {
		return nil, false, fmt.Errorf("Error getting state from stub:  %s", err)
	}
	if data == nil {
		return nil, false, nil
	}
	return data, true, nil
}

func (s *Store) PutTran(key string, value []byte) error {
	return s.stub.PutState(key, value)
}
