/*
Licensed to the Apache Software Foundation (ASF) under one
or more contributor license agreements.  See the NOTICE file
distributed with this work for additional information
regarding copyright ownership.  The ASF licenses this file
to you under the Apache License, Version 2.0 (the
"License"); you may not use this file except in compliance
with the License.  You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing,
software distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
KIND, either express or implied.  See the License for the
specific language governing permissions and limitations
under the License.
*/

package obcpbft

import (
	"bytes"
	"fmt"
	"strconv"
	"testing"

	"github.com/openblockchain/obc-peer/protos"
)

type mockRequest int

const (
	SyncDeltas mockRequest = iota
	SyncBlocks
	SyncSnapshot
)

type mockResponse int

const (
	Normal  mockResponse = iota
	Corrupt              // TODO implement
	Timeout
)

type MockLedger struct {
	cleanML             *MockLedger
	blocks              map[uint64]*protos.Block
	state               [][]byte
	remoteStateBlock    uint64
	stateDeltasPerBlock uint64
	filter              func(request mockRequest, replicaId uint64) mockResponse
}

func NewMockLedger(filter func(request mockRequest, replicaId uint64) mockResponse) *MockLedger {
	mock := &MockLedger{}
	mock.blocks = make(map[uint64]*protos.Block)
	mock.state = make([][]byte, 0)
	mock.remoteStateBlock = uint64(0)
	mock.stateDeltasPerBlock = uint64(3)
	if nil == filter {
		mock.filter = func(request mockRequest, replicaId uint64) mockResponse {
			return Normal
		}
	} else {
		mock.filter = filter
	}
	return mock
}

func (mock *MockLedger) GetBlockchainSize() (uint64, error) {
	max := ^uint64(0) // Count on the overflow
	for blockNumber := range mock.blocks {
		if max+1 <= blockNumber {
			max = blockNumber
		}
	}
	return max + 1, nil
}

func (mock *MockLedger) GetBlock(id uint64) (*protos.Block, error) {
	block, ok := mock.blocks[id]
	if !ok {
		return nil, fmt.Errorf("Block not found")
	}
	return block, nil
}

func (mock *MockLedger) HashBlock(block *protos.Block) ([]byte, error) {
	previousBlockNumberAsString := string(block.PreviousBlockHash)
	previousBlockAsUint64, err := strconv.ParseUint(previousBlockNumberAsString, 10, 64)
	if nil != err {
		return nil, fmt.Errorf("Could not convert previous blockhash to a uint64: %s", err)
	}

	return []byte(strconv.FormatUint(previousBlockAsUint64+uint64(1), 10)), nil
}

func (mock *MockLedger) forceRemoteStateBlock(blockNumber uint64) {
	mock.remoteStateBlock = blockNumber
}

func SimpleGetStateHash(blockNumber uint64) []byte {
	ml := NewMockLedger(nil)
	ml.forceRemoteStateBlock(blockNumber)
	syncStateMessages, _ := ml.GetRemoteStateSnapshot(uint64(0)) // Note in this implementation of the interface, this call never returns err
	for syncStateMessage := range syncStateMessages {
		ml.ApplyStateDelta(syncStateMessage.Delta, false)
	}
	stateHash, _ := ml.GetCurrentStateHash()
	return stateHash
}

func SimpleGetBlockHash(blockNumber uint64) []byte {
	block := &protos.Block{
		PreviousBlockHash: []byte(strconv.FormatUint(blockNumber-uint64(1), 10)),
	}
	res, _ := NewMockLedger(nil).HashBlock(block) // In this implementation, this call will never return err
	return res
}

func SimpleGetBlock(blockNumber uint64) *protos.Block {
	blockMessages, _ := NewMockLedger(nil).GetRemoteBlocks(0, blockNumber, blockNumber) // In this implementation, this call will never return err

	for blockMessage := range blockMessages {
		return blockMessage.Blocks[0]
	}
	return nil // unreachable
}

func (mock *MockLedger) GetRemoteBlocks(replicaId uint64, start, finish uint64) (<-chan *protos.SyncBlocks, error) {
	res := make(chan *protos.SyncBlocks)
	ft := mock.filter(SyncBlocks, replicaId)
	switch ft {
	case Normal:
		go func() {
			current := start
			for {
				res <- &protos.SyncBlocks{
					Range: &protos.SyncBlockRange{
						Start: current,
						End:   current,
					},
					Blocks: []*protos.Block{
						&protos.Block{
							PreviousBlockHash: []byte(strconv.FormatUint(current-uint64(1), 10)),
							StateHash:         SimpleGetStateHash(current),
						},
					},
				}

				if current == finish {
					break
				}

				if start < finish {
					current++
				} else {
					current--
				}
			}
			close(res)
		}()
	case Timeout:
	default:
		return nil, fmt.Errorf("Unsupported filter result %d", ft)
	}

	return res, nil
}

