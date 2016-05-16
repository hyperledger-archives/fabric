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

package obcpbft

import (
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"

	"github.com/hyperledger/fabric/consensus"
	"github.com/hyperledger/fabric/protos"
)

type LedgerDirectory interface {
	GetLedgerByPeerID(peerID *protos.PeerID) (consensus.ReadOnlyLedger, bool)
}

type HashLedgerDirectory struct {
	remoteLedgers map[protos.PeerID]consensus.ReadOnlyLedger
}

func (hd *HashLedgerDirectory) GetLedgerByPeerID(peerID *protos.PeerID) (consensus.ReadOnlyLedger, bool) {
	ledger, ok := hd.remoteLedgers[*peerID]
	return ledger, ok
}

func (hd *HashLedgerDirectory) GetPeers() (*protos.PeersMessage, error) {
	_, network, err := hd.GetNetworkInfo()
	return &protos.PeersMessage{network}, err
}

func (hd *HashLedgerDirectory) GetPeerEndpoint() (*protos.PeerEndpoint, error) {
	self, _, err := hd.GetNetworkInfo()
	return self, err
}

func (hd *HashLedgerDirectory) GetNetworkInfo() (self *protos.PeerEndpoint, network []*protos.PeerEndpoint, err error) {
	network = make([]*protos.PeerEndpoint, len(hd.remoteLedgers)+1)
	i := 0
	for peerID := range hd.remoteLedgers {
		peerID := peerID // Get a memory address which will not be overwritten
		network[i] = &protos.PeerEndpoint{
			ID:   &peerID,
			Type: protos.PeerEndpoint_VALIDATOR,
		}
		i++
	}
	network[i] = &protos.PeerEndpoint{
		ID: &protos.PeerID{
			Name: "SelfID",
		},
		Type: protos.PeerEndpoint_VALIDATOR,
	}

	self = network[i]
	return
}

func (hd *HashLedgerDirectory) GetNetworkHandles() (self *protos.PeerID, network []*protos.PeerID, err error) {
	oSelf, oNetwork, err := hd.GetNetworkInfo()
	if nil != err {
		return
	}

	self = oSelf.ID
	network = make([]*protos.PeerID, len(oNetwork))
	for i, endpoint := range oNetwork {
		network[i] = endpoint.ID
	}
	return
}

type MockLedger struct {
	cleanML       *MockLedger
	blocks        map[uint64]*protos.Block
	blockHeight   uint64
	remoteLedgers LedgerDirectory

	mutex *sync.Mutex

	txID          interface{}
	curBatch      []*protos.Transaction
	curResults    []byte
	preBatchState uint64

	ce *consumerEndpoint // To support the ExecTx stuff
}

func NewMockLedger(remoteLedgers LedgerDirectory) *MockLedger {
	mock := &MockLedger{}
	mock.mutex = &sync.Mutex{}
	mock.blocks = make(map[uint64]*protos.Block)
	mock.blockHeight = 1
	mock.blocks[0] = &protos.Block{}
	mock.remoteLedgers = remoteLedgers

	return mock
}

func (mock *MockLedger) BeginTxBatch(id interface{}) error {
	if mock.txID != nil {
		return fmt.Errorf("Tx batch is already active")
	}
	mock.txID = id
	mock.curBatch = nil
	mock.curResults = nil
	return nil
}

func (mock *MockLedger) ExecTxs(id interface{}, txs []*protos.Transaction) ([]byte, error) {
	if !reflect.DeepEqual(mock.txID, id) {
		return nil, fmt.Errorf("Invalid batch ID")
	}

	mock.curBatch = append(mock.curBatch, txs...)
	var err error
	var txResult []byte
	if nil != mock.ce && nil != mock.ce.execTxResult {
		txResult, err = mock.ce.execTxResult(txs)
	} else {
		// This is basically a default fake default transaction execution
		if nil == txs {
			txs = []*protos.Transaction{&protos.Transaction{Payload: []byte("DUMMY")}}
		}

		for _, transaction := range txs {
			if transaction.Payload == nil {
				transaction.Payload = []byte("DUMMY")
			}

			txResult = append(txResult, transaction.Payload...)
		}

	}

	mock.curResults = append(mock.curResults, txResult...)

	return txResult, err
}

func (mock *MockLedger) CommitTxBatch(id interface{}, metadata []byte) (*protos.Block, error) {
	block, err := mock.commonCommitTx(id, metadata, false)
	if nil == err {
		mock.txID = nil
		mock.curBatch = nil
		mock.curResults = nil
	}
	return block, err
}

