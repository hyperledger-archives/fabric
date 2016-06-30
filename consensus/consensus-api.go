package consensus

import (
	"fmt"
	"io"
	"github.com/op/go-logging"
//	"github.com/golang/protobuf/proto"
//	"github.com/hyperledger/fabric/core/util"
	pb "github.com/hyperledger/fabric/protos"
//	context "golang.org/x/net/context"
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
}

var c *consensusAPI

// NewConsensusAPIServer creates and returns a new Consensus API server instance.
func NewConsensusAPIServer() *consensusAPI {
	if c == nil {
		c = new(consensusAPI)
		c.consToSend = make(chan *pb.Deliver)
		c.streamList = newStreamList()
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
		err = c.handleMessage(broadcast)
		if err != nil {
			logger.Errorf("Error handling message: %s", err)
			return err
		}
	}
}

// handleMessage handles messages
func (c *consensusAPI) handleMessage(broadcast *pb.Broadcast) error {
	return nil
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
