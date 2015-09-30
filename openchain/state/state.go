package state

import (
	"bytes"
	"github.com/openblockchain/obc-peer/openchain/db"
	"github.com/tecbot/gorocksdb"
)

type State struct {
	contractStateMap map[string]*contractState
	statehash        *stateHash
}

var stateInstance *State

func GetState() *State {
	if stateInstance == nil {
		state := new(State)
		state.contractStateMap = make(map[string]*contractState)
	}
	return stateInstance
}

func (state *State) Get(contractId string, key string) ([]byte, error) {
	updatedValue := state.contractStateMap[contractId].Get(key)
	if updatedValue != nil {
		return updatedValue.value, nil
	} else {
		return state.fetchStateFromDB(contractId, key)
	}
}

func (state *State) Set(contractId string, key string, value []byte) error {
	contractState := getOrCreateContractState(contractId)
	contractState.Set(key, value)
	return nil
}

func (state *State) Delete(contractId string, key string) error {
	contractState := getOrCreateContractState(contractId)
	contractState.Delete(key)
	return nil
}

func (state *State) GetUpdates() map[string]*contractState {
	return state.contractStateMap
}

func (state *State) GetStateHash() ([]byte, error) {
	if state.statehash == nil {
		hash, err := computeStateHash(state.contractStateMap)
		if err != nil {
			return nil, err
		}
		state.statehash = hash
	}
	return state.statehash.globalHash, nil
}

func (state *State) addChangesForPersistence(writeBatch *gorocksdb.WriteBatch) {
	openChainDB := db.GetDBHandle()
	for _, contractState := range state.contractStateMap {
		for key, updatedValue := range contractState.updatedStateMap {
			dbKey := encodeStateDBKey(contractState.contractId, key)
			value := updatedValue.value
			if value != nil {
				writeBatch.PutCF(openChainDB.StateCF, dbKey, value)
			} else {
				writeBatch.DeleteCF(openChainDB.StateCF, dbKey)
			}
		}
	}
	state.statehash.addChangesForPersistence(writeBatch)
}

func (state *State) ClearInMemoryChanges() {
	state.contractStateMap = make(map[string]*contractState)
	state.statehash = nil
}

func getOrCreateContractState(contractId string) *contractState {
	contractState := stateInstance.contractStateMap[contractId]
	if contractState == nil {
		contractState = NewContractState(contractId)
		stateInstance.contractStateMap[contractId] = contractState
	}
	return contractState
}

func (state *State) fetchStateFromDB(contractId string, key string) ([]byte, error) {
	return db.GetDBHandle().GetFromStateCF(encodeStateDBKey(contractId, key))
}

func encodeStateDBKey(contractId string, key string) []byte {
	retKey := []byte(contractId)
	retKey = append(retKey, stateKeyDelimiter...)
	keybytes := ([]byte(key))
	retKey = append(retKey, keybytes...)
	return retKey
}

func decodeStateDBKey(dbKey []byte) (string, string) {
	split := bytes.Split(dbKey, stateKeyDelimiter)
	return string(split[0]), string(split[1])
}

func buildLowestStateDBKey(contractId string) []byte {
	retKey := []byte(contractId)
	retKey = append(retKey, stateKeyDelimiter...)
	return retKey
}

var stateKeyDelimiter = []byte{0x00}

type valueHolder struct {
	value []byte
}

func (valueHolder *valueHolder) isDelete() bool {
	return valueHolder.value == nil
}

type contractState struct {
	contractId      string
	updatedStateMap map[string]*valueHolder
}

func NewContractState(contractId string) *contractState {
	return &contractState{contractId, make(map[string]*valueHolder)}
}

func (contractState *contractState) Get(key string) *valueHolder {
	return contractState.updatedStateMap[key]
}

func (contractState *contractState) Set(key string, value []byte) {
	contractState.updatedStateMap[key] = &valueHolder{value}
}

func (contractState *contractState) Delete(key string) {
	contractState.updatedStateMap[key] = &valueHolder{nil}
}

func (contractState *contractState) Changed() bool {
	return len(contractState.updatedStateMap) > 0
}
