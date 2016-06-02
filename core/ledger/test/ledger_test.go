package ledger_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"bytes"
	"strconv"

	"github.com/hyperledger/fabric/core/db"
	"github.com/hyperledger/fabric/core/ledger"
	"github.com/hyperledger/fabric/core/ledger/statemgmt"
	//"github.com/hyperledger/fabric/core/ledger/statemgmt/state"
	"github.com/hyperledger/fabric/protos"
	"github.com/hyperledger/fabric/core/util"
)

func appendAll(content ...[]byte) []byte {
	combinedContent := []byte{}
	for _, b := range content {
		combinedContent = append(combinedContent, b...)
	}
	return combinedContent
}

var _ = Describe("Ledger", func() {
	setupTestConfig()

	Describe("Ledger Commit", func() {
		testDBWrapper := db.NewTestDBWrapper()
		testDBWrapper.CreateFreshDBGinkgo()
		ledgerPtr, err := ledger.GetNewLedger()
		if err != nil {
			Fail("failed to get a fresh ledger")
		}
		It("creates, populates and finishes a batch", func() {
			Expect(ledgerPtr.BeginTxBatch(1)).To(BeNil())
			ledgerPtr.TxBegin("txUuid")
			Expect(ledgerPtr.SetState("chaincode1", "key1", []byte("value1"))).To(BeNil())
			Expect(ledgerPtr.SetState("chaincode2", "key2", []byte("value2"))).To(BeNil())
			Expect(ledgerPtr.SetState("chaincode3", "key3", []byte("value3"))).To(BeNil())
			ledgerPtr.TxFinished("txUuid", true)
		})
		It("should return uncommitted state from memory", func() {
			state, _ := ledgerPtr.GetState("chaincode1", "key1", false)
			Expect(state).To(Equal([]byte("value1")))
			state, _ = ledgerPtr.GetState("chaincode2", "key2", false)
			Expect(state).To(Equal([]byte("value2")))
			state, _ = ledgerPtr.GetState("chaincode3", "key3", false)
			Expect(state).To(Equal([]byte("value3")))
		})
		It("should commit the batch", func() {
			uuid := util.GenerateUUID()
			tx, err := protos.NewTransaction(protos.ChaincodeID{Path: "testUrl"}, uuid, "anyfunction", []string{"param1, param2"})
			Expect(err).To(BeNil())
			err = ledgerPtr.CommitTxBatch(1, []*protos.Transaction{tx}, nil, []byte("proof"))
			Expect(err).To(BeNil())
		})
		It("should still return state from memory", func() {
			state, _ := ledgerPtr.GetState("chaincode1", "key1", false)
			Expect(state).To(Equal([]byte("value1")))
			state, _ = ledgerPtr.GetState("chaincode2", "key2", false)
			Expect(state).To(Equal([]byte("value2")))
			state, _ = ledgerPtr.GetState("chaincode3", "key3", false)
			Expect(state).To(Equal([]byte("value3")))
		})
		It("should return committed state", func() {
			state, _ := ledgerPtr.GetState("chaincode1", "key1", true)
			Expect(state).To(Equal([]byte("value1")))
			state, _ = ledgerPtr.GetState("chaincode2", "key2", true)
			Expect(state).To(Equal([]byte("value2")))
			state, _ = ledgerPtr.GetState("chaincode3", "key3", true)
			Expect(state).To(Equal([]byte("value3")))
		})
	})

	Describe("Ledger Rollback", func() {
		testDBWrapper := db.NewTestDBWrapper()
		testDBWrapper.CreateFreshDBGinkgo()
		ledgerPtr, err := ledger.GetNewLedger()
		if err != nil {
			Fail("failed to get a fresh ledger")
		}
		It("creates, populates and finishes a batch", func() {
			Expect(ledgerPtr.BeginTxBatch(1)).To(BeNil())
			ledgerPtr.TxBegin("txUuid")
			Expect(ledgerPtr.SetState("chaincode1", "key1", []byte("value1"))).To(BeNil())
			Expect(ledgerPtr.SetState("chaincode2", "key2", []byte("value2"))).To(BeNil())
			Expect(ledgerPtr.SetState("chaincode3", "key3", []byte("value3"))).To(BeNil())
			ledgerPtr.TxFinished("txUuid", true)
		})
		It("should rollback the batch", func() {
			Expect(ledgerPtr.RollbackTxBatch(1)).To(BeNil())
		})
		/*
		It("should not return state from memory", func() {
			state, _ := ledgerPtr.GetState("chaincode1", "key1", false)
			Expect(state).To(BeNil())
			state, _ = ledgerPtr.GetState("chaincode2", "key2", false)
			Expect(state).To(BeNil())
			state, _ = ledgerPtr.GetState("chaincode3", "key3", false)
			Expect(state).To(BeNil())
		})
		*/
		It("should not return committed state", func() {
			state, _ := ledgerPtr.GetState("chaincode1", "key1", true)
			Expect(state).To(BeNil())
			state, _ = ledgerPtr.GetState("chaincode2", "key2", true)
			Expect(state).To(BeNil())
			state, _ = ledgerPtr.GetState("chaincode3", "key3", true)
			Expect(state).To(BeNil())
		})
	})

	Describe("Ledger Rollback with Hash", func() {
		var hash0, hash1 []byte
		testDBWrapper := db.NewTestDBWrapper()
		testDBWrapper.CreateFreshDBGinkgo()
		ledgerPtr, err := ledger.GetNewLedger()
		if err != nil {
			Fail("failed to get a fresh ledger")
		}
		It("creates, populates and finishes a batch", func() {
			Expect(ledgerPtr.BeginTxBatch(0)).To(BeNil())
			ledgerPtr.TxBegin("txUuid")
			Expect(ledgerPtr.SetState("chaincode1", "key1", []byte("value1"))).To(BeNil())
			Expect(ledgerPtr.SetState("chaincode2", "key2", []byte("value2"))).To(BeNil())
			Expect(ledgerPtr.SetState("chaincode3", "key3", []byte("value3"))).To(BeNil())
			ledgerPtr.TxFinished("txUuid", true)
		})
		It("rollsback the batch", func() {
			Expect(ledgerPtr.RollbackTxBatch(0)).To(BeNil())
		})
		It("should not return an error from GetTempStateHash", func() {
			hash0, err = ledgerPtr.GetTempStateHash()
			Expect(err).To(BeNil())
		})
		It("creates, populates and finishes a batch", func() {
			Expect(ledgerPtr.BeginTxBatch(1)).To(BeNil())
			ledgerPtr.TxBegin("txUuid")
			Expect(ledgerPtr.SetState("chaincode1", "key1", []byte("value1"))).To(BeNil())
			Expect(ledgerPtr.SetState("chaincode2", "key2", []byte("value2"))).To(BeNil())
			Expect(ledgerPtr.SetState("chaincode3", "key3", []byte("value3"))).To(BeNil())
			ledgerPtr.TxFinished("txUuid", true)
		})
		It("retrieves state hash from GetTempStateHash without error", func() {
			hash1, err = ledgerPtr.GetTempStateHash()
			Expect(err).To(BeNil())
		})
		It("should have different values for the hash", func() {
			Expect(hash0).ToNot(Equal(hash1))
		})
		It("rollsback the batch", func() {
			Expect(ledgerPtr.RollbackTxBatch(1)).To(BeNil())
		})
		It("retrieves state hash from GetTempStateHash without error", func() {
			hash1, err = ledgerPtr.GetTempStateHash()
			Expect(err).To(BeNil())
		})
		It("should have the same values for the hash", func() {
			Expect(hash0).To(Equal(hash1))
		})
		It("should not return state from memory", func() {
			state, _ := ledgerPtr.GetState("chaincode1", "key1", false)
			Expect(state).To(BeNil())
			state, _ = ledgerPtr.GetState("chaincode2", "key2", false)
			Expect(state).To(BeNil())
			state, _ = ledgerPtr.GetState("chaincode3", "key3", false)
			Expect(state).To(BeNil())
		})
		It("should not return committed state", func() {
			state, _ := ledgerPtr.GetState("chaincode1", "key1", true)
			Expect(state).To(BeNil())
			state, _ = ledgerPtr.GetState("chaincode2", "key2", true)
			Expect(state).To(BeNil())
			state, _ = ledgerPtr.GetState("chaincode3", "key3", true)
			Expect(state).To(BeNil())
		})
	})

	Describe("Ledger Commit with Incorrect ID", func() {
		testDBWrapper := db.NewTestDBWrapper()
		testDBWrapper.CreateFreshDBGinkgo()
		ledgerPtr, err := ledger.GetNewLedger()
		if err != nil {
			Fail("failed to get a fresh ledger")
		}
		It("creates, populates and finishes a batch", func() {
			Expect(ledgerPtr.BeginTxBatch(1)).To(BeNil())
			ledgerPtr.TxBegin("txUuid")
			Expect(ledgerPtr.SetState("chaincode1", "key1", []byte("value1"))).To(BeNil())
			Expect(ledgerPtr.SetState("chaincode2", "key2", []byte("value2"))).To(BeNil())
			Expect(ledgerPtr.SetState("chaincode3", "key3", []byte("value3"))).To(BeNil())
			ledgerPtr.TxFinished("txUuid", true)
		})
		It("should return uncommitted state from memory", func() {
			state, _ := ledgerPtr.GetState("chaincode1", "key1", false)
			Expect(state).To(Equal([]byte("value1")))
			state, _ = ledgerPtr.GetState("chaincode2", "key2", false)
			Expect(state).To(Equal([]byte("value2")))
			state, _ = ledgerPtr.GetState("chaincode3", "key3", false)
			Expect(state).To(Equal([]byte("value3")))
		})
		It("should not commit batch ith incorrect ID", func() {
			uuid := util.GenerateUUID()
			tx, err := protos.NewTransaction(protos.ChaincodeID{Path: "testUrl"}, uuid, "anyfunction", []string{"param1, param2"})
			Expect(err).To(BeNil())
			err = ledgerPtr.CommitTxBatch(2, []*protos.Transaction{tx}, nil, []byte("proof"))
			Expect(err).ToNot(BeNil())
		})
	})

	Describe("Ledger GetTempStateHashWithTxDeltaStateHashes", func() {
		testDBWrapper := db.NewTestDBWrapper()
		testDBWrapper.CreateFreshDBGinkgo()
		ledgerPtr, err := ledger.GetNewLedger()
		if err != nil {
			Fail("failed to get a fresh ledger")
		}
		It("creates, populates and finishes a transaction", func() {
			Expect(ledgerPtr.BeginTxBatch(1)).To(BeNil())
			ledgerPtr.TxBegin("txUuid1")
			Expect(ledgerPtr.SetState("chaincode1", "key1", []byte("value1"))).To(BeNil())
			ledgerPtr.TxFinished("txUuid1", true)
		})
		It("creates, populates and finishes a transaction", func() {
			ledgerPtr.TxBegin("txUuid2")
			Expect(ledgerPtr.SetState("chaincode2", "key2", []byte("value2"))).To(BeNil())
			ledgerPtr.TxFinished("txUuid2", true)
		})
		It("creates, but does not populate and finishes a transaction", func() {
			ledgerPtr.TxBegin("txUuid3")
			ledgerPtr.TxFinished("txUuid3", true)
		})
		It("creates, populates and finishes a transaction", func() {
			ledgerPtr.TxBegin("txUuid4")
			Expect(ledgerPtr.SetState("chaincode4", "key4", []byte("value4"))).To(BeNil())
			ledgerPtr.TxFinished("txUuid4", false)
		})
		It("should retrieve the delta state hash array containing expected values", func() {
			_, txDeltaHashes, err := ledgerPtr.GetTempStateHashWithTxDeltaStateHashes()
			Expect(err).To(BeNil())
			Expect(util.ComputeCryptoHash(appendAll([]byte("chaincode1key1value1")))).To(Equal(txDeltaHashes["txUuid1"]))
			Expect(util.ComputeCryptoHash(appendAll([]byte("chaincode2key2value2")))).To(Equal(txDeltaHashes["txUuid2"]))
			Expect(txDeltaHashes["txUuid3"]).To(BeNil())
			_, ok := txDeltaHashes["txUuid4"]
			Expect(ok).To(Equal(false))
		})
		It("should commit the batch", func() {
			Expect(ledgerPtr.CommitTxBatch(1, []*protos.Transaction{}, nil, []byte("proof"))).To(BeNil())
		})
		It("creates, populates and finishes a transaction", func() {
			Expect(ledgerPtr.BeginTxBatch(2)).To(BeNil())
			ledgerPtr.TxBegin("txUuid1")
			Expect(ledgerPtr.SetState("chaincode1", "key1", []byte("value1"))).To(BeNil())
			ledgerPtr.TxFinished("txUuid1", true)
		})
		It("should retrieve a delta state hash array of length 1", func () {
			_, txDeltaHashes, err := ledgerPtr.GetTempStateHashWithTxDeltaStateHashes()
			Expect(err).To(BeNil())
			Expect(len(txDeltaHashes)).To(Equal(1))
		})
	})

	Describe("Ledger StateSnapshot", func() {
		testDBWrapper := db.NewTestDBWrapper()
		testDBWrapper.CreateFreshDBGinkgo()
		ledgerPtr, err := ledger.GetNewLedger()
		if err != nil {
			Fail("failed to get a fresh ledger")
		}
		It("creates, populates and finishes a batch", func() {
			Expect(ledgerPtr.BeginTxBatch(1)).To(BeNil())
			ledgerPtr.TxBegin("txUuid")
			Expect(ledgerPtr.SetState("chaincode1", "key1", []byte("value1"))).To(BeNil())
			Expect(ledgerPtr.SetState("chaincode2", "key2", []byte("value2"))).To(BeNil())
			Expect(ledgerPtr.SetState("chaincode3", "key3", []byte("value3"))).To(BeNil())
			ledgerPtr.TxFinished("txUuid", true)
		})
		It("should commit the batch", func() {
			uuid := util.GenerateUUID()
			tx, err := protos.NewTransaction(protos.ChaincodeID{Path: "testUrl"}, uuid, "anyfunction", []string{"param1, param2"})
			Expect(err).To(BeNil())
			err = ledgerPtr.CommitTxBatch(1, []*protos.Transaction{tx}, nil, []byte("proof"))
			Expect(err).To(BeNil())
		})
		snapshot, err := ledgerPtr.GetStateSnapshot()
		It("creates a snapshot without error", func() {
			Expect(err).To(BeNil())
			defer snapshot.Release()
		})
		// Modify keys to ensure they do not impact the snapshot
		It("creates, populates and finishes another batch, deleting some state from prior batch", func() {
			Expect(ledgerPtr.BeginTxBatch(2)).To(BeNil())
			ledgerPtr.TxBegin("txUuid")
			Expect(ledgerPtr.DeleteState("chaincode1", "key1")).To(BeNil())
			Expect(ledgerPtr.SetState("chaincode4", "key4", []byte("value4"))).To(BeNil())
			Expect(ledgerPtr.SetState("chaincode5", "key5", []byte("value5"))).To(BeNil())
			Expect(ledgerPtr.SetState("chaincode6", "key6", []byte("value6"))).To(BeNil())
			ledgerPtr.TxFinished("txUuid", true)
		})
		It("should commit the batch", func() {
			uuid := util.GenerateUUID()
			tx, err := protos.NewTransaction(protos.ChaincodeID{Path: "testUrl"}, uuid, "anyfunction", []string{"param1, param2"})
			Expect(err).To(BeNil())
			err = ledgerPtr.CommitTxBatch(2, []*protos.Transaction{tx}, nil, []byte("proof"))
			Expect(err).To(BeNil())
		})
		It("confirms the contents of the snapshot", func() {
			var count = 0
			for snapshot.Next() {
				//_, _ := snapshot.GetRawKeyValue()
				//t.Logf("Key %v, Val %v", k, v)
				count++
			}
			Expect(count).To(Equal(3))
			Expect(snapshot.GetBlockNumber()).To(Equal(0))
		})
	})

	Describe("Ledger PutRawBlock", func() {
		testDBWrapper := db.NewTestDBWrapper()
		testDBWrapper.CreateFreshDBGinkgo()
		ledgerPtr, err := ledger.GetNewLedger()
		if err != nil {
			Fail("failed to get a fresh ledger")
		}
		block := new(protos.Block)
		block.PreviousBlockHash = []byte("foo")
		block.StateHash = []byte("bar")
		It("creates a raw block and puts it in the ledger without error", func() {
			Expect(ledgerPtr.PutRawBlock(block, 4)).To(BeNil())
		})
		It("should return the same block that was stored", func() {
			Expect(ledgerPtr.GetBlockByNumber(4)).To(Equal(block))
		})
		It("creates, populates and finishes a transaction", func() {
			Expect(ledgerPtr.BeginTxBatch(1)).To(BeNil())
			ledgerPtr.TxBegin("txUuid")
			Expect(ledgerPtr.SetState("chaincode1", "key1", []byte("value1"))).To(BeNil())
			ledgerPtr.TxFinished("txUuid", true)
		})
		It("should commit the batch", func() {
			uuid := util.GenerateUUID()
			tx, err := protos.NewTransaction(protos.ChaincodeID{Path: "testUrl"}, uuid, "anyfunction", []string{"param1, param2"})
			Expect(err).To(BeNil())
			err = ledgerPtr.CommitTxBatch(1, []*protos.Transaction{tx}, nil, []byte("proof"))
			Expect(err).To(BeNil())
		})
		It("should have retrieved a block without error", func() {
			previousHash, _ := block.GetHash()
			newBlock, err := ledgerPtr.GetBlockByNumber(5)
			Expect(err).To(BeNil())
			Expect(newBlock.PreviousBlockHash).To(Equal(previousHash))
		})
	})

	Describe("Ledger SetRawState", func() {
		//var hash1, hash2, hash3 []byte
		//var snapshot *state.StateSnapshot
		var hash1 []byte
		var err error
		testDBWrapper := db.NewTestDBWrapper()
		testDBWrapper.CreateFreshDBGinkgo()
		ledgerPtr, err := ledger.GetNewLedger()
		if err != nil {
			Fail("failed to get a fresh ledger")
		}
		It("creates, populates and finishes a batch", func() {
			Expect(ledgerPtr.BeginTxBatch(1)).To(BeNil())
			ledgerPtr.TxBegin("txUuid")
			Expect(ledgerPtr.SetState("chaincode1", "key1", []byte("value1"))).To(BeNil())
			Expect(ledgerPtr.SetState("chaincode2", "key2", []byte("value2"))).To(BeNil())
			Expect(ledgerPtr.SetState("chaincode3", "key3", []byte("value3"))).To(BeNil())
			ledgerPtr.TxFinished("txUuid", true)
		})
		It("should commit the batch", func() {
			uuid := util.GenerateUUID()
			tx, err := protos.NewTransaction(protos.ChaincodeID{Path: "testUrl"}, uuid, "anyfunction", []string{"param1, param2"})
			Expect(err).To(BeNil())
			err = ledgerPtr.CommitTxBatch(1, []*protos.Transaction{tx}, nil, []byte("proof"))
			Expect(err).To(BeNil())
		})
		It("should validate that the state is what was committed", func() {
			// Ensure values are in the DB
			val, err := ledgerPtr.GetState("chaincode1", "key1", true)
			Expect(bytes.Compare(val, []byte("value1"))).To(Equal(0))
			Expect(err).To(BeNil())
			val, err = ledgerPtr.GetState("chaincode2", "key2", true)
			Expect(bytes.Compare(val, []byte("value2"))).To(Equal(0))
			Expect(err).To(BeNil())
			val, err = ledgerPtr.GetState("chaincode3", "key3", true)
			Expect(bytes.Compare(val, []byte("value3"))).To(Equal(0))
			Expect(err).To(BeNil())
		})
		It("should get state hash without error", func() {
			hash1, err = ledgerPtr.GetTempStateHash()
			Expect(err).To(BeNil())
		})
		/*
		It("should get snapshot without error", func() {
			snapshot, err = ledgerPtr.GetStateSnapshot()
			Expect(err).To(BeNil())
			defer snapshot.Release()
		})
		It("creates, populates and finishes a batch", func() {
			Expect(ledgerPtr.BeginTxBatch(2)).To(BeNil())
			ledgerPtr.TxBegin("txUuid2")
			Expect(ledgerPtr.DeleteState("chaincode1", "key1")).To(BeNil())
			Expect(ledgerPtr.DeleteState("chaincode2", "key2")).To(BeNil())
			Expect(ledgerPtr.DeleteState("chaincode3", "key3")).To(BeNil())
			ledgerPtr.TxFinished("txUuid2", true)
		})
		It("should commit the batch", func() {
			uuid := util.GenerateUUID()
			tx, err := protos.NewTransaction(protos.ChaincodeID{Path: "testUrl"}, uuid, "anyfunction", []string{"param1, param2"})
			Expect(err).To(BeNil())
			err = ledgerPtr.CommitTxBatch(2, []*protos.Transaction{tx}, nil, []byte("proof"))
			Expect(err).To(BeNil())
		})
		It("should not return committed state", func() {
			state, _ := ledgerPtr.GetState("chaincode1", "key1", true)
			Expect(state).To(BeNil())
			state, _ = ledgerPtr.GetState("chaincode2", "key2", true)
			Expect(state).To(BeNil())
			state, _ = ledgerPtr.GetState("chaincode3", "key3", true)
			Expect(state).To(BeNil())
		})
		It("should get state hash without error", func() {
			hash2, err = ledgerPtr.GetTempStateHash()
			Expect(err).To(BeNil())
		})
		It("should match the current hash with the previously returned hash", func() {
			Expect(bytes.Compare(hash1, hash2)).To(Equal(0))
		})
		It("creates a delta, applies it and commits it without error", func() {
			// put key/values from the snapshot back in the DB
			//var keys, values [][]byte
			delta := statemgmt.NewStateDelta()
			for i := 0; snapshot.Next(); i++ {
				k, v := snapshot.GetRawKeyValue()
				cID, keyID := statemgmt.DecodeCompositeKey(k)
				delta.Set(cID, keyID, v, nil)
			}
			ledgerPtr.ApplyStateDelta(1, delta)
			ledgerPtr.CommitStateDelta(1)
		})
		It("should return restored state", func() {
			state, _ := ledgerPtr.GetState("chaincode1", "key1", true)
			Expect(state).To(Equal([]byte("value1")))
			state, _ = ledgerPtr.GetState("chaincode2", "key2", true)
			Expect(state).To(Equal([]byte("value2")))
			state, _ = ledgerPtr.GetState("chaincode3", "key3", true)
			Expect(state).To(Equal([]byte("value3")))
		})
		It("should get state hash without error", func() {
			hash3, err = ledgerPtr.GetTempStateHash()
			Expect(err).To(BeNil())
		})
		It("should match the current hash with the originally returned hash", func() {
			Expect(bytes.Compare(hash1, hash3)).To(Equal(0))
		})
		*/
	})

	Describe("Ledger DeleteAllStateKeysAndValues", func() {
		testDBWrapper := db.NewTestDBWrapper()
		testDBWrapper.CreateFreshDBGinkgo()
		ledgerPtr, err := ledger.GetNewLedger()
		if err != nil {
			Fail("failed to get a fresh ledger")
		}
		It("creates, populates and finishes a batch", func() {
			Expect(ledgerPtr.BeginTxBatch(1)).To(BeNil())
			ledgerPtr.TxBegin("txUuid1")
			Expect(ledgerPtr.SetState("chaincode1", "key1", []byte("value1"))).To(BeNil())
			Expect(ledgerPtr.SetState("chaincode2", "key2", []byte("value2"))).To(BeNil())
			Expect(ledgerPtr.SetState("chaincode3", "key3", []byte("value3"))).To(BeNil())
			ledgerPtr.TxFinished("txUuid1", true)
		})
		It("should commit the batch without error", func() {
			uuid := util.GenerateUUID()
			tx, err := protos.NewTransaction(protos.ChaincodeID{Path: "testUrl"}, uuid, "anyfunction", []string{"param1, param2"})
			Expect(err).To(BeNil())
			err = ledgerPtr.CommitTxBatch(1, []*protos.Transaction{tx}, nil, []byte("proof"))
			Expect(err).To(BeNil())
		})
		// Confirm values are present in state
		It("should return committed state", func() {
			state, _ := ledgerPtr.GetState("chaincode1", "key1", true)
			Expect(state).To(Equal([]byte("value1")))
			state, _ = ledgerPtr.GetState("chaincode2", "key2", true)
			Expect(state).To(Equal([]byte("value2")))
			state, _ = ledgerPtr.GetState("chaincode3", "key3", true)
			Expect(state).To(Equal([]byte("value3")))
		})
		// Delete all keys/values
		It("deletes all state, keys and values from ledger without error", func() {
			Expect(ledgerPtr.DeleteALLStateKeysAndValues()).To(BeNil())
		})
		// Confirm values are deleted
		It("should not return committed state", func() {
			state, _ := ledgerPtr.GetState("chaincode1", "key1", true)
			Expect(state).To(BeNil())
			state, _ = ledgerPtr.GetState("chaincode2", "key2", true)
			Expect(state).To(BeNil())
			state, _ = ledgerPtr.GetState("chaincode3", "key3", true)
			Expect(state).To(BeNil())
		})
		// Test that we can now store new stuff in the state
		It("creates, populates and finishes a batch", func() {
			Expect(ledgerPtr.BeginTxBatch(2)).To(BeNil())
			ledgerPtr.TxBegin("txUuid1")
			Expect(ledgerPtr.SetState("chaincode1", "key1", []byte("value1"))).To(BeNil())
			Expect(ledgerPtr.SetState("chaincode2", "key2", []byte("value2"))).To(BeNil())
			Expect(ledgerPtr.SetState("chaincode3", "key3", []byte("value3"))).To(BeNil())
			ledgerPtr.TxFinished("txUuid1", true)
		})
		It("should commit the batch without error", func() {
			uuid := util.GenerateUUID()
			tx, err := protos.NewTransaction(protos.ChaincodeID{Path: "testUrl"}, uuid, "anyfunction", []string{"param1, param2"})
			Expect(err).To(BeNil())
			err = ledgerPtr.CommitTxBatch(2, []*protos.Transaction{tx}, nil, []byte("proof"))
			Expect(err).To(BeNil())
		})
		// Confirm values are present in state
		It("should return committed state", func() {
			state, _ := ledgerPtr.GetState("chaincode1", "key1", true)
			Expect(state).To(Equal([]byte("value1")))
			state, _ = ledgerPtr.GetState("chaincode2", "key2", true)
			Expect(state).To(Equal([]byte("value2")))
			state, _ = ledgerPtr.GetState("chaincode3", "key3", true)
			Expect(state).To(Equal([]byte("value3")))
		})
	})

	Describe("Ledger VerifyChain", func() {
		testDBWrapper := db.NewTestDBWrapper()
		testDBWrapper.CreateFreshDBGinkgo()
		ledgerPtr, err := ledger.GetNewLedger()
		if err != nil {
			Fail("failed to get a fresh ledger")
		}
		// Build a big blockchain
		It("creates, populates, finishes and commits a large blockchain", func() {
			for i := 0; i < 100; i++ {
				Expect(ledgerPtr.BeginTxBatch(i)).To(BeNil())
				ledgerPtr.TxBegin("txUuid" + strconv.Itoa(i))
				Expect(ledgerPtr.SetState("chaincode"+strconv.Itoa(i), "key"+strconv.Itoa(i), []byte("value"+strconv.Itoa(i)))).To(BeNil())
				ledgerPtr.TxFinished("txUuid" + strconv.Itoa(i), true)
				uuid := util.GenerateUUID()
				tx, err := protos.NewTransaction(protos.ChaincodeID{Path: "testUrl"}, uuid, "anyfunction", []string{"param1, param2"})
				Expect(err).To(BeNil())
				err = ledgerPtr.CommitTxBatch(i, []*protos.Transaction{tx}, nil, []byte("proof"))
				Expect(err).To(BeNil())
			}
		})
		It("verifies the blockchain", func() {
			// Verify the chain
			for lowBlock := uint64(0); lowBlock < ledgerPtr.GetBlockchainSize()-1; lowBlock++ {
				Expect(ledgerPtr.VerifyChain(ledgerPtr.GetBlockchainSize()-1, lowBlock)).To(Equal(uint64(0)))
			}
			for highBlock := ledgerPtr.GetBlockchainSize() - 1; highBlock > 0; highBlock-- {
				Expect(ledgerPtr.VerifyChain(highBlock, 0)).To(Equal(uint64(0)))
			}
		})
		It("adds bad blocks to the blockchain", func() {
			// Add bad blocks and test
			badBlock := protos.NewBlock(nil, nil)
			badBlock.PreviousBlockHash = []byte("evil")
			for i := uint64(0); i < ledgerPtr.GetBlockchainSize(); i++ {
				goodBlock, _ := ledgerPtr.GetBlockByNumber(i)
				ledgerPtr.PutRawBlock(badBlock, i)
				for lowBlock := uint64(0); lowBlock < ledgerPtr.GetBlockchainSize()-1; lowBlock++ {
					if i >= lowBlock {
						expected := uint64(i + 1)
						if i == ledgerPtr.GetBlockchainSize()-1 {
							expected--
						}
						Expect(ledgerPtr.VerifyChain(ledgerPtr.GetBlockchainSize()-1, lowBlock)).To(Equal(expected))
					} else {
						Expect(ledgerPtr.VerifyChain(ledgerPtr.GetBlockchainSize()-1, lowBlock)).To(Equal(uint64(0)))
					}
				}
				for highBlock := ledgerPtr.GetBlockchainSize() - 1; highBlock > 0; highBlock-- {
					if i <= highBlock {
						expected := uint64(i + 1)
						if i == highBlock {
							expected--
						}
						Expect(ledgerPtr.VerifyChain(highBlock, 0)).To(Equal(expected))
					} else {
						Expect(ledgerPtr.VerifyChain(highBlock, 0)).To(Equal(uint64(0)))
					}
				}
				Expect(ledgerPtr.PutRawBlock(goodBlock, i)).To(BeNil())
			}
		})
		// Test edge cases
		It("tests some edge cases", func() {
			_, err := ledgerPtr.VerifyChain(2, 10)
			Expect(err).To(Equal("Expected error as high block is less than low block"))
			_, err = ledgerPtr.VerifyChain(2, 2)
			Expect(err).To(Equal("Expected error as high block is equal to low block"))
			_, err = ledgerPtr.VerifyChain(0, 100)
			Expect(err).To(Equal("Expected error as high block is out of bounds"))
		})
	})
	Describe("Ledger BlockNumberOutOfBoundsError", func() {
		testDBWrapper := db.NewTestDBWrapper()
		testDBWrapper.CreateFreshDBGinkgo()
		ledgerPtr, err := ledger.GetNewLedger()
		if err != nil {
			Fail("failed to get a fresh ledger")
		}
		// Build a big blockchain
		It("creates, populates, finishes and commits a large blockchain", func() {
			for i := 0; i < 100; i++ {
				Expect(ledgerPtr.BeginTxBatch(i)).To(BeNil())
				ledgerPtr.TxBegin("txUuid" + strconv.Itoa(i))
				Expect(ledgerPtr.SetState("chaincode"+strconv.Itoa(i), "key"+strconv.Itoa(i), []byte("value"+strconv.Itoa(i)))).To(BeNil())
				ledgerPtr.TxFinished("txUuid" + strconv.Itoa(i), true)
				uuid := util.GenerateUUID()
				tx, err := protos.NewTransaction(protos.ChaincodeID{Path: "testUrl"}, uuid, "anyfunction", []string{"param1, param2"})
				Expect(err).To(BeNil())
				err = ledgerPtr.CommitTxBatch(i, []*protos.Transaction{tx}, nil, []byte("proof"))
				Expect(err).To(BeNil())
			}
		})
		It("forces some ErrOutOfBounds conditions", func() {
			ledgerPtr.GetBlockByNumber(9)
			_, err := ledgerPtr.GetBlockByNumber(10)
			Expect(err).To(Equal(ledger.ErrOutOfBounds))

			ledgerPtr.GetStateDelta(9)
			_, err = ledgerPtr.GetStateDelta(10)
			Expect(err).To(Equal(ledger.ErrOutOfBounds))
		})
	})

	Describe("Ledger RollBackwardsAndForwards", func() {
		testDBWrapper := db.NewTestDBWrapper()
		testDBWrapper.CreateFreshDBGinkgo()
		ledgerPtr, err := ledger.GetNewLedger()
		if err != nil {
			Fail("failed to get a fresh ledger")
		}
		// Block 0
		It("creates, populates and finishes a batch", func() {
			Expect(ledgerPtr.BeginTxBatch(0)).To(BeNil())
			ledgerPtr.TxBegin("txUuid1")
			Expect(ledgerPtr.SetState("chaincode1", "key1", []byte("value1A"))).To(BeNil())
			Expect(ledgerPtr.SetState("chaincode2", "key2", []byte("value2A"))).To(BeNil())
			Expect(ledgerPtr.SetState("chaincode3", "key3", []byte("value3A"))).To(BeNil())
			ledgerPtr.TxFinished("txUuid1", true)
		})
		It("should commit the batch", func() {
			uuid := util.GenerateUUID()
			tx, err := protos.NewTransaction(protos.ChaincodeID{Path: "testUrl"}, uuid, "anyfunction", []string{"param1, param2"})
			Expect(err).To(BeNil())
			err = ledgerPtr.CommitTxBatch(0, []*protos.Transaction{tx}, nil, []byte("proof"))
			Expect(err).To(BeNil())
		})
		It("should return committed state", func() {
			state, _ := ledgerPtr.GetState("chaincode1", "key1", true)
			Expect(state).To(Equal([]byte("value1A")))
			state, _ = ledgerPtr.GetState("chaincode2", "key2", true)
			Expect(state).To(Equal([]byte("value2A")))
			state, _ = ledgerPtr.GetState("chaincode3", "key3", true)
			Expect(state).To(Equal([]byte("value3A")))
		})
		// Block 1
		It("creates, populates and finishes a batch", func() {
			Expect(ledgerPtr.BeginTxBatch(1)).To(BeNil())
			ledgerPtr.TxBegin("txUuid1")
			Expect(ledgerPtr.SetState("chaincode1", "key1", []byte("value1B"))).To(BeNil())
			Expect(ledgerPtr.SetState("chaincode2", "key2", []byte("value2B"))).To(BeNil())
			Expect(ledgerPtr.SetState("chaincode3", "key3", []byte("value3B"))).To(BeNil())
			ledgerPtr.TxFinished("txUuid1", true)
		})
		It("should commit the batch", func() {
			uuid := util.GenerateUUID()
			tx, err := protos.NewTransaction(protos.ChaincodeID{Path: "testUrl"}, uuid, "anyfunction", []string{"param1, param2"})
			Expect(err).To(BeNil())
			err = ledgerPtr.CommitTxBatch(1, []*protos.Transaction{tx}, nil, []byte("proof"))
			Expect(err).To(BeNil())
		})
		It("should return committed state from memory", func() {
			state, _ := ledgerPtr.GetState("chaincode1", "key1", true)
			Expect(state).To(Equal([]byte("value1B")))
			state, _ = ledgerPtr.GetState("chaincode2", "key2", true)
			Expect(state).To(Equal([]byte("value2B")))
			state, _ = ledgerPtr.GetState("chaincode3", "key3", true)
			Expect(state).To(Equal([]byte("value3B")))
		})
		// Block 2
		It("creates, populates and finishes a batch", func() {
			Expect(ledgerPtr.BeginTxBatch(2)).To(BeNil())
			ledgerPtr.TxBegin("txUuid1")
			Expect(ledgerPtr.SetState("chaincode1", "key1", []byte("value1C"))).To(BeNil())
			Expect(ledgerPtr.SetState("chaincode2", "key2", []byte("value2C"))).To(BeNil())
			Expect(ledgerPtr.SetState("chaincode3", "key3", []byte("value3C"))).To(BeNil())
			Expect(ledgerPtr.SetState("chaincode4", "key4", []byte("value4C"))).To(BeNil())
			ledgerPtr.TxFinished("txUuid1", true)
		})
		It("should commit the batch", func() {
			uuid := util.GenerateUUID()
			tx, err := protos.NewTransaction(protos.ChaincodeID{Path: "testUrl"}, uuid, "anyfunction", []string{"param1, param2"})
			Expect(err).To(BeNil())
			err = ledgerPtr.CommitTxBatch(2, []*protos.Transaction{tx}, nil, []byte("proof"))
			Expect(err).To(BeNil())
		})
		It("should return committed state", func() {
			state, _ := ledgerPtr.GetState("chaincode1", "key1", true)
			Expect(state).To(Equal([]byte("value1C")))
			state, _ = ledgerPtr.GetState("chaincode2", "key2", true)
			Expect(state).To(Equal([]byte("value2C")))
			state, _ = ledgerPtr.GetState("chaincode3", "key3", true)
			Expect(state).To(Equal([]byte("value3C")))
			state, _ = ledgerPtr.GetState("chaincode4", "key4", true)
			Expect(state).To(Equal([]byte("value4C")))
		})
		// Roll backwards once
		It("rolls backwards once without error", func() {
			delta2, err := ledgerPtr.GetStateDelta(2)
			Expect(err).To(BeNil())
			delta2.RollBackwards = true
			err = ledgerPtr.ApplyStateDelta(1, delta2)
			Expect(err).To(BeNil())
			err = ledgerPtr.CommitStateDelta(1)
			Expect(err).To(BeNil())
		})
		It("should return committed state", func() {
			state, _ := ledgerPtr.GetState("chaincode1", "key1", true)
			Expect(state).To(Equal([]byte("value1B")))
			state, _ = ledgerPtr.GetState("chaincode2", "key2", true)
			Expect(state).To(Equal([]byte("value2B")))
			state, _ = ledgerPtr.GetState("chaincode3", "key3", true)
			Expect(state).To(Equal([]byte("value3B")))
			state, _ = ledgerPtr.GetState("chaincode4", "key4", true)
			Expect(state).To(BeNil())
		})
		// Now roll forwards once
		It("rolls forwards once without error", func() {
			delta2, err := ledgerPtr.GetStateDelta(2)
			Expect(err).To(BeNil())
			delta2.RollBackwards = false
			err = ledgerPtr.ApplyStateDelta(2, delta2)
			Expect(err).To(BeNil())
			err = ledgerPtr.CommitStateDelta(2)
			Expect(err).To(BeNil())
		})
		It("should return committed state", func() {
			state, _ := ledgerPtr.GetState("chaincode1", "key1", true)
			Expect(state).To(Equal([]byte("value1C")))
			state, _ = ledgerPtr.GetState("chaincode2", "key2", true)
			Expect(state).To(Equal([]byte("value2C")))
			state, _ = ledgerPtr.GetState("chaincode3", "key3", true)
			Expect(state).To(Equal([]byte("value3C")))
			state, _ = ledgerPtr.GetState("chaincode4", "key4", true)
			Expect(state).To(Equal([]byte("value4C")))
		})
		It("rolls backwards twice without error", func() {
			delta2, err := ledgerPtr.GetStateDelta(2)
			Expect(err).To(BeNil())
			delta2.RollBackwards = true
			delta1, err := ledgerPtr.GetStateDelta(1)
			Expect(err).To(BeNil())
			err = ledgerPtr.ApplyStateDelta(3, delta2)
			Expect(err).To(BeNil())
			err = ledgerPtr.CommitStateDelta(3)
			Expect(err).To(BeNil())
			delta1.RollBackwards = true
			err = ledgerPtr.ApplyStateDelta(4, delta1)
			Expect(err).To(BeNil())
			err = ledgerPtr.CommitStateDelta(4)
			Expect(err).To(BeNil())
		})
		It("should return committed state", func() {
			state, _ := ledgerPtr.GetState("chaincode1", "key1", true)
			Expect(state).To(Equal([]byte("value1A")))
			state, _ = ledgerPtr.GetState("chaincode2", "key2", true)
			Expect(state).To(Equal([]byte("value2A")))
			state, _ = ledgerPtr.GetState("chaincode3", "key3", true)
			Expect(state).To(Equal([]byte("value3A")))
		})

		// Now roll forwards twice
		It("rolls forwards twice without error", func() {
			delta2, err := ledgerPtr.GetStateDelta(2)
			Expect(err).To(BeNil())
			delta2.RollBackwards = false
			delta1, err := ledgerPtr.GetStateDelta(1)
			Expect(err).To(BeNil())
			delta1.RollBackwards = false

			err = ledgerPtr.ApplyStateDelta(5, delta1)
			Expect(err).To(BeNil())
			err = ledgerPtr.CommitStateDelta(5)
			Expect(err).To(BeNil())
			delta1.RollBackwards = false
			err = ledgerPtr.ApplyStateDelta(6, delta2)
			Expect(err).To(BeNil())
			err = ledgerPtr.CommitStateDelta(6)
			Expect(err).To(BeNil())
		})
		It("should return committed state", func() {
			state, _ := ledgerPtr.GetState("chaincode1", "key1", true)
			Expect(state).To(Equal([]byte("value1C")))
			state, _ = ledgerPtr.GetState("chaincode2", "key2", true)
			Expect(state).To(Equal([]byte("value2C")))
			state, _ = ledgerPtr.GetState("chaincode3", "key3", true)
			Expect(state).To(Equal([]byte("value3C")))
			state, _ = ledgerPtr.GetState("chaincode4", "key4", true)
			Expect(state).To(Equal([]byte("value4C")))
		})
	})
	Describe("Ledger InvalidOrderDelta", func() {
		testDBWrapper := db.NewTestDBWrapper()
		testDBWrapper.CreateFreshDBGinkgo()
		ledgerPtr, err := ledger.GetNewLedger()
		var delta *statemgmt.StateDelta
		if err != nil {
			Fail("failed to get a fresh ledger")
		}
		// Block 0
		It("creates, populates and finishes a batch", func() {
			Expect(ledgerPtr.BeginTxBatch(0)).To(BeNil())
			ledgerPtr.TxBegin("txUuid1")
			Expect(ledgerPtr.SetState("chaincode1", "key1", []byte("value1A"))).To(BeNil())
			Expect(ledgerPtr.SetState("chaincode2", "key2", []byte("value2A"))).To(BeNil())
			Expect(ledgerPtr.SetState("chaincode3", "key3", []byte("value3A"))).To(BeNil())
			ledgerPtr.TxFinished("txUuid1", true)
		})
		It("should commit the batch", func() {
			uuid := util.GenerateUUID()
			tx, err := protos.NewTransaction(protos.ChaincodeID{Path: "testUrl"}, uuid, "anyfunction", []string{"param1, param2"})
			Expect(err).To(BeNil())
			err = ledgerPtr.CommitTxBatch(0, []*protos.Transaction{tx}, nil, []byte("proof"))
			Expect(err).To(BeNil())
		})
		It("should return committed state", func() {
			state, _ := ledgerPtr.GetState("chaincode1", "key1", true)
			Expect(state).To(Equal([]byte("value1A")))
			state, _ = ledgerPtr.GetState("chaincode2", "key2", true)
			Expect(state).To(Equal([]byte("value2A")))
			state, _ = ledgerPtr.GetState("chaincode3", "key3", true)
			Expect(state).To(Equal([]byte("value3A")))
		})
		// Block 1
		It("creates, populates and finishes a batch", func() {
			Expect(ledgerPtr.BeginTxBatch(1)).To(BeNil())
			ledgerPtr.TxBegin("txUuid1")
			Expect(ledgerPtr.SetState("chaincode1", "key1", []byte("value1B"))).To(BeNil())
			Expect(ledgerPtr.SetState("chaincode2", "key2", []byte("value2B"))).To(BeNil())
			Expect(ledgerPtr.SetState("chaincode3", "key3", []byte("value3B"))).To(BeNil())
			ledgerPtr.TxFinished("txUuid1", true)
		})
		It("should commit the batch", func() {
			uuid := util.GenerateUUID()
			tx, err := protos.NewTransaction(protos.ChaincodeID{Path: "testUrl"}, uuid, "anyfunction", []string{"param1, param2"})
			Expect(err).To(BeNil())
			err = ledgerPtr.CommitTxBatch(1, []*protos.Transaction{tx}, nil, []byte("proof"))
			Expect(err).To(BeNil())
		})
		It("should return committed state", func() {
			state, _ := ledgerPtr.GetState("chaincode1", "key1", true)
			Expect(state).To(Equal([]byte("value1B")))
			state, _ = ledgerPtr.GetState("chaincode2", "key2", true)
			Expect(state).To(Equal([]byte("value2B")))
			state, _ = ledgerPtr.GetState("chaincode3", "key3", true)
			Expect(state).To(Equal([]byte("value3B")))
		})
		It("should return error trying to commit state delta", func() {
			delta, _ = ledgerPtr.GetStateDelta(1)
			Expect(ledgerPtr.CommitStateDelta(1)).ToNot(BeNil())
		})
		It("should return error trying to rollback batch", func() {
			Expect(ledgerPtr.RollbackTxBatch(1)).ToNot(BeNil())
		})
		It("should return error trying to apply state delta", func() {
			Expect(ledgerPtr.ApplyStateDelta(2, delta)).ToNot(BeNil())
			Expect(ledgerPtr.ApplyStateDelta(3, delta)).ToNot(BeNil())
		})
		It("should return error trying to commit state delta", func() {
			Expect(ledgerPtr.CommitStateDelta(3)).ToNot(BeNil())
		})
		It("should return error trying to rollback state delta", func() {
			Expect(ledgerPtr.RollbackStateDelta(3)).ToNot(BeNil())
		})
	})
/*
	Describe("Ledger ApplyDeltaHash", func() {
		testDBWrapper := db.NewTestDBWrapper()
		testDBWrapper.CreateFreshDBGinkgo()
		ledgerPtr, err := ledger.GetNewLedger()
		if err != nil {
			Fail("failed to get a fresh ledger")
		}
		// Block 0
		ledger.BeginTxBatch(0)
		ledger.TxBegin("txUuid1")
		ledger.SetState("chaincode1", "key1", []byte("value1A"))
		ledger.SetState("chaincode2", "key2", []byte("value2A"))
		ledger.SetState("chaincode3", "key3", []byte("value3A"))
		ledger.TxFinished("txUuid1", true)
		transaction, _ := buildTestTx(t)
		ledger.CommitTxBatch(0, []*protos.Transaction{transaction}, nil, []byte("proof"))
		testutil.AssertEquals(t, ledgerTestWrapper.GetState("chaincode1", "key1", true), []byte("value1A"))
		testutil.AssertEquals(t, ledgerTestWrapper.GetState("chaincode2", "key2", true), []byte("value2A"))
		testutil.AssertEquals(t, ledgerTestWrapper.GetState("chaincode3", "key3", true), []byte("value3A"))

		// Block 1
		ledger.BeginTxBatch(1)
		ledger.TxBegin("txUuid1")
		ledger.SetState("chaincode1", "key1", []byte("value1B"))
		ledger.SetState("chaincode2", "key2", []byte("value2B"))
		ledger.SetState("chaincode3", "key3", []byte("value3B"))
		ledger.TxFinished("txUuid1", true)
		transaction, _ = buildTestTx(t)
		ledger.CommitTxBatch(1, []*protos.Transaction{transaction}, nil, []byte("proof"))
		testutil.AssertEquals(t, ledgerTestWrapper.GetState("chaincode1", "key1", true), []byte("value1B"))
		testutil.AssertEquals(t, ledgerTestWrapper.GetState("chaincode2", "key2", true), []byte("value2B"))
		testutil.AssertEquals(t, ledgerTestWrapper.GetState("chaincode3", "key3", true), []byte("value3B"))

		// Block 2
		ledger.BeginTxBatch(2)
		ledger.TxBegin("txUuid1")
		ledger.SetState("chaincode1", "key1", []byte("value1C"))
		ledger.SetState("chaincode2", "key2", []byte("value2C"))
		ledger.SetState("chaincode3", "key3", []byte("value3C"))
		ledger.SetState("chaincode4", "key4", []byte("value4C"))
		ledger.TxFinished("txUuid1", true)
		transaction, _ = buildTestTx(t)
		ledger.CommitTxBatch(2, []*protos.Transaction{transaction}, nil, []byte("proof"))
		testutil.AssertEquals(t, ledgerTestWrapper.GetState("chaincode1", "key1", true), []byte("value1C"))
		testutil.AssertEquals(t, ledgerTestWrapper.GetState("chaincode2", "key2", true), []byte("value2C"))
		testutil.AssertEquals(t, ledgerTestWrapper.GetState("chaincode3", "key3", true), []byte("value3C"))
		testutil.AssertEquals(t, ledgerTestWrapper.GetState("chaincode4", "key4", true), []byte("value4C"))

		hash2 := ledgerTestWrapper.GetTempStateHash()

		// Roll backwards once
		delta2 := ledgerTestWrapper.GetStateDelta(2)
		delta2.RollBackwards = true
		ledgerTestWrapper.ApplyStateDelta(1, delta2)

		preHash1 := ledgerTestWrapper.GetTempStateHash()
		testutil.AssertNotEquals(t, preHash1, hash2)

		ledgerTestWrapper.CommitStateDelta(1)

		hash1 := ledgerTestWrapper.GetTempStateHash()
		testutil.AssertEquals(t, preHash1, hash1)
		testutil.AssertNotEquals(t, hash1, hash2)

		// Roll forwards once
		delta2.RollBackwards = false
		ledgerTestWrapper.ApplyStateDelta(2, delta2)
		preHash2 := ledgerTestWrapper.GetTempStateHash()
		testutil.AssertEquals(t, preHash2, hash2)
		ledgerTestWrapper.RollbackStateDelta(2)
		preHash2 = ledgerTestWrapper.GetTempStateHash()
		testutil.AssertEquals(t, preHash2, hash1)
		ledgerTestWrapper.ApplyStateDelta(3, delta2)
		preHash2 = ledgerTestWrapper.GetTempStateHash()
		testutil.AssertEquals(t, preHash2, hash2)
		ledgerTestWrapper.CommitStateDelta(3)
		preHash2 = ledgerTestWrapper.GetTempStateHash()
		testutil.AssertEquals(t, preHash2, hash2)
	})

	Describe("Ledger PreviewTXBatchBlock", func() {
		testDBWrapper := db.NewTestDBWrapper()
		testDBWrapper.CreateFreshDBGinkgo()
		ledgerPtr, err := ledger.GetNewLedger()
		if err != nil {
			Fail("failed to get a fresh ledger")
		}
		// Block 0
		ledger.BeginTxBatch(0)
		ledger.TxBegin("txUuid1")
		ledger.SetState("chaincode1", "key1", []byte("value1A"))
		ledger.SetState("chaincode2", "key2", []byte("value2A"))
		ledger.SetState("chaincode3", "key3", []byte("value3A"))
		ledger.TxFinished("txUuid1", true)
		transaction, _ := buildTestTx(t)

		previewBlockInfo, err := ledger.GetTXBatchPreviewBlockInfo(0, []*protos.Transaction{transaction}, []byte("proof"))
		testutil.AssertNoError(t, err, "Error fetching preview block info.")

		ledger.CommitTxBatch(0, []*protos.Transaction{transaction}, nil, []byte("proof"))
		commitedBlockInfo, err := ledger.GetBlockchainInfo()
		testutil.AssertNoError(t, err, "Error fetching committed block hash.")

		testutil.AssertEquals(t, previewBlockInfo, commitedBlockInfo)
	})

	Describe("Ledger GetTransactionByUUID", func() {
		testDBWrapper := db.NewTestDBWrapper()
		testDBWrapper.CreateFreshDBGinkgo()
		ledgerPtr, err := ledger.GetNewLedger()
		if err != nil {
			Fail("failed to get a fresh ledger")
		}
		// Block 0
		ledger.BeginTxBatch(0)
		ledger.TxBegin("txUuid1")
		ledger.SetState("chaincode1", "key1", []byte("value1A"))
		ledger.SetState("chaincode2", "key2", []byte("value2A"))
		ledger.SetState("chaincode3", "key3", []byte("value3A"))
		ledger.TxFinished("txUuid1", true)
		transaction, uuid := buildTestTx(t)
		ledger.CommitTxBatch(0, []*protos.Transaction{transaction}, nil, []byte("proof"))

		ledgerTransaction, err := ledger.GetTransactionByUUID(uuid)
		testutil.AssertNoError(t, err, "Error fetching transaction by UUID.")
		testutil.AssertEquals(t, transaction, ledgerTransaction)

		ledgerTransaction, err = ledger.GetTransactionByUUID("InvalidUUID")
		testutil.AssertEquals(t, err, ErrResourceNotFound)
		testutil.AssertNil(t, ledgerTransaction)
	})

	Describe("Ledger TransactionResult", func() {
		testDBWrapper := db.NewTestDBWrapper()
		testDBWrapper.CreateFreshDBGinkgo()
		ledgerPtr, err := ledger.GetNewLedger()
		if err != nil {
			Fail("failed to get a fresh ledger")
		}
		// Block 0
		ledger.BeginTxBatch(0)
		ledger.TxBegin("txUuid1")
		ledger.SetState("chaincode1", "key1", []byte("value1A"))
		ledger.SetState("chaincode2", "key2", []byte("value2A"))
		ledger.SetState("chaincode3", "key3", []byte("value3A"))
		ledger.TxFinished("txUuid1", true)
		transaction, uuid := buildTestTx(t)

		transactionResult := &protos.TransactionResult{Uuid: uuid, ErrorCode: 500, Error: "bad"}

		ledger.CommitTxBatch(0, []*protos.Transaction{transaction}, []*protos.TransactionResult{transactionResult}, []byte("proof"))

		block := ledgerTestWrapper.GetBlockByNumber(0)

		nonHashData := block.GetNonHashData()
		if nonHashData == nil {
			t.Fatal("Expected block to have non hash data, but non hash data was nil.")
		}

		if nonHashData.TransactionResults == nil || len(nonHashData.TransactionResults) == 0 {
			t.Fatal("Expected block to have non hash data transaction results.")
		}

		testutil.AssertEquals(t, nonHashData.TransactionResults[0].Uuid, uuid)
		testutil.AssertEquals(t, nonHashData.TransactionResults[0].Error, "bad")
		testutil.AssertEquals(t, nonHashData.TransactionResults[0].ErrorCode, uint32(500))

	})

	Describe("Ledger RangeScanIterator", func() {
		testDBWrapper := db.NewTestDBWrapper()
		testDBWrapper.CreateFreshDBGinkgo()
		ledgerPtr, err := ledger.GetNewLedger()
		if err != nil {
			Fail("failed to get a fresh ledger")
		}
		///////// Test with an empty Ledger //////////
		//////////////////////////////////////////////
		itr, _ := ledger.GetStateRangeScanIterator("chaincodeID2", "key2", "key5", false)
		statemgmt.AssertIteratorContains(t, itr, map[string][]byte{})
		itr.Close()

		itr, _ = ledger.GetStateRangeScanIterator("chaincodeID2", "key2", "key5", true)
		statemgmt.AssertIteratorContains(t, itr, map[string][]byte{})
		itr.Close()

		// Commit initial data to ledger
		ledger.BeginTxBatch(0)
		ledger.TxBegin("txUuid1")
		ledger.SetState("chaincodeID1", "key1", []byte("value1"))

		ledger.SetState("chaincodeID2", "key1", []byte("value1"))
		ledger.SetState("chaincodeID2", "key2", []byte("value2"))
		ledger.SetState("chaincodeID2", "key3", []byte("value3"))

		ledger.SetState("chaincodeID3", "key1", []byte("value1"))

		ledger.SetState("chaincodeID4", "key1", []byte("value1"))
		ledger.SetState("chaincodeID4", "key2", []byte("value2"))
		ledger.SetState("chaincodeID4", "key3", []byte("value3"))
		ledger.SetState("chaincodeID4", "key4", []byte("value4"))
		ledger.SetState("chaincodeID4", "key5", []byte("value5"))
		ledger.SetState("chaincodeID4", "key6", []byte("value6"))
		ledger.SetState("chaincodeID4", "key7", []byte("value7"))

		ledger.SetState("chaincodeID5", "key1", []byte("value5"))
		ledger.SetState("chaincodeID6", "key1", []byte("value6"))

		ledger.TxFinished("txUuid1", true)
		transaction, _ := buildTestTx(t)
		ledger.CommitTxBatch(0, []*protos.Transaction{transaction}, nil, []byte("proof"))

		// Add new keys and modify existing keys in on-going tx-batch
		ledger.BeginTxBatch(1)
		ledger.TxBegin("txUuid1")
		ledger.SetState("chaincodeID4", "key2", []byte("value2_new"))
		ledger.DeleteState("chaincodeID4", "key3")
		ledger.SetState("chaincodeID4", "key8", []byte("value8_new"))

		///////////////////// Test with committed=true ///////////
		//////////////////////////////////////////////////////////
		// test range scan for chaincodeID4
		itr, _ = ledger.GetStateRangeScanIterator("chaincodeID4", "key2", "key5", true)
		statemgmt.AssertIteratorContains(t, itr,
			map[string][]byte{
				"key2": []byte("value2"),
				"key3": []byte("value3"),
				"key4": []byte("value4"),
				"key5": []byte("value5"),
			})
		itr.Close()

		// test with empty start-key
		itr, _ = ledger.GetStateRangeScanIterator("chaincodeID4", "", "key5", true)
		statemgmt.AssertIteratorContains(t, itr,
			map[string][]byte{
				"key1": []byte("value1"),
				"key2": []byte("value2"),
				"key3": []byte("value3"),
				"key4": []byte("value4"),
				"key5": []byte("value5"),
			})
		itr.Close()

		// test with empty end-key
		itr, _ = ledger.GetStateRangeScanIterator("chaincodeID4", "", "", true)
		statemgmt.AssertIteratorContains(t, itr,
			map[string][]byte{
				"key1": []byte("value1"),
				"key2": []byte("value2"),
				"key3": []byte("value3"),
				"key4": []byte("value4"),
				"key5": []byte("value5"),
				"key6": []byte("value6"),
				"key7": []byte("value7"),
			})
		itr.Close()

		///////////////////// Test with committed=false ///////////
		//////////////////////////////////////////////////////////
		// test range scan for chaincodeID4
		itr, _ = ledger.GetStateRangeScanIterator("chaincodeID4", "key2", "key5", false)
		statemgmt.AssertIteratorContains(t, itr,
			map[string][]byte{
				"key2": []byte("value2_new"),
				"key4": []byte("value4"),
				"key5": []byte("value5"),
			})
		itr.Close()

		// test with empty start-key
		itr, _ = ledger.GetStateRangeScanIterator("chaincodeID4", "", "key5", false)
		statemgmt.AssertIteratorContains(t, itr,
			map[string][]byte{
				"key1": []byte("value1"),
				"key2": []byte("value2_new"),
				"key4": []byte("value4"),
				"key5": []byte("value5"),
			})
		itr.Close()

		// test with empty end-key
		itr, _ = ledger.GetStateRangeScanIterator("chaincodeID4", "", "", false)
		statemgmt.AssertIteratorContains(t, itr,
			map[string][]byte{
				"key1": []byte("value1"),
				"key2": []byte("value2_new"),
				"key4": []byte("value4"),
				"key5": []byte("value5"),
				"key6": []byte("value6"),
				"key7": []byte("value7"),
				"key8": []byte("value8_new"),
			})
		itr.Close()
	})

	Describe("Ledger GetSetMultipleKeys", func() {
		testDBWrapper := db.NewTestDBWrapper()
		testDBWrapper.CreateFreshDBGinkgo()
		ledgerPtr, err := ledger.GetNewLedger()
		if err != nil {
			Fail("failed to get a fresh ledger")
		}
	})

	Describe("Ledger CopyState", func() {
		testDBWrapper := db.NewTestDBWrapper()
		testDBWrapper.CreateFreshDBGinkgo()
		ledgerPtr, err := ledger.GetNewLedger()
		if err != nil {
			Fail("failed to get a fresh ledger")
		}
	})

	Describe("Ledger EmptyArrayValue", func() {
		testDBWrapper := db.NewTestDBWrapper()
		testDBWrapper.CreateFreshDBGinkgo()
		ledgerPtr, err := ledger.GetNewLedger()
		if err != nil {
			Fail("failed to get a fresh ledger")
		}
	})

	Describe("Ledger InvalidInput", func() {
		testDBWrapper := db.NewTestDBWrapper()
		testDBWrapper.CreateFreshDBGinkgo()
		ledgerPtr, err := ledger.GetNewLedger()
		if err != nil {
			Fail("failed to get a fresh ledger")
		}
	})
	*/
})
