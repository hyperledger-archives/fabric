package core

import (
	"fmt"
//	"io"
//	"github.com/op/go-logging"
//	"github.com/golang/protobuf/proto"
//	"github.com/hyperledger/fabric/core/util"
	pb "github.com/hyperledger/fabric/protos"
	context "golang.org/x/net/context"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
//	"sync"
	"time"
)

var neededEndorsementCount = 2

// Endorser Server
type endorserServer struct{}

var e *endorserServer

// NewEndorserServer creates and returns a new Endorser server instance.
func NewEndorserServer() *endorserServer {
	if e == nil {
		e = new(endorserServer)
		return e
	}
	panic("Endorser server is a singleton. It must be instantiated only once.")
}

func (e endorserServer) Propose(ctx context.Context, tp *pb.TransactionProposal) (*pb.ProposalResponse, error) {
	response := checkTxProposal(tp)
	return response, nil
}

func checkTxProposal(tp *pb.TransactionProposal) (*pb.ProposalResponse) {
	return &pb.ProposalResponse{Response: &pb.ProposalResponse_Valid{}}
}

// Get endorsements and check if there are enough endorsers
func CheckEndorsements(transaction *pb.Transaction) ([]*pb.ProposalResponse, error) {
	responses := getEndorsements(transaction)
	if len(responses) < neededEndorsementCount {
		return nil, fmt.Errorf("There is a problem with endorsements. Not enough endorsements.")
	}
	return responses, nil
}

func getEndorsements(transaction *pb.Transaction) ([]*pb.ProposalResponse) {
	responses := make(chan *pb.ProposalResponse)
	timeoutch := make(chan bool)
	signal := make(chan bool)
	endorsers := getEndorsers()
	endorsernum := len(endorsers)
	for _, epeer := range endorsers {
		go sendReqAndWaitForEndorsement(epeer, transaction, responses, signal)
	}
	responsearr := make([]*pb.ProposalResponse, 0, len(endorsers))

	select {
	case resp := <- responses:
        	responsearr = append(responsearr, resp)
		if len(responsearr) >= endorsernum {
			break
		}
	case <- timeoutch:
		signal <- true
		break
	}
	return responsearr
}

func getEndorsers() ([]pb.EndorserClient) {
	// TODO: this should not be hardcoded
	endorserIps := viper.GetStringSlice("endorsement.endorsers")
	endorserClients := make([]pb.EndorserClient, 0, len(endorserIps))
	for _, ip := range endorserIps {
		conn, err := grpc.Dial(ip)
		if nil == err {
			endorserClients = append(endorserClients, pb.NewEndorserClient(conn))
		}
	}
	return []pb.EndorserClient{}
}

func sendReqAndWaitForEndorsement(epeer pb.EndorserClient, tx *pb.Transaction, ch chan *pb.ProposalResponse, signal chan bool) {
        resp, err := epeer.Propose(context.Background(), &pb.TransactionProposal{})
	_, gotexitsignal := <- signal
	if !gotexitsignal && nil == err {
		ch <- resp
	}
}

func timeoutSignal(timeout chan bool) {
	time.Sleep(time.Second * 10)
	timeout <- true
}
