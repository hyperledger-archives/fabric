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
	msgChans map[uint64]chan *sendRequest
	closed   sync.WaitGroup
	closedCh chan struct{}
}

type sendRequest struct {
	msg  *pb.Message
	done chan struct{}
}

func newBroadcaster(self uint64, N int, f int, c communicator) *broadcaster {
	queueSize := 10 // XXX increase after testing

	chans := make(map[uint64]chan *sendRequest)
	b := &broadcaster{
		comm:     c,
		f:        f,
		msgChans: chans,
		closedCh: make(chan struct{}),
	}
	for i := 0; i < N; i++ {
		if uint64(i) == self {
			continue
		}
		chans[uint64(i)] = make(chan *sendRequest, queueSize)
		go b.drainer(uint64(i))
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

func (b *broadcaster) drainerSend(dest uint64, send *sendRequest, printedValidatorNotFound bool) bool {
	defer func() {
		send.done <- struct{}{}
		b.closed.Done()
	}()
	h, err := getValidatorHandle(dest)
	if err != nil {
		if !printedValidatorNotFound {
			logger.Warningf("could not get handle for replica %d", dest)
		}
		time.Sleep(time.Second)
		return true
	}

	if printedValidatorNotFound {
		logger.Infof("Found handle for replica %d", dest)
		printedValidatorNotFound = false
	}

	err = b.comm.Unicast(send.msg, h)
	if err != nil {
		logger.Warningf("could not send to replica %d: %v", dest, err)
	}

	return false
}

func (b *broadcaster) drainer(dest uint64) {
	printedValidatorNotFound := false

	for {
		select {
		case <-b.closedCh:
			for {
				// Drain the message channel to free calling waiters before we shut down
				select {
				case send := <-b.msgChans[dest]:
					send.done <- struct{}{}
					b.closed.Done()
				default:
					return
				}
			}
		case send := <-b.msgChans[dest]:
			printedValidatorNotFound = b.drainerSend(dest, send, printedValidatorNotFound)
		}
	}
}

func (b *broadcaster) unicastOne(msg *pb.Message, dest uint64, wait chan struct{}) {
	select {
	case b.msgChans[dest] <- &sendRequest{
		msg:  msg,
		done: wait,
	}:
	default:
		// If this channel is full, we must discard the message and flag it as done
		wait <- struct{}{}
		b.closed.Done()
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

	wait := make(chan struct{}, destCount)

	if dest != nil {
		b.closed.Add(1)
		b.unicastOne(msg, *dest, wait)
	} else {
		b.closed.Add(len(b.msgChans))
		for i := range b.msgChans {
			b.unicastOne(msg, i, wait)
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
