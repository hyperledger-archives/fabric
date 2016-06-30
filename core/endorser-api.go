package core

import (
//	"fmt"
//	"io"
//	"github.com/op/go-logging"
//	"github.com/golang/protobuf/proto"
//	"github.com/hyperledger/fabric/core/util"
	pb "github.com/hyperledger/fabric/protos"
	context "golang.org/x/net/context"
//	"sync"
//	"time"
)

func Propose(ctx context.Context, tp *pb.TransactionProposal) (*pb.ProposalResponse, error) {
	response := checkTxProposal(tp)
	return response, nil
}

func checkTxProposal(tp *pb.TransactionProposal) (*pb.ProposalResponse) {
	return &pb.ProposalResponse{Response: &pb.ProposalResponse_Valid{}}
}
