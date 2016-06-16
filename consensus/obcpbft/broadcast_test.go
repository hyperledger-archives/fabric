/*
Copyright IBM Corp. 2016 All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

		 http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package obcpbft

import (
	"fmt"
	"testing"
	"time"

	pb "github.com/hyperledger/fabric/protos"
)

type mockMsg struct {
	msg  *pb.Message
	dest *pb.PeerID
}

type mockComm struct {
	self  uint64
	n     uint64
	msgCh chan mockMsg
}

func (m *mockComm) Unicast(msg *pb.Message, dest *pb.PeerID) error {
	m.msgCh <- mockMsg{msg, dest}
	return nil
}

func (m *mockComm) Broadcast(msg *pb.Message, t pb.PeerEndpoint_Type) error {
	return nil
}

func (m *mockComm) GetNetworkInfo() (*pb.PeerEndpoint, []*pb.PeerEndpoint, error) {
	return nil, nil, nil
}

func (m *mockComm) GetNetworkHandles() (*pb.PeerID, []*pb.PeerID, error) {
	var h []*pb.PeerID
	for n := uint64(0); n < m.n; n++ {
		h = append(h, &pb.PeerID{Name: fmt.Sprintf("vp%d", n)})
	}
	return h[m.self], h, nil
}

func TestBroadcast(t *testing.T) {
	m := &mockComm{
		self:  1,
		n:     4,
		msgCh: make(chan mockMsg, 4),
	}
	sent := make(map[string]int)
	go func() {
		for msg := range m.msgCh {
			sent[msg.dest.Name] += 1
		}
	}()

	b := newBroadcaster(1, 4, 1, m)

	msg := &pb.Message{Payload: []byte("hi")}
	b.Broadcast(msg)
	time.Sleep(100 * time.Millisecond)
	b.Close()

	sentCount := 0
	for _, q := range sent {
		if q == 1 {
			sentCount += 1
		}
	}

	if sentCount < 2 {
		t.Errorf("broadcast did not send to all peers: %v", sent)
	}
}

type mockStuckComm struct {
	mockComm
	done chan struct{}
}

func (m *mockStuckComm) Unicast(msg *pb.Message, dest *pb.PeerID) error {
	ret := m.mockComm.Unicast(msg, dest)
	if dest.Name == "vp0" {
		select {
		case <-time.After(2 * time.Second):
			return fmt.Errorf("timeout")
		case <-m.done:
			return fmt.Errorf("closed")
		}
	}
	return ret
}

func TestBroadcastStuck(t *testing.T) {
	m := &mockStuckComm{
		mockComm: mockComm{
			self:  1,
			n:     4,
			msgCh: make(chan mockMsg),
		},
		done: make(chan struct{}),
	}
	sent := make(map[string][]string)
	go func() {
		for msg := range m.msgCh {
			key := string(msg.msg.Payload)
			sent[key] = append(sent[key], msg.dest.Name)
		}
	}()

	b := newBroadcaster(1, 4, 1, m)

	maxc := 20
	for c := 0; c < maxc; c++ {
		b.Broadcast(&pb.Message{Payload: []byte(fmt.Sprintf("%d", c))})
	}

	done := make(chan struct{})
	go func() {
		select {
		case <-done:
			return
		case <-time.After(time.Second):
			t.Fatal("blocked")
		}
	}()
	time.Sleep(100 * time.Millisecond)
	close(m.done)
	b.Close()
	close(done)

	sendDone := 0
	for _, q := range sent {
		if len(q) >= 2 {
			sendDone += 1
		}
	}
	if sendDone != maxc {
		t.Errorf("expected %d sent messages: %v", maxc, sent)
	}
}

func TestBroadcastUnicast(t *testing.T) {
	m := &mockComm{
		self:  1,
		n:     4,
		msgCh: make(chan mockMsg, 4),
	}
	sent := make(map[string]int)
	go func() {
		for msg := range m.msgCh {
			sent[msg.dest.Name] += 1
		}
	}()

	b := newBroadcaster(1, 4, 1, m)

	msg := &pb.Message{Payload: []byte("hi")}
	b.Unicast(msg, 0)
	time.Sleep(100 * time.Millisecond)
	b.Close()

	sentCount := 0
	for _, q := range sent {
		if q == 1 {
			sentCount += 1
		}
	}

	if sentCount != 1 {
		t.Errorf("broadcast did not send to dest peer: %v", sent)
	}
}
