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
	"bytes"
	"io/ioutil"
	"os"
	"testing"

	"github.com/spf13/viper"
	"github.com/tecbot/gorocksdb"
)

func TestMain(m *testing.M) {
	setupTestConfig()
	os.Exit(m.Run())
}

func TestCreateDB_DirDoesNotExist(t *testing.T) {
	defer deleteTestDBPath()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Failed to create DB: %s", r)
		}
	}()
	openchainDB := Create()
	defer openchainDB.Close()
}

func TestCreateDB_NonEmptyDirExists(t *testing.T) {
	createNonEmptyTestDBPath()
	defer deleteTestDBPath()
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("Dir alrady exists. DB creation should throw error")
		}
	}()
	openchainDB := Create()
	defer openchainDB.Close()

}

func TestOpenDB_DirDoesNotExist(t *testing.T) {
	openchainDB := Create()
	defer deleteTestDBPath()
	defer openchainDB.Close()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Failed to open DB: %s", r)
		}
	}()
	openchainDB.Open()
}

func TestOpenDB_NonEmptyDirExists(t *testing.T) {
	openchainDB := Create()
	deleteTestDBPath()
	createNonEmptyTestDBPath()

	defer deleteTestDBPath()
	defer openchainDB.Close()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Failed to open DB: %s", r)
		}
	}()
	openchainDB.Open()
}

func TestGetDB_DirDoesNotExist(t *testing.T) {
	defer deleteTestDBPath()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Failed to open DB: %s", r)
		}
	}()
	openchainDB := GetDBHandle()

	defer openchainDB.Close()
}

func TestGetDB_NonEmptyDirExists(t *testing.T) {
	createNonEmptyTestDBPath()
	defer deleteTestDBPath()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Failed to open DB: %s", r)
		}
	}()
	openchainDB := GetDBHandle()
	defer openchainDB.Close()
}

func TestWriteAndRead(t *testing.T) {
	openchainDB := GetDBHandle()
	defer deleteTestDBPath()
	defer openchainDB.Close()
	performBasicReadWrite(openchainDB, t)
}

// This test verifies that when a new column family is added to the DB
// users at an older level of the DB will still be able to open it with new code
func TestDBColumnUpgrade(t *testing.T) {
	openchainDB := GetDBHandle()
	openchainDB.Close()

	oldcfs := columnfamilies
	columnfamilies = append([]string{"Testing"}, columnfamilies...)
	defer func() {
		columnfamilies = oldcfs
	}()
	openchainDB = GetDBHandle()

	defer deleteTestDBPath()
	defer openchainDB.Close()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Error re-opening DB with upgraded columnFamilies")
		}
	}()
}

// db helper functions
func createNonEmptyTestDBPath() {
	dbPath := viper.GetString("peer.fileSystemPath")
	os.MkdirAll(dbPath+"/db/tmpFile", 0775)
}

func deleteTestDBPath() {
	dbPath := viper.GetString("peer.fileSystemPath")
	os.RemoveAll(dbPath)
}

func setupTestConfig() {
	tempDir, err := ioutil.TempDir("", "fabric-db-test")
	if err != nil {
		panic(err)
	}
	viper.Set("peer.fileSystemPath", tempDir)
	deleteTestDBPath()
}

func performBasicReadWrite(openchainDB *OpenchainDB, t *testing.T) {
	opt := gorocksdb.NewDefaultWriteOptions()
	defer opt.Destroy()
	writeBatch := gorocksdb.NewWriteBatch()
	defer writeBatch.Destroy()
	writeBatch.PutCF(openchainDB.BlockchainCF, []byte("dummyKey"), []byte("dummyValue"))
	err := openchainDB.DB.Write(opt, writeBatch)
	if err != nil {
		t.Fatal("Error while writing to db")
	}
	value, err := openchainDB.GetFromBlockchainCF([]byte("dummyKey"))

	if err != nil {
		t.Fatalf("read error = [%s]", err)
	}

	if !bytes.Equal(value, []byte("dummyValue")) {
		t.Fatal("read error. Bytes not equal")
	}
}
