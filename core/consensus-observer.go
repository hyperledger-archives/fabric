package core

import (
	"fmt"
	pb "github.com/hyperledger/fabric/protos"
	capi "github.com/hyperledger/fabric/consensus/api"
	"github.com/hyperledger/fabric/core/chaincode"
	"github.com/hyperledger/fabric/core/ledger"
)

type consensusObserver struct {
	incoming chan *pb.Deliver
	control  chan error
}

var c *consensusObserver

// NewConsensusObserver creates a client that is able to observe the consensus
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
		case deliver := <- incoming:
			handleDeliver(deliver)
		case err := <- control:
			if err != nil {
				panic(fmt.Sprintf("Error in consenter connection: %s", err))
			}
			panic("The consenter exited and closed the connection.")
		}
	}
}

func handleDeliver(deliver *pb.Deliver) {
    	_, ccevents, txerrs, _ := chaincode.ExecuteTransactions(context.Background(), chaincode.DefaultChain, txs)

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
	_, err := commit(id, metadata, txs, txresults)
	if err != nil {
		panic(fmt.Sprintf("Serious problem occured when fabric tried to write the ledger: %s", err))
	}
}

func commit(id interface{}, metadata []byte, curBatch []*pb.Transaction, curBatchErrs []*pb.TransactionResult) (*pb.Block, error) {
    	ledger, err := ledger.GetLedger()
	if err != nil {
		return nil, fmt.Errorf("Failed to get the ledger: %v", err)
	}
	// TODO fix this one the ledger has been fixed to implement
	if err := ledger.CommitTxBatch(id, curBatch, curBatchErrs, metadata); err != nil {
		return nil, fmt.Errorf("Failed to commit transaction to the ledger: %v", err)
	}

	size := ledger.GetBlockchainSize()

	block, err := ledger.GetBlockByNumber(size - 1)
	if err != nil {
		return nil, fmt.Errorf("Failed to get the block at the head of the chain: %v", err)
	}

	logger.Debugf("Committed block with %d transactions, intended to include %d", len(block.Transactions), len(h.curBatch))

	return block, nil
}