func (mock *MockLedger) GetRemoteStateSnapshot(replicaId uint64) (<-chan *protos.SyncStateSnapshot, error) {
	res := make(chan *protos.SyncStateSnapshot)
	ft := mock.filter(SyncSnapshot, replicaId)
	switch ft {
	case Normal:
		rds, err := (NewMockLedger(nil)).GetRemoteStateDeltas(uint64(0), 0, mock.remoteStateBlock)
		if nil != err {
			return nil, err
		}
		go func() {
			i := uint64(0)
			for deltas := range rds {
				for _, delta := range deltas.Deltas {
					res <- &protos.SyncStateSnapshot{
						Delta:       delta,
						Sequence:    i,
						BlockNumber: mock.remoteStateBlock,
						Request:     nil,
					}
					i++
				}
			}
			close(res)
		}()
	case Timeout:
	default:
		return nil, fmt.Errorf("Unsupported filter result %d", ft)
	}
	return res, nil
}

func (mock *MockLedger) GetRemoteStateDeltas(replicaId uint64, start, finish uint64) (<-chan *protos.SyncStateDeltas, error) {
	res := make(chan *protos.SyncStateDeltas)
	ft := mock.filter(SyncDeltas, replicaId)
	switch ft {
	case Normal:
		go func() {
			current := start
			for {
				ires := make([][]byte, mock.stateDeltasPerBlock)
				for i := uint64(0); i < mock.stateDeltasPerBlock; i++ {
					ires = append(ires, []byte(string(current)))
				}
				res <- &protos.SyncStateDeltas{
					Range: &protos.SyncBlockRange{
						Start: current,
						End:   current,
					},
					Deltas: ires,
				}
				if current == finish {
					break
				}

				if start < finish {
					current++
				} else {
					current--
				}
			}
			close(res)
		}()
	case Timeout:
	default:
		return nil, fmt.Errorf("Unsupported filter result %d", ft)
	}
	return res, nil
}

func (mock *MockLedger) PutBlock(blockNumber uint64, block *protos.Block) error {
	mock.blocks[blockNumber] = block
	return nil
}

func (mock *MockLedger) ApplyStateDelta(delta []byte, unapply bool) {
	if !unapply {
		mock.state = append(mock.state, delta)
	} else {
		mock.state = mock.state[:len(mock.state)-1]
	}
}

func (mock *MockLedger) EmptyState() error {
	mock.state = make([][]byte, 0)
	return nil
}

func (mock *MockLedger) GetCurrentStateHash() ([]byte, error) {
	res := make([]byte, mock.stateDeltasPerBlock)

	i := uint64(0)
	for _, states := range mock.state {
		for _, state := range states {
			res[i] += state
		}
		i = (i + 1) % mock.stateDeltasPerBlock
	}
	return res, nil
}

func (mock *MockLedger) VerifyBlockchain(start, finish uint64) (uint64, error) {
	current := start
	for {
		if current == finish {
			return 0, nil
		}

		cb, err := mock.GetBlock(current)

		if nil != err {
			return current, err
		}

		next := current

		if start < finish {
			next++
		} else {
			next--
		}

		nb, err := mock.GetBlock(next)

		if nil != err {
			return current, err
		}

		nbh, err := mock.HashBlock(nb)

		if nil != err {
			return current, err
		}

		if !bytes.Equal(nbh, cb.PreviousBlockHash) {
			return current, nil
		}

		current = next
	}
}

func TestMockLedger(t *testing.T) {
	ml := NewMockLedger(nil)
	ml.GetCurrentStateHash()

	blockMessages, err := ml.GetRemoteBlocks(uint64(0), uint64(10), uint64(0))

	for blockMessage := range blockMessages {
		current := blockMessage.Range.Start
		i := 0
		for {
			_ = ml.PutBlock(current, blockMessage.Blocks[i]) // Never fails
			i++

			if current == blockMessage.Range.End {
				break
			}

			if blockMessage.Range.Start < blockMessage.Range.End {
				current++
			} else {
				current--
			}
		}
	}

	blockNumber, err := ml.VerifyBlockchain(uint64(10), uint64(0))

	if nil != err {
		t.Fatalf("Retrieved blockchain did not validate at block %d with error '%s', error in mock ledger implementation.", blockNumber, err)
	}

	if blockNumber != 0 {
		t.Fatalf("Retrieved blockchain did not validate at block %d, error in mock ledger implementation.", blockNumber)
	}

	_ = ml.PutBlock(uint64(3), &protos.Block{ // Never fails
		PreviousBlockHash: []byte("WRONG"),
		StateHash:         []byte("WRONG"),
	})

	blockNumber, err = ml.VerifyBlockchain(uint64(10), uint64(0))

	if blockNumber != 4 {
		t.Fatalf("Mangled blockchain did not detect the correct block with the wrong hash, error in mock ledger implementation.")
	}

	ml.forceRemoteStateBlock(7)

	syncStateMessages, err := ml.GetRemoteStateSnapshot(uint64(0))

	if nil != err {
		t.Fatalf("Remote state snapshot call failed, error in mock ledger implementation: %s", err)
	}

	_ = ml.EmptyState() // Never fails
	for syncStateMessage := range syncStateMessages {
		ml.ApplyStateDelta(syncStateMessage.Delta, false)
	}

	block7, err := ml.GetBlock(7)

	if nil != err {
		t.Fatalf("Error retrieving block 7, which we should have, error in mock ledger implementation")
	}
	stateHash, _ := ml.GetCurrentStateHash()
	if !bytes.Equal(block7.StateHash, stateHash) {
		t.Fatalf("Computed state hash (%x) and block state hash (%x) do not match, error in mock ledger implementation", stateHash, block7.StateHash)
	}
}
