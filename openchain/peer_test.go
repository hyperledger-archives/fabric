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

package openchain

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"testing"

	"github.com/spf13/viper"

	pb "github.com/openblockchain/obc-peer/protos"
	"github.com/openblockchain/obc-peer/openchain/container"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/grpclog"
)

var peerClientConn *grpc.ClientConn

func TestMain(m *testing.M) {
	SetupTestConfig()

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
	testAcceptPeerChatStream(stream)
	if err != nil {
		t.Logf("%v.performChat(_) = _, %v", serverClient, err)
		return err
	}
	defer stream.CloseSend()
	stream.Send(&pb.OpenchainMessage{Type: pb.OpenchainMessage_DISC_HELLO})
	waitc := make(chan struct{})
	go func() {
		for {
			in, err := stream.Recv()
			if err == io.EOF {
				// read done.
				close(waitc)
				return
			}
			if err != nil {
				grpclog.Fatalf("Failed to receive a DiscoverMessage from server : %v", err)
			}
			if in.Type == pb.OpenchainMessage_DISC_HELLO {
				t.Logf("Received message: %s", in.Type)
				stream.Send(&pb.OpenchainMessage{Type: pb.OpenchainMessage_DISC_GET_PEERS})
			} else if in.Type == pb.OpenchainMessage_DISC_PEERS {
				//stream.Send(&pb.DiscoveryMessage{Type: pb.DiscoveryMessage_PEERS})
				t.Logf("Received message: %s", in.Type)
				t.Logf("Closing stream and channel")
				//stream.CloseSend()
				close(waitc)
				return
			}

		}
	}()
	<-waitc
	return nil
}

func sendLargeMsg(t testing.TB) (*pb.OpenchainMessage, error) {
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
	return &pb.OpenchainMessage{Type: pb.OpenchainMessage_DISC_NEWMSG, Payload: payload}, nil

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
	performChat(t, peerClientConn)
}

func TestPeer_FSM(t *testing.T) {
	t.Skip("Not running at moment")
	peerConn := NewPeerFSM("10.10.10.10:30303", nil)

	err := peerConn.FSM.Event(pb.OpenchainMessage_DISC_HELLO.String())
	if err != nil {
		t.Error(err)
	}
	if peerConn.FSM.Current() != "established" {
		t.Error("Expected to be in establised state")
	}

	err = peerConn.FSM.Event("DISCONNECT")
	if err != nil {
		t.Error(err)
	}
}
