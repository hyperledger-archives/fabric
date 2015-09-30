package state

import (
	"bytes"
	"github.com/openblockchain/obc-peer/openchain/db"
	"github.com/tecbot/gorocksdb"
	"golang.org/x/crypto/sha3"
)

type stateHash struct {
	globalHash  []byte
	updatedHash map[string][]byte
}

func (statehash *stateHash) addChangesForPersistence(writeBatch *gorocksdb.WriteBatch) {
	openchainDB := db.GetDBHandle()	
	for contractId, updatedhash := range statehash.updatedHash {
		writeBatch.PutCF(openchainDB.StateHashCF, encodeStateHashDBKey(contractId), updatedhash)
	}
}

func computeStateHash(updatedContracts map[string]*contractState) (*stateHash, error) {
	openchainDB := db.GetDBHandle()
	statehash := &stateHash{nil, make(map[string][]byte)}
	for contractId, updatedContract := range updatedContracts {
		updatedHash, err := computeContractHash(updatedContract)
		if err != nil {
			return nil, err
		}
		statehash.updatedHash[contractId] = updatedHash
	}
	itr := openchainDB.GetStateHashCFIterator()
	defer itr.Close()
	var buffer bytes.Buffer

	for ; itr.Valid(); itr.Next() {
		contractId := decodeStateHashDBKey(itr.Key().Data())
		contractHash := statehash.updatedHash[contractId]
		if contractHash == nil {
			contractHash = itr.Value().Data()
		}
		buffer.Write(contractHash)
	}
	hash := make([]byte, 64)
	sha3.ShakeSum256(hash, buffer.Bytes())
	statehash.globalHash = hash
	return statehash, nil
}

func computeContractHash(changedContract *contractState) ([]byte, error) {
	openchainDB := db.GetDBHandle()
	var buffer bytes.Buffer
	buffer.Write([]byte(changedContract.contractId))
	itr := openchainDB.GetStateCFIterator()
	defer itr.Close()
	itr.Seek(buildLowestStateDBKey(changedContract.contractId))
	for ; itr.Valid(); itr.Next() {
		contractId, key := decodeStateDBKey(itr.Key().Data())
		if contractId != changedContract.contractId {
			break
		}
		updatedValue := changedContract.updatedStateMap[key]
		if updatedValue != nil && updatedValue.isDelete() {
			continue
		}
		buffer.Write([]byte(key))
		var value []byte
		if updatedValue != nil {
			value = updatedValue.value
		} else {
			value = itr.Value().Data()
		}
		buffer.Write(value)
	}
	hash := make([]byte, 64)
	sha3.ShakeSum256(hash, buffer.Bytes())
	return hash, nil
}

func encodeStateHashDBKey(contractId string) []byte {
	return []byte(contractId)
}

func decodeStateHashDBKey(dbKey []byte) string {
	return string(dbKey)
}