func (mock *MockLedger) commonCommitTx(id interface{}, metadata []byte, preview bool) (*protos.Block, error) {
	if !reflect.DeepEqual(mock.txID, id) {
		return nil, fmt.Errorf("Invalid batch ID")
	}

	previousBlockHash := []byte("Genesis")
	if 0 < mock.blockHeight {
		previousBlock, _ := mock.GetBlock(mock.blockHeight - 1)
		previousBlockHash, _ = mock.HashBlock(previousBlock)
	}

	block := &protos.Block{
		ConsensusMetadata: metadata,
		PreviousBlockHash: previousBlockHash,
		StateHash:         mock.curResults, // Use the current result output in the hash
		Transactions:      mock.curBatch,
		NonHashData: &protos.NonHashData{
			TransactionResults: []*protos.TransactionResult{
				&protos.TransactionResult{
					Result: mock.curResults,
				},
			},
		},
	}

	if !preview {
		hash, _ := mock.HashBlock(block)
		fmt.Printf("TEST LEDGER: Mock ledger is inserting block %d with hash %x\n", mock.blockHeight, hash)
		mock.blocks[mock.blockHeight] = block
		mock.blockHeight++
	}

	return block, nil
}

func (mock *MockLedger) PreviewCommitTxBatch(id interface{}, metadata []byte) ([]byte, error) {
	b, err := mock.commonCommitTx(id, metadata, true)
	if err != nil {
		return nil, err
	}
	return mock.getBlockInfoBlob(mock.blockHeight+1, b), nil
}

func (mock *MockLedger) RollbackTxBatch(id interface{}) error {
	if !reflect.DeepEqual(mock.txID, id) {
		return fmt.Errorf("Invalid batch ID")
	}
	mock.curBatch = nil
	mock.curResults = nil
	mock.txID = nil
	return nil
}

func (mock *MockLedger) GetBlockchainSize() uint64 {
	mock.mutex.Lock()
	defer func() {
		mock.mutex.Unlock()
	}()
	return mock.blockHeight
}

func (mock *MockLedger) GetBlock(id uint64) (*protos.Block, error) {
	mock.mutex.Lock()
	defer func() {
		mock.mutex.Unlock()
	}()
	block, ok := mock.blocks[id]
	if !ok {
		return nil, fmt.Errorf("Block not found")
	}
	return block, nil
}

func (mock *MockLedger) HashBlock(block *protos.Block) ([]byte, error) {
	return block.GetHash()
}

func (mock *MockLedger) GetBlockchainInfoBlob() []byte {
	b, _ := mock.GetBlock(mock.blockHeight - 1)
	return mock.getBlockInfoBlob(mock.blockHeight, b)
}

func (mock *MockLedger) getBlockInfoBlob(height uint64, block *protos.Block) []byte {
	info := &protos.BlockchainInfo{Height: height}
	info.CurrentBlockHash, _ = mock.HashBlock(block)
	h, _ := proto.Marshal(info)
	return h
}

func (mock *MockLedger) GetBlockHeadMetadata() ([]byte, error) {
	b, ok := mock.blocks[mock.blockHeight-1]
	if !ok {
		return nil, fmt.Errorf("could not retrieve block from mock ledger")
	}
	return b.ConsensusMetadata, nil
}

func (mock *MockLedger) simulateStateTransfer(meta []byte, id []byte, peers []*protos.PeerID) {
	var remoteLedger consensus.ReadOnlyLedger
	if len(peers) > 0 {
		var ok bool
		remoteLedger, ok = mock.remoteLedgers.GetLedgerByPeerID(peers[0])
		if !ok {
			panic("Asked for results from a peer which does not exist")
		}
	} else {
		panic("TODO, support state transfer from nil peers")
	}
	info := &protos.BlockchainInfo{}
	proto.Unmarshal(id, info)
	fmt.Printf("TEST LEDGER skipping to %+v, %+v", meta, info)
	p := 0
	if mock.blockHeight >= info.Height {
		panic(fmt.Sprintf("Asked to skip to a block (%d) which is lower than our current height of %d", info.Height, mock.blockHeight))
	}
	for n := mock.blockHeight; n < info.Height; n++ {
		block, err := remoteLedger.GetBlock(n)

		if nil != err {
			n--
			// To accomodate Sieve, it's possible to ask for a block that hasn't been committed yet
			// because Sieve must broadcast the verify set before it is committed
			fmt.Printf("TEST LEDGER: Block not ready yet")
			time.Sleep(100 * time.Millisecond)
			p++
			if p > 10 {
				panic("Tried to get a block 10 times, no luck")
			}
			continue
		}

		block.ConsensusMetadata = meta
		mock.blocks[n] = block
	}
	mock.blockHeight = info.Height
}
