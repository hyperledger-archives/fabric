package db

import (
	"bytes"
	"fmt"
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
	viper.SetConfigName("db_test") // name of config file (without extension)
	viper.AddConfigPath(".")       // path to look for the config file in
	err := viper.ReadInConfig()    // Find and read the config file
	if err != nil {                // Handle errors reading the config file
		panic(fmt.Errorf("Fatal error config file: %s \n", err))
	}
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
