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

package peer

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/spf13/viper"

	"github.com/hyperledger/fabric/core/config"
	"github.com/hyperledger/fabric/core/container"
	pb "github.com/hyperledger/fabric/protos"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

var peerClientConn *grpc.ClientConn

func TestMain(m *testing.M) {
	config.SetupTestConfig("./../../peer")
	viper.Set("ledger.blockchain.deploy-system-chaincode", "false")
	viper.Set("peer.validator.validity-period.verification", "false")

	tmpConn, err := NewPeerClientConnection()
	if err != nil {
		fmt.Printf("error connection to server at host:port = %s\n", viper.GetString("peer.address"))
		os.Exit(1)
	}
	peerClientConn = tmpConn
	os.Exit(m.Run())
}

func performChat(t testing.TB, conn *grpc.ClientConn) error {
	serverClient := pb.NewPeerClient(conn)
	stream, err := serverClient.Chat(context.Background())
	if err != nil {
		t.Logf("%v.performChat(_) = _, %v", serverClient, err)
		return err
	}
	defer stream.CloseSend()
	t.Log("Starting performChat")

	waitc := make(chan struct{})
	go func() {
		// Be sure to close the channel
		defer close(waitc)
		for {
			in, err := stream.Recv()
			if err == io.EOF {
				t.Logf("Received EOR, exiting chat")
				return
			}
			if err != nil {
				t.Errorf("stream closed with unexpected error: %s", err)
				return
			}
			if in.Type == pb.Message_DISC_HELLO {
				t.Logf("Received message: %s, sending %s", in.Type, pb.Message_DISC_GET_PEERS)
				stream.Send(&pb.Message{Type: pb.Message_DISC_GET_PEERS})
			} else if in.Type == pb.Message_DISC_PEERS {
				//stream.Send(&pb.DiscoveryMessage{Type: pb.DiscoveryMessage_PEERS})
				t.Logf("Received message: %s", in.Type)
				t.Logf("Closing stream and channel")
				return
			} else {
				t.Logf("Received message: %s", in.Type)

			}

		}
	}()
	select {
	case <-waitc:
		return nil
	case <-time.After(1 * time.Second):
		t.Fail()
		return fmt.Errorf("Timeout expired while performChat")
	}
}

func sendLargeMsg(t testing.TB) (*pb.Message, error) {
	vm, err := container.NewVM()
	if err != nil {
		t.Fail()
		t.Logf("Error getting VM: %s", err)
		return nil, err
	}

	inputbuf, err := vm.GetPeerPackageBytes()
	if err != nil {
		t.Fail()
		t.Logf("Error Getting Peer package bytes: %s", err)
		return nil, err
	}
	payload, err := ioutil.ReadAll(inputbuf)
	return &pb.Message{Type: pb.Message_DISC_NEWMSG, Payload: payload}, nil

}

func Benchmark_Chat(b *testing.B) {
	for i := 0; i < b.N; i++ {
		performChat(b, peerClientConn)
	}
}

func Benchmark_Chat_Parallel(b *testing.B) {
	b.SetParallelism(10)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			performChat(b, peerClientConn)
		}
	})
}

func TestServer_Chat(t *testing.T) {
	t.Skip()
	performChat(t, peerClientConn)
}
