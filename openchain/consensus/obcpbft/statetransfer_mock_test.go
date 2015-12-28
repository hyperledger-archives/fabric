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

type mockLedger struct {
	blocks              map[uint64]*protos.Block
	state               [][]byte
	remoteStateBlock    uint64
	stateDeltasPerBlock uint64
	timeoutReplicas     map[uint64]bool
}

func newMockLedger(timeoutReplicas map[uint64]bool) *mockLedger {
	mock := &mockLedger{}
	mock.blocks = make(map[uint64]*protos.Block)
	mock.state = make([][]byte, 0)
	mock.remoteStateBlock = uint64(0)
	mock.stateDeltasPerBlock = uint64(3)
	if nil == timeoutReplicas {
		mock.timeoutReplicas = make(map[uint64]bool)
	} else {
		mock.timeoutReplicas = timeoutReplicas
	}
	return mock
}

func (mock *mockLedger) getBlockchainSize() uint64 {
	max := ^uint64(0) // Count on the overflow
	for blockNumber := range mock.blocks {
		if max+1 <= blockNumber {
			max = blockNumber
		}
	}
	return max + 1
}

func (mock *mockLedger) getBlock(id uint64) (*protos.Block, error) {
	block, ok := mock.blocks[id]
	if !ok {
		return nil, fmt.Errorf("Block not found")
	}
	return block, nil
}

func (mock *mockLedger) hashBlock(block *protos.Block) ([]byte, error) {
	previousBlockNumberAsString := string(block.PreviousBlockHash)
	previousBlockAsUint64, err := strconv.ParseUint(previousBlockNumberAsString, 10, 64)
	if nil != err {
		return nil, fmt.Errorf("Could not convert previous blockhash to a uint64: %s", err)
	}

	return []byte(strconv.FormatUint(previousBlockAsUint64+uint64(1), 10)), nil
}

func (mock *mockLedger) getRemoteBlocks(replicaId uint64, start, finish uint64) (<-chan *protos.SyncBlocks, error) {
	res := make(chan *protos.SyncBlocks)

	if _, ok := mock.timeoutReplicas[replicaId]; !ok {
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
							StateHash:         simpleGetStateHash(current),
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
	}
	return res, nil
}

func (mock *mockLedger) forceRemoteStateBlock(blockNumber uint64) {
	mock.remoteStateBlock = blockNumber
}

func simpleGetStateHash(blockNumber uint64) []byte {
	ml := newMockLedger(nil)
	ml.forceRemoteStateBlock(blockNumber)
	syncStateMessages, _ := ml.getRemoteStateSnapshot(uint64(0)) // Note in this implementation of the interface, this call never returns err
	for syncStateMessage := range syncStateMessages {
		ml.applyStateDelta(syncStateMessage.Delta, false)
	}
	return ml.getCurrentStateHash()
}

func simpleGetBlockHash(blockNumber uint64) []byte {
	ml := &mockLedger{}
	block := &protos.Block{
		PreviousBlockHash: []byte(strconv.FormatUint(blockNumber-uint64(1), 10)),
	}
	res, _ := ml.hashBlock(block) // In this implementation, this call will never return err
	return res
}

func simpleGetBlock(blockNumber uint64) *protos.Block {
	ml := &mockLedger{}
	blockMessages, _ := ml.getRemoteBlocks(0, blockNumber, blockNumber) // In this implementation, this call will never return err

	for blockMessage := range blockMessages {
		return blockMessage.Blocks[0]
	}
	return nil // unreachable
}

func (mock *mockLedger) getRemoteStateSnapshot(replicaId uint64) (<-chan *protos.SyncStateSnapshot, error) {
	res := make(chan *protos.SyncStateSnapshot)
	if _, ok := mock.timeoutReplicas[replicaId]; !ok {
		rds, err := mock.getRemoteStateDeltas(uint64(0), 0, mock.remoteStateBlock)
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
	}
	return res, nil
}

func (mock *mockLedger) getRemoteStateDeltas(replicaId uint64, start, finish uint64) (<-chan *protos.SyncStateDeltas, error) {
	res := make(chan *protos.SyncStateDeltas)
	if _, ok := mock.timeoutReplicas[replicaId]; !ok {
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
	}
	return res, nil
}

func (mock *mockLedger) putBlock(blockNumber uint64, block *protos.Block) {
	mock.blocks[blockNumber] = block
}

func (mock *mockLedger) applyStateDelta(delta []byte, unapply bool) {
	if !unapply {
		mock.state = append(mock.state, delta)
	} else {
		mock.state = mock.state[:len(mock.state)-1]
	}
}

func (mock *mockLedger) emptyState() {
	mock.state = make([][]byte, 0)
}

func (mock *mockLedger) getCurrentStateHash() []byte {
	res := make([]byte, mock.stateDeltasPerBlock)

	i := uint64(0)
	for _, states := range mock.state {
		for _, state := range states {
			res[i] += state
		}
		i = (i + 1) % mock.stateDeltasPerBlock
	}
	return res
}

func (mock *mockLedger) verifyBlockChain(start, finish uint64) (uint64, error) {
	current := start
	for {
		if current == finish {
			return 0, nil
		}

		cb, err := mock.getBlock(current)

		if nil != err {
			return current, err
		}

		next := current

		if start < finish {
			next++
		} else {
			next--
		}

		nb, err := mock.getBlock(next)

		if nil != err {
			return current, err
		}

		nbh, err := mock.hashBlock(nb)

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
	ml := newMockLedger(nil)
	ml.getCurrentStateHash()

	blockMessages, err := ml.getRemoteBlocks(uint64(0), uint64(10), uint64(0))

	for blockMessage := range blockMessages {
		current := blockMessage.Range.Start
		i := 0
		for {
			ml.putBlock(current, blockMessage.Blocks[i])
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

	blockNumber, err := ml.verifyBlockChain(uint64(10), uint64(0))

	if nil != err {
		t.Fatalf("Retrieved blockchain did not validate at block %d with error '%s', error in mock ledger implementation.", blockNumber, err)
	}

	if blockNumber != 0 {
		t.Fatalf("Retrieved blockchain did not validate at block %d, error in mock ledger implementation.", blockNumber)
	}

	ml.putBlock(uint64(3), &protos.Block{
		PreviousBlockHash: []byte("WRONG"),
		StateHash:         []byte("WRONG"),
	})

	blockNumber, err = ml.verifyBlockChain(uint64(10), uint64(0))

	if blockNumber != 4 {
		t.Fatalf("Mangled blockchain did not detect the correct block with the wrong hash, error in mock ledger implementation.")
	}

	ml.forceRemoteStateBlock(7)

	syncStateMessages, err := ml.getRemoteStateSnapshot(uint64(0))

	if nil != err {
		t.Fatalf("Remote state snapshot call failed, error in mock ledger implementation: %s", err)
	}

	ml.emptyState()
	for syncStateMessage := range syncStateMessages {
		ml.applyStateDelta(syncStateMessage.Delta, false)
	}

	block7, err := ml.getBlock(7)

	if nil != err {
		t.Fatalf("Error retrieving block 7, which we should have, error in mock ledger implementation")
	}

	if !bytes.Equal(block7.StateHash, ml.getCurrentStateHash()) {
		t.Fatalf("Computed state hash (%x) and block state hash (%x) do not match, error in mock ledger implementation", ml.getCurrentStateHash(), block7.StateHash)
	}
}
