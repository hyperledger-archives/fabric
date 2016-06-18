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
	"sync"
	"time"

	"github.com/hyperledger/fabric/consensus"
	pb "github.com/hyperledger/fabric/protos"
)

type communicator interface {
	consensus.Communicator
	consensus.Inquirer
}

type broadcaster struct {
	comm communicator

	f        int
	msgChans map[uint64]chan *pb.Message
	closed   sync.WaitGroup
	closedCh chan struct{}
}

func newBroadcaster(self uint64, N int, f int, c communicator) *broadcaster {
	queueSize := 10 // XXX increase after testing

	chans := make(map[uint64]chan *pb.Message)
	for i := 0; i < N; i++ {
		if uint64(i) == self {
			continue
		}
		chans[uint64(i)] = make(chan *pb.Message, queueSize)
	}
	b := &broadcaster{
		comm:     c,
		f:        f,
		msgChans: chans,
		closedCh: make(chan struct{}),
	}
	return b
}

func (b *broadcaster) Close() {
	close(b.closedCh)
	b.closed.Wait()
}

func (b *broadcaster) Wait() {
	b.closed.Wait()
}

func (b *broadcaster) unicastOne(msg *pb.Message, dest uint64, wait chan bool) {
	defer func() {
		b.closed.Done()
		wait <- true
	}()

	select {
	case <-b.closedCh:
		return
	default:
	}

	h, err := getValidatorHandle(dest)
	if err != nil {
		logger.Warningf("could not get handle for replica %d", dest)
	} else {
		err = b.comm.Unicast(msg, h)
	}
	if err != nil {
		logger.Debugf("could not send to replica %d: %v", dest, err)
		select {
		case <-b.closedCh:
			return
		case <-time.After(1 * time.Second):
			return
		}
	}
}

func (b *broadcaster) send(msg *pb.Message, dest *uint64) error {
	select {
	case <-b.closedCh:
		return fmt.Errorf("broadcaster closed")
	default:
	}

	var destCount int
	var required int
	if dest != nil {
		destCount = 1
		required = 1
	} else {
		destCount = len(b.msgChans)
		required = destCount - b.f
	}

	wait := make(chan bool, destCount)

	if dest != nil {
		b.closed.Add(1)
		go b.unicastOne(msg, *dest, wait)
	} else {
		b.closed.Add(len(b.msgChans))
		for i := range b.msgChans {
			go b.unicastOne(msg, i, wait)
		}
	}

	for i := 0; i < required; i++ {
		<-wait
	}

	return nil
}

func (b *broadcaster) Unicast(msg *pb.Message, dest uint64) error {
	return b.send(msg, &dest)
}

func (b *broadcaster) Broadcast(msg *pb.Message) error {
	return b.send(msg, nil)
}
