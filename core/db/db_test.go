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
	err := CreateDB()
	if err != nil {
		t.Fatalf("Failed to create DB: %s", err)
	}
	deleteTestDB()
}

func TestCreateDB_NonEmptyDirExists(t *testing.T) {
	createNonEmptyTestDBPath()
	err := CreateDB()
	if err == nil {
		t.Fatal("Dir alrady exists. DB creation should throw error")
	}
	deleteTestDBPath()
}

func TestWriteAndRead(t *testing.T) {
	createTestDB()
	defer deleteTestDB()
	performBasicReadWrite(t)
}

func TestOpenDB_DirDoesNotExist(t *testing.T) {
	deleteTestDBPath()
	defer deleteTestDB()
	performBasicReadWrite(t)
}

func TestOpenDB_DirEmpty(t *testing.T) {
	deleteTestDBPath()
	createTestDBPath()
	defer deleteTestDB()
	performBasicReadWrite(t)
}

// This test verifies that when a new column family is added to the DB
// users at an older level of the DB will still be able to open it with new code
func TestDBColumnUpgrade(t *testing.T) {
	deleteTestDBPath()
	createTestDBPath()
	err := CreateDB()
	if nil != err {
		t.Fatalf("Error creating DB")
	}
	db, err := openDB()
	if nil != err {
		t.Fatalf("Error opening DB")
	}
	db.CloseDB()

	oldcfs := columnfamilies
	columnfamilies = append([]string{"Testing"}, columnfamilies...)
	defer func() {
		columnfamilies = oldcfs
	}()
	db, err = openDB()
	if nil != err {
		t.Fatalf("Error re-opening DB with upgraded columnFamilies")
	}
	db.CloseDB()
}

// db helper functions
func createTestDBPath() {
	dbPath := viper.GetString("peer.fileSystemPath")
	os.MkdirAll(dbPath, 0775)
}

func createNonEmptyTestDBPath() {
	dbPath := viper.GetString("peer.fileSystemPath")
	os.MkdirAll(dbPath+"/db/tmpFile", 0775)
}

func createTestDB() error {
	return CreateDB()
}

func deleteTestDBPath() {
	dbPath := viper.GetString("peer.fileSystemPath")
	os.RemoveAll(dbPath)
}

func deleteTestDB() {
	GetDBHandle().CloseDB()
	deleteTestDBPath()
}

func setupTestConfig() {
	tempDir, err := ioutil.TempDir("", "fabric-db-test")
	if err != nil {
		panic(err)
	}
	viper.Set("peer.fileSystemPath", tempDir)
	deleteTestDBPath()
}

func performBasicReadWrite(t *testing.T) {
	openchainDB := GetDBHandle()
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
