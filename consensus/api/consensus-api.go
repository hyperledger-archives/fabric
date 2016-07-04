package consensus

import (
	"fmt"
	"github.com/golang/protobuf/proto"
	"github.com/op/go-logging"
	"io"
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
	internalStream chan *pb.Deliver
	streamList     *streamList
	secHelper      crypto.Peer
	engine         ConsensusEngine
}

var c *consensusAPI

type ConsensusEngine interface {
	ProcessTransactionMsg(msg *pb.Message, tx *pb.Transaction) (response *pb.Response)
}

// NewConsensusAPIServer creates and returns a new Consensus API server instance.
func NewConsensusAPIServer(engineGetterFunc func() ConsensusEngine, secHelperFunc func() crypto.Peer) *consensusAPI {
	if c == nil {
		c = new(consensusAPI)
		c.internalStream = make(chan *pb.Deliver)
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
		err = handleBroadcastMessageWithVerification(broadcast)
		if err != nil {
			logger.Errorf("Error handling message: %s", err)
			return err
		}
	}
}

// handleBroadcastMessageWithVerification handles messages after verification
// This one needs to be called if we receive a broadcast from some other peer
func handleBroadcastMessageWithVerification(broadcast *pb.Broadcast) error {
	// verification
	// Verify transaction signature if security is enabled
	if nil != c.secHelper {
		logger.Debugf("Verifying signature...")
		//if broadcast, err = secHelper.TransactionPreValidation(); err != nil {
		//    logger.Errorf("Failed to verify transaction %v", err)
		//    return fmt.Error("Failed to verify transaction.")
		//}
	}
	return HandleBroadcastMessage(broadcast)
}

// HandleBroadcastMessage handles a broadcast that asks for a consensus
// This one is enough to be called if we want to handle a local, own broadcast message
func HandleBroadcastMessage(broadcast *pb.Broadcast) error {
	// time := util.CreateUtcTimestamp()
	payload := broadcast.Proposal.TxContent
	tx := &pb.Transaction{Type: pb.Transaction_CHAINCODE_INVOKE, Payload: payload, Uuid: "temporaryID"}
	txbytes, err := proto.Marshal(tx)
	if nil != err {
		return err
	}
	msg := &pb.Message{Type: pb.Message_CHAIN_TRANSACTION, Payload: txbytes, Timestamp: nil}
	logger.Debugf("Sending message %s with timestamp %v to local engine", msg.Type, msg.Timestamp)
	response := c.engine.ProcessTransactionMsg(msg, tx)
	if response.Status == pb.Response_FAILURE {
		return fmt.Errorf(string(response.Msg))
	}
	return nil
}

// SendNewConsensusToClients sends a signal of a newly made consensus to the observing clients.
func SendNewConsensusToClients(consensus *pb.Deliver) error {
	logger.Info("Sending consensus to clients.")
	c.internalStream <- consensus
	for stream, _ := range c.streamList.m {
		err := stream.Send(consensus)
		if nil != err {
			logger.Errorf("Problem with sending to %s", stream)
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

// -------------------------------------------------------
// Client-side calls

// Broadcast sends a broadcast message to consenters
func SendBroadcastMessage(broadcast *pb.Broadcast) error {
	if c != nil {
		return HandleBroadcastMessage(broadcast)
	} else {
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
}

// Observe sends new agreed consensus to a channel
func Observe(ch chan *pb.Deliver) error {
	if c != nil {
		// If we have a cons-api and an internal stream, we don't want to close
		// it until shut down
		for {
			deliver := <-c.internalStream
			ch <- deliver
		}
	} else {
		var conn *grpc.ClientConn
		var err error
		consenters := getConsenters()
		if len(consenters) == 0 {
			return fmt.Errorf("No consenter node found in configuration.")
		}
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
		for {
			deliver, err := s.Recv()
			if err == io.EOF {
				return nil
			}
			if err != nil {
				return err
			}
			ch <- deliver
		}
	}
}

func getConsenters() []string {
	return viper.GetStringSlice("consensus.consenters")
}
