package state

import (
	"bytes"
	"encoding/binary"
	"github.com/openblockchain/obc-peer/protos"
	"github.com/openblockchain/obc-peer/openchain/db"
	"github.com/tecbot/gorocksdb"
)

type Blockchain struct {
	size        uint64
}

var blockchainInstance *Blockchain

func GetBlockChain() (*Blockchain, error) {
	if blockchainInstance == nil {
		blockchainInstance = new(Blockchain)
		var err error
		blockchainInstance.size, err = fetchBlockChainSizeFromDB()
		if err != nil {
			return nil, err
		}
	}
	return blockchainInstance, nil
}

func (blockchain *Blockchain) GetLastBlock() (*protos.Block, error) {
	return blockchain.GetBlock(blockchain.size - 1)
}

func (blockchain *Blockchain) GetBlock(blockNumber uint64) (*protos.Block, error) {
	return blockchain.fetchBlockFromDB(blockNumber)
}

func (blockchain *Blockchain) GetSize() uint64 {
	return blockchainInstance.size
}

func (blockchain *Blockchain) fetchBlockFromDB(blockNumber uint64) (*protos.Block, error) {
	blockBytes, err := db.GetDBHandle().GetFromBlockChainCF(EncodeBlockNumberDBKey(blockNumber))
	if err != nil {
		return nil, err
	}
	return protos.UnmarshallBlock(blockBytes)
}

func fetchBlockChainSizeFromDB() (uint64, error) {
	bytes, err := db.GetDBHandle().GetFromBlockChainCF(blockCountKey)
	if err != nil {
		return 0, err
	}
	if bytes == nil {
		return 0, nil
	}
	return decodeToUint64(bytes), nil
}

func (blockchain *Blockchain) AddBlock(block *protos.Block) error {
	err := blockchain.persistBlock(block, blockchain.size)
	if err != nil {
		return err
	}
	blockchain.size++
	return nil
}

func (blockchain *Blockchain) persistBlock(block *protos.Block, blockNumber uint64) error {
	state, err := GetState()
	if err != nil {
		return err
	}
	blockBytes, blockBytesErr := block.Bytes()
	if blockBytesErr != nil {
		return blockBytesErr
	}
	writeBatch := gorocksdb.NewWriteBatch()
	writeBatch.PutCF(db.GetDBHandle().BlockchainCF, EncodeBlockNumberDBKey(blockNumber), blockBytes)

	sizeBytes := encodeUint64(blockNumber + 1)
	writeBatch.PutCF(db.GetDBHandle().BlockchainCF, blockCountKey, sizeBytes)

	state.addChangesForPersistence(writeBatch)

	opt := gorocksdb.NewDefaultWriteOptions()
	err = db.GetDBHandle().DB.Write(opt, writeBatch)
	if err != nil {
		return err
	}
	return nil
}

var blockCountKey = []byte("blockCount")

func EncodeBlockNumberDBKey(blockNumber uint64) []byte {
	return encodeUint64(blockNumber)
}

func DecodeBlockNumberDBKey(dbKey []byte) uint64 {
	return decodeToUint64(dbKey)
}

func encodeUint64(number uint64) []byte {
	bytes := make([]byte, 8)
	binary.BigEndian.PutUint64(bytes, number)
	return bytes
}

func decodeToUint64(bytes []byte) uint64 {
	return binary.BigEndian.Uint64(bytes)
}

func (blockchain *Blockchain) String() string {
	var buffer bytes.Buffer
	size := blockchain.GetSize()
	for i := uint64(0); i < size; i++ {
		block, blockErr := blockchain.GetBlock(i)
		if blockErr != nil {
			return ""
		}
		buffer.WriteString("\n----------<block>----------\n")
		buffer.WriteString(block.String())
		buffer.WriteString("\n----------<\\block>----------\n")
	}
	return buffer.String()
}
