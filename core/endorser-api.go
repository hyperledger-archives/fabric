package core

import (
	"fmt"
	//	"io"
	//	"github.com/op/go-logging"
	//	"github.com/golang/protobuf/proto"
	//	"github.com/hyperledger/fabric/core/util"
	pb "github.com/hyperledger/fabric/protos"
	"github.com/spf13/viper"
	context "golang.org/x/net/context"
	"google.golang.org/grpc"
	//	"sync"
	"time"
)

var neededEndorsementCount = 1
var defaultTimeout = time.Second * 3

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

func checkTxProposal(tp *pb.TransactionProposal) *pb.ProposalResponse {
	devopsLogger.Info("Checking proposal...")
	return &pb.ProposalResponse{Response: &pb.ProposalResponse_Valid{Valid: &pb.TransactionValid{EndorsingSignature: []byte{}}}}
}

// CheckEndorsements gets endorsements and checks if there are enough endorsers
func CheckEndorsements(transaction *pb.Transaction) ([][]byte, error) {
	responses := getEndorsements(transaction)
	if len(responses) < neededEndorsementCount {
		return nil, fmt.Errorf("There is a problem with endorsements. Not enough endorsements.")
	}
	endorsements := make([][]byte, 0, len(responses))
	for _, resp := range responses {
		r, ok := resp.Response.(*pb.ProposalResponse_Valid)
		if ok {
			endorsement := r.Valid.EndorsingSignature
			endorsements = append(endorsements, endorsement)
		}
	}
	return endorsements, nil
}

func getEndorsements(transaction *pb.Transaction) []*pb.ProposalResponse {
	timeoutch := make(chan bool, 1)
	signal := make(chan bool, 1)
	endorsers := getEndorsers()
	endorsernum := len(endorsers)
	responses := make(chan *pb.ProposalResponse, endorsernum)
	go timeoutSignal(timeoutch)
	for _, epeer := range endorsers {
		go sendReqAndWaitForEndorsement(epeer, transaction, responses, signal)
	}
	responsearr := make([]*pb.ProposalResponse, 0, len(endorsers))

WAITING:
	for {
		select {
		case resp := <-responses:
			devopsLogger.Info("1")
			responsearr = append(responsearr, resp)
			if len(responsearr) >= endorsernum {
				break WAITING
			}
		case <-timeoutch:
			devopsLogger.Info("Endorsement timed out...")
			signal <- true
			devopsLogger.Info("2")
			break WAITING
		}
	}
	return responsearr
}

func getEndorsers() []pb.EndorserClient {
	// TODO: this should not be hardcoded
	endorserIps := viper.GetStringSlice("endorsement.endorsers")
	devopsLogger.Info("Endorsers: %s", endorserIps)
	endorserClients := make([]pb.EndorserClient, 0, len(endorserIps))
	tlsEnabled := false
	var opts []grpc.DialOption
	if tlsEnabled {
		opts = append(opts, grpc.WithTransportCredentials(nil))
	} else {
		opts = append(opts, grpc.WithInsecure())
	}
	opts = append(opts, grpc.WithTimeout(defaultTimeout))
	for _, ip := range endorserIps {
		conn, err := grpc.Dial(ip, opts...)
		if nil == err {
			endorserClients = append(endorserClients, pb.NewEndorserClient(conn))
		}
		devopsLogger.Info("Endorser says: %s", err)
	}
	devopsLogger.Info("Endorser connections: %s", endorserClients)
	return endorserClients
}

func sendReqAndWaitForEndorsement(epeer pb.EndorserClient, tx *pb.Transaction, ch chan *pb.ProposalResponse, signal chan bool) {
	proposal := &pb.TransactionProposal{}
	resp, err := epeer.Propose(context.Background(), proposal)
	select {
	case <-signal:
		break
	default:
		if nil == err {
			devopsLogger.Debug("Endorsement received.")
			ch <- resp
		}
	}
	devopsLogger.Debug("No endorsement received received from %s", epeer)
}

func timeoutSignal(timeout chan bool) {
	devopsLogger.Debug("Endorsement timeout starting...")
	time.Sleep(time.Second * 10)
	devopsLogger.Debug("Endorsement timeout is over...")
	timeout <- true
}
