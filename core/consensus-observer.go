package core

import (
	"fmt"

	"github.com/golang/protobuf/proto"
	capi "github.com/hyperledger/fabric/consensus/api"
	"github.com/hyperledger/fabric/core/chaincode"
	"github.com/hyperledger/fabric/core/ledger"
	pb "github.com/hyperledger/fabric/protos"
	"golang.org/x/net/context"
)

type consensusObserver struct {
	incoming chan *pb.Deliver
	control  chan error
}

var c *consensusObserver

// ObserveConsensus creates a client that is able to observe the consensus
func ObserveConsensus() *consensusObserver {
	if c == nil {
		c = new(consensusObserver)
		c.incoming = make(chan *pb.Deliver)
		c.control = make(chan error)
		go observe(c.incoming, c.control)
		go main(c.incoming, c.control)
		return c
	}
	panic("Consensus Observer is a singleton. It must be instantiated only once.")
}

func observe(incoming chan *pb.Deliver, control chan error) {
	err := capi.Observe(incoming)
	control <- err
}

func main(incoming chan *pb.Deliver, control chan error) {
	for {
		select {
		case deliver := <-incoming:
			handleDeliver(deliver)
		case err := <-control:
			if err != nil {
				panic(fmt.Sprintf("Error in consenter connection: %s", err))
			}
			panic("The consenter exited and closed the connection.")
		}
	}
}

func handleDeliver(deliver *pb.Deliver) {
	devopsLogger.Info("We have received a new consensus.")
	newTx := &pb.Transaction{}
	err := proto.Unmarshal(deliver.Blob.Proposal.TxContent, newTx)
	if err != nil {
		devopsLogger.Error("Received consensus has encoding problems: %s", err)
        return
	}
	txs := []*pb.Transaction{newTx}
	_, ccevents, txerrs, err := chaincode.ExecuteTransactions(context.Background(), chaincode.DefaultChain, txs)
	if err != nil {
		devopsLogger.Warning("An error happened while executing the TXs: %s", err)
	}

	//copy errs to results
	txresults := make([]*pb.TransactionResult, len(txerrs))

	//process errors for each transaction
	for i, e := range txerrs {
		//NOTE- it'll be nice if we can have error values. For now success == 0, error == 1
		if txerrs[i] != nil {
			txresults[i] = &pb.TransactionResult{Uuid: txs[i].Uuid, Error: e.Error(), ErrorCode: 1, ChaincodeEvent: ccevents[i]}
		} else {
			txresults[i] = &pb.TransactionResult{Uuid: txs[i].Uuid, ChaincodeEvent: ccevents[i]}
		}
	}
	// CON-API: we don't have 'id' here so we use c, the consensus-observer itself. Note: id =/= txId
	// CON-API: we don't have block metadata here so we use an empty array of bytes
	_, err = commit(c, []byte{}, txs, txresults)
	if err != nil {
		panic(fmt.Sprintf("Serious problem occured when fabric tried to write the ledger: %s", err))
	}
}

func commit(id interface{}, metadata []byte, curBatch []*pb.Transaction, curBatchErrs []*pb.TransactionResult) (*pb.Block, error) {
	devopsLogger.Info("Comitting to the ledger...")
	ledger, err := ledger.GetLedger()
	if err != nil {
		return nil, fmt.Errorf("Failed to get the ledger: %v", err)
	}
	ledger.BeginTxBatch(id)
	// TODO fix this one the ledger has been fixed to implement
	if err := ledger.CommitTxBatch(id, curBatch, curBatchErrs, metadata); err != nil {
		return nil, fmt.Errorf("Failed to commit transaction to the ledger: %v", err)
	}

	size := ledger.GetBlockchainSize()

	block, err := ledger.GetBlockByNumber(size - 1)
	if err != nil {
		return nil, fmt.Errorf("Failed to get the block at the head of the chain: %v", err)
	}

	devopsLogger.Debugf("Committed block with %d transactions, intended to include %d", len(block.Transactions), len(curBatch))

	return block, nil
}
