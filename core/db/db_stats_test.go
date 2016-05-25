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

package db

import (
	"fmt"
	"testing"

	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric/protos"
	"github.com/spf13/viper"
	"github.com/tecbot/gorocksdb"
)

//This file contains a test function TestDumpDBStats. This test is not a unit test, rather this is used for printing
// the details of a db-dump pointed to by DBDir. For analysing a db-dump,
// - comment the t.SkipNow(),
// - specify the location of db-dump in DBDir, and
// - execute "go test -run TestDumpDBStats"

const (
	//MaxValueSize is used to compare the size of the key-value in db.
	//If a key-value is more than this size, it's details are printed for further analysis.
	MaxValueSize = 1024 * 1024

	//DBDir the name of the folder that can be given for off-line study of a db dump
	DBDir = ""
)

type detailPrinter func(data []byte)

func TestDumpDBStats(t *testing.T) {
	t.SkipNow()
	if DBDir != "" {
		viper.Set("peer.fileSystemPath", DBDir)
	}
	openchainDB := GetDBHandle()
	defer openchainDB.CloseDB()
	scan(openchainDB, blockchainCF, openchainDB.BlockchainCF, blockDetailPrinter)
	scan(openchainDB, persistCF, openchainDB.PersistCF, nil)
}

func printLiveFilesMetaData(openchainDB *OpenchainDB) {
	fmt.Println("------ Details of LiveFilesMetaData ---")
	db := openchainDB.DB
	liveFileMetadata := db.GetLiveFilesMetaData()
	for _, file := range liveFileMetadata {
		fmt.Printf("file.Name=[%s], file.Level=[%d], file.Size=[%d], file.SmallestKey=[%s], file.LargestKey=[%s]\n",
			file.Name, file.Level, file.Size, file.SmallestKey, file.LargestKey)
	}
}

func printProperties(openchainDB *OpenchainDB) {
	fmt.Println("------ Details of Properties ---")
	db := openchainDB.DB
	fmt.Printf("rocksdb.estimate-live-data-size:- BlockchainCF:%s, StateCF:%s, StateDeltaCF:%s, IndexesCF:%s, PersistCF:%s\n\n",
		db.GetPropertyCF("rocksdb.estimate-live-data-size", openchainDB.BlockchainCF),
		db.GetPropertyCF("rocksdb.estimate-live-data-size", openchainDB.StateCF),
		db.GetPropertyCF("rocksdb.estimate-live-data-size", openchainDB.StateDeltaCF),
		db.GetPropertyCF("rocksdb.estimate-live-data-size", openchainDB.IndexesCF),
		db.GetPropertyCF("rocksdb.estimate-live-data-size", openchainDB.PersistCF))
	fmt.Printf("Default:%s\n", db.GetProperty("rocksdb.estimate-live-data-size"))

	fmt.Printf("rocksdb.num-live-versions:- BlockchainCF:%s, StateCF:%s, StateDeltaCF:%s, IndexesCF:%s, PersistCF:%s\n\n",
		db.GetPropertyCF("rocksdb.num-live-versions", openchainDB.BlockchainCF),
		db.GetPropertyCF("rocksdb.num-live-versions", openchainDB.StateCF),
		db.GetPropertyCF("rocksdb.num-live-versions", openchainDB.StateDeltaCF),
		db.GetPropertyCF("rocksdb.num-live-versions", openchainDB.IndexesCF),
		db.GetPropertyCF("rocksdb.num-live-versions", openchainDB.PersistCF))

	fmt.Printf("rocksdb.cfstats:\n %s %s %s %s %s\n\n",
		db.GetPropertyCF("rocksdb.cfstats", openchainDB.BlockchainCF),
		db.GetPropertyCF("rocksdb.cfstats", openchainDB.StateCF),
		db.GetPropertyCF("rocksdb.cfstats", openchainDB.StateDeltaCF),
		db.GetPropertyCF("rocksdb.cfstats", openchainDB.IndexesCF),
		db.GetPropertyCF("rocksdb.cfstats", openchainDB.PersistCF))
}

func scan(openchainDB *OpenchainDB, cfName string, cf *gorocksdb.ColumnFamilyHandle, printer detailPrinter) {
	fmt.Printf("------- Details of column family = [%s]--------\n", cfName)
	itr := openchainDB.GetIterator(cf)
	totalKVs := 0
	overSizeKVs := 0
	itr.SeekToFirst()
	for ; itr.Valid(); itr.Next() {
		k := itr.Key()
		v := itr.Value()
		keyBytes := k.Data()
		valueSize := v.Size()
		totalKVs++
		if valueSize >= MaxValueSize {
			overSizeKVs++
			fmt.Printf("key=[%x], valueSize=[%d]\n", keyBytes, valueSize)
			if printer != nil {
				fmt.Println("=== KV Details === ")
				printer(v.Data())
				fmt.Println("")
			}
		}
		k.Free()
		v.Free()
	}
	itr.Close()
	fmt.Printf("totalKVs=[%d], overSizeKVs=[%d]\n", totalKVs, overSizeKVs)
}

func blockDetailPrinter(blockBytes []byte) {
	block, _ := protos.UnmarshallBlock(blockBytes)
	txs := block.GetTransactions()
	fmt.Printf("Number of transactions = [%d]\n", len(txs))
	for _, tx := range txs {
		if len(tx.Payload) >= MaxValueSize {
			cIDBytes := tx.ChaincodeID
			cID := &protos.ChaincodeID{}
			proto.Unmarshal(cIDBytes, cID)
			fmt.Printf("TxDetails: payloadSize=[%d], tx.Type=[%s], cID.Name=[%s], cID.Path=[%s]\n", len(tx.Payload), tx.Type, cID.Name, cID.Path)
		}
	}
}
