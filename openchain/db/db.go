package db

import (
	"fmt"
	"github.com/tecbot/gorocksdb"
	"os"
)

const blockchainCF = "blockchainCF"
const stateCF = "stateCF"
const statehashCF = "statehashCF"

var columnfamilies = []string{blockchainCF, stateCF, statehashCF}

type OpenchainDB struct {
	DB           *gorocksdb.DB
	BlockchainCF *gorocksdb.ColumnFamilyHandle
	StateCF      *gorocksdb.ColumnFamilyHandle
	StateHashCF  *gorocksdb.ColumnFamilyHandle
}

var openchainDB *OpenchainDB
var isOpen bool

func GetDBHandle() (*OpenchainDB) {
	var err error
	if isOpen {
		return openchainDB
	}
	openchainDB, err = open(getDBPath())
	if err != nil {
		panic("Could not open openchain db: " + err.Error())
	}
	return openchainDB
}

func getDBPath() string {
	// get from configuration
	return os.TempDir() + "/openchainDB"
}

func CreateDB(dbPath string) error {
	opts := gorocksdb.NewDefaultOptions()
	opts.SetCreateIfMissing(true)
	var err error
	db, err := gorocksdb.OpenDb(opts, dbPath)
	defer db.Close()
	if err != nil {
		fmt.Println("Error opening DB", err)
		return err
	}

	for _, cf := range columnfamilies {
		_, err = db.CreateColumnFamily(opts, cf)
		if err != nil {
			fmt.Println("Error creating CF:", cf, err)
			return err
		}
	}
	return nil
}

func open(dbPath string) (*OpenchainDB, error) {
	if isOpen {
		return openchainDB, nil
	}
	opts := gorocksdb.NewDefaultOptions()
	opts.SetCreateIfMissing(false)
	db, cfHandlers, err := gorocksdb.OpenDbColumnFamilies(opts, dbPath,
		[]string{"default", blockchainCF, stateCF, statehashCF},
		[]*gorocksdb.Options{opts, opts, opts, opts})

	if err != nil {
		fmt.Println("Error opening DB", err)
		return nil, err
	}
	isOpen = true
	return &OpenchainDB{db, cfHandlers[1], cfHandlers[2], cfHandlers[3]}, nil
}

func (openchainDB *OpenchainDB) GetFromBlockChainCF(key []byte) ([]byte, error) {
	return openchainDB.get(openchainDB.BlockchainCF, key)
}

func (openchainDB *OpenchainDB) GetFromStateCF(key []byte) ([]byte, error) {
	return openchainDB.get(openchainDB.StateCF, key)
}

func (openchainDB *OpenchainDB) GetFromStateHashCF(key []byte) ([]byte, error) {
	return openchainDB.get(openchainDB.StateHashCF, key)
}

func (openchainDB *OpenchainDB) GetBlockChainCFIterator() *gorocksdb.Iterator {
	return openchainDB.getIterator(openchainDB.BlockchainCF)
}

func (openchainDB *OpenchainDB) GetStateCFIterator() *gorocksdb.Iterator {
	return openchainDB.getIterator(openchainDB.StateCF)
}

func (openchainDB *OpenchainDB) GetStateHashCFIterator() *gorocksdb.Iterator {
	return openchainDB.getIterator(openchainDB.StateHashCF)
}

func (openchainDB *OpenchainDB) get(cfHandler *gorocksdb.ColumnFamilyHandle, key []byte) ([]byte, error) {
	opt := gorocksdb.NewDefaultReadOptions()
	slice, err := openchainDB.DB.GetCF(opt, cfHandler, key)
	if err != nil {
		fmt.Println("Error while trying to retrieve key:", key)
		return nil, err
	} else {
		return slice.Data(), nil
	}
}

func (openchainDB *OpenchainDB) getIterator(cfHandler *gorocksdb.ColumnFamilyHandle) *gorocksdb.Iterator {
	opt := gorocksdb.NewDefaultReadOptions()
	return openchainDB.DB.NewIteratorCF(opt, cfHandler)
}
