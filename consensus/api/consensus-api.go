package consensus

import (
	"fmt"
	"io"
	"github.com/op/go-logging"
//	"github.com/golang/protobuf/proto"
//	"github.com/hyperledger/fabric/core/util"
	pb "github.com/hyperledger/fabric/protos"
//	context "golang.org/x/net/context"
	"github.com/hyperledger/fabric/core/crypto"
	"github.com/spf13/viper"
	context "golang.org/x/net/context"
	"google.golang.org/grpc"
	"sync"
    //	"time"
)

var logger *logging.Logger // package-level logger

func init() {
	logger = logging.MustGetLogger("consensus/handler")
}

// Consensus API
type consensusAPI struct {
	consToSend  chan *pb.Deliver
	streamList  *streamList
    secHelper   crypto.Peer
    engine      ConsensusEngine
}

var c *consensusAPI

type ConsensusEngine interface {
    ProcessTransactionMsg(msg *pb.Message, tx *pb.Transaction) (response *pb.Response)
}

// NewConsensusAPIServer creates and returns a new Consensus API server instance.
func NewConsensusAPIServer(engineGetterFunc func() ConsensusEngine, secHelperFunc func() crypto.Peer) *consensusAPI {
	if c == nil {
		c = new(consensusAPI)
		c.consToSend = make(chan *pb.Deliver)
		c.streamList = newStreamList()
		c.secHelper = secHelperFunc()
        c.engine = engineGetterFunc()
        return c
	}
	panic("Consensus API is a singleton. It must be instantiated only once.")
}

// ConsentData sends data to the consensus to achieve an agreement with other nodes.
func (c *consensusAPI) Chat(stream pb.Consensus_ChatServer) error {
	logger.Info("Getting a consensus stream")
	c.streamList.add(stream)
	for {
		broadcast, err := stream.Recv()
		if err == io.EOF {
			logger.Debug("Received EOF, ending Consensus Chat")
			return nil
		}
		if err != nil {
			e := fmt.Errorf("Error during Consensus Chat, stopping: %s", err)
			return e
		}
		err = c.handleBroadcastMessageWithVerification(broadcast)
		if err != nil {
			logger.Errorf("Error handling message: %s", err)
			return err
		}
	}
}

// handleMessage handles messages
func (c *consensusAPI) handleBroadcastMessageWithVerification(broadcast *pb.Broadcast) error {
	// verification
    // Verify transaction signature if security is enabled
    secHelper := c.secHelper
    if nil != secHelper {
        logger.Debugf("Verifying signature...")
        //if broadcast, err = secHelper.TransactionPreValidation(); err != nil {
        //    logger.Errorf("Failed to verify transaction %v", err)
        //    return fmt.Error("Failed to verify transaction.")
        //}
    }
    return c.HandleBroadcastMessage(broadcast)
}

// HandleBroadcastMessage handles a broadcast that asks for a consensus
func (c *consensusAPI) HandleBroadcastMessage(broadcast *pb.Broadcast) error {
    // time := util.CreateUtcTimestamp()
    payload := broadcast.Proposal.TxContent
    tx := &pb.Transaction{Type: pb.Transaction_CHAINCODE_INVOKE, Payload: payload, Uuid: "temporaryID"}
    msg := &pb.Message{Type: pb.Message_CHAIN_TRANSACTION, Payload: payload, Timestamp: nil}
	logger.Debugf("Sending message %s with timestamp %v to local engine", msg.Type, msg.Timestamp)
	c.engine.ProcessTransactionMsg(msg, tx)
	return nil
}

// HandleBroadcastMessage handles a broadcast that asks for a consensus
func HandleBroadcastMessage(broadcast *pb.Broadcast) error {
	return c.HandleBroadcastMessage(broadcast)
}

// SendNewConsensusToClients sends a signal of a newly made consensus to the observing clients.
func SendNewConsensusToClients(consensus *pb.Deliver) error {
        logger.Info("Sending consensus to clients.")
	for stream, _ := range c.streamList.m {
		err := stream.Send(consensus)
		if nil != err {
			logger.Error("Problem with sending to %s", stream)
		}
	}
	return nil
}

type streamList struct {
	m map[pb.Consensus_ChatServer]bool
	sync.RWMutex
}

func newStreamList() *streamList {
	s := new(streamList)
	s.m = make(map[pb.Consensus_ChatServer]bool)
	return s
}

func (s *streamList) add(k pb.Consensus_ChatServer) {
	s.Lock()
	defer s.Unlock()
	s.m[k] = true
}

func (s *streamList) del(k pb.Consensus_ChatServer) {
	s.Lock()
	defer s.Unlock()
	delete(s.m, k)
}

// Broadcast sends a broadcast message to consenters
func SendBroadcastMessage(broadcast *pb.Broadcast) error {
    var conn *grpc.ClientConn
    var err error
    consenters := getConsenters()
    for _, ip := range consenters {
        conn, err = grpc.Dial(ip)
        if err == nil {
            break
        }
    }
    if err != nil {
        return err
    }

    consclient := pb.NewConsensusClient(conn)
    s, err := consclient.Chat(context.Background())
    if err != nil {
        return err
    }
    s.Send(broadcast)
    return nil
}

func getConsenters() ([]string) {
	return viper.GetStringSlice("consensus.consenters")
}

