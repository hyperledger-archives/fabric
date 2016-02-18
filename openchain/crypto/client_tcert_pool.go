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

package crypto

import (
	"errors"
	"github.com/openblockchain/obc-peer/openchain/crypto/utils"
	"time"
)

type tCertPool interface {
	Start() error

	GetNextTCert() (tCert, error)

	AddTCert(tCert tCert) error
}

type tCertPoolImpl struct {
	client *clientImpl

	tCertChannel         chan tCert
	tCertChannelFeedback chan struct{}
}

func (tCertPool *tCertPoolImpl) init(client *clientImpl) (err error) {
	tCertPool.client = client

	tCertPool.tCertChannel = make(chan tCert, client.node.conf.getTCertBathSize()*2)
	tCertPool.tCertChannelFeedback = make(chan struct{}, client.node.conf.getTCertBathSize()*2)

	return
}

func (tCertPool *tCertPoolImpl) Start() (err error) {
	go tCertPool.filler()

	return
}

func (tCertPool *tCertPoolImpl) GetNextTCert() (tCert tCert, err error) {
	for i := 0; i < 3; i++ {
		tCertPool.client.node.log.Debug("Getting next TCert... %d out of 3", i)
		select {
		case tCert = <-tCertPool.tCertChannel:
			break
		case <-time.After(30 * time.Second):
			tCertPool.client.node.log.Error("Failed getting a new TCert. Buffer is empty!")

			//return nil, errors.New("Failed getting a new TCert. Buffer is empty!")
		}
		if tCert != nil {
			// Send feedback to the filler
			tCertPool.tCertChannelFeedback <- struct{}{}
			break
		}
	}

	if tCert == nil {
		// TODO: change error here
		return nil, errors.New("Failed getting a new TCert. Buffer is empty!")
	}

	tCertPool.client.node.log.Debug("Cert [% x].", tCert.GetCertificate().Raw)
	tCertPool.client.node.log.Debug("Getting next TCert...done!")

	return
}

func (tCertPool *tCertPoolImpl) filler() {
	for {
		select {
		case <-tCertPool.tCertChannelFeedback:
		case <-time.After(1 * time.Second):
		}

		if len(tCertPool.tCertChannel) < tCertPool.client.node.conf.getTCertBathSize() {
			tCertPool.client.node.log.Debug("Refill TCert Pool. Current size [%d].",
				len(tCertPool.tCertChannel),
			)
			err := tCertPool.client.getTCertsFromTCA(
				cap(tCertPool.tCertChannel) - len(tCertPool.tCertChannel),
			)
			if err != nil {
				tCertPool.client.node.log.Error("Failed getting TCerts from the TCA: [%s]", err)
			}
		}
	}
}

func (tCertPool *tCertPoolImpl) AddTCert(tCert tCert) (err error) {
	tCertPool.tCertChannel <- tCert

	return
}
