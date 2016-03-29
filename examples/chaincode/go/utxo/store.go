package main

import (
	"fmt"

	"github.com/golang/protobuf/proto"
	"github.com/openblockchain/obc-peer/examples/chaincode/go/utxo/util"
	"github.com/openblockchain/obc-peer/openchain/chaincode/shim"
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
