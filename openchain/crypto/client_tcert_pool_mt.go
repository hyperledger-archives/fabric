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
	"time"
)

// The Multi-threaded tCertPool is currently not used.
// It plays only a role in testing.
type tCertPoolMultithreadingImpl struct {
	client *clientImpl

	tCertChannel         chan tCert
	tCertChannelFeedback chan struct{}
	done                 chan struct{}
}

func (tCertPool *tCertPoolMultithreadingImpl) Start() (err error) {
	// Start the filler
	go tCertPool.filler()

	return
}

func (tCertPool *tCertPoolMultithreadingImpl) Stop() (err error) {
	// Stop the filler
	tCertPool.done <- struct{}{}

	// Store unused TCert
	tCertPool.client.node.debug("Store unused TCerts...")

	tCerts := []tCert{}
	for {
		if len(tCertPool.tCertChannel) > 0 {
			tCerts = append(tCerts, <-tCertPool.tCertChannel)
		} else {
			break
		}
	}

	tCertPool.client.node.debug("Found %d unused TCerts...", len(tCerts))

	tCertPool.client.node.ks.storeUnusedTCerts(tCerts)

	tCertPool.client.node.debug("Store unused TCerts...done!")

	return
}

func (tCertPool *tCertPoolMultithreadingImpl) GetNextTCert() (tCert tCert, err error) {
	for i := 0; i < 3; i++ {
		tCertPool.client.node.debug("Getting next TCert... %d out of 3", i)
		select {
		case tCert = <-tCertPool.tCertChannel:
			break
		case <-time.After(30 * time.Second):
			tCertPool.client.node.error("Failed getting a new TCert. Buffer is empty!")

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

	tCertPool.client.node.debug("Cert [% x].", tCert.GetCertificate().Raw)

	// Store the TCert permanently
	tCertPool.client.node.ks.storeUsedTCert(tCert)

	tCertPool.client.node.debug("Getting next TCert...done!")

	return
}

func (tCertPool *tCertPoolMultithreadingImpl) AddTCert(tCert tCert) (err error) {
	tCertPool.client.node.debug("New TCert added.")
	tCertPool.tCertChannel <- tCert

	return
}

func (tCertPool *tCertPoolMultithreadingImpl) init(client *clientImpl) (err error) {
	tCertPool.client = client

	tCertPool.tCertChannel = make(chan tCert, client.node.conf.getTCertBathSize()*2)
	tCertPool.tCertChannelFeedback = make(chan struct{}, client.node.conf.getTCertBathSize()*2)
	tCertPool.done = make(chan struct{})

	return
}

func (tCertPool *tCertPoolMultithreadingImpl) filler() {
	// Load unused TCerts
	stop := false
	full := false
	for {
		// Check if Stop was called
		select {
		case <-tCertPool.done:
			tCertPool.client.node.debug("Force stop!")
			stop = true
		default:
		}
		if stop {
			break
		}

		tCertDER, err := tCertPool.client.node.ks.loadUnusedTCert()
		if err != nil {
			tCertPool.client.node.error("Failed loading TCert: [%s]", err)
			break
		}
		if tCertDER == nil {
			tCertPool.client.node.debug("No more TCerts in cache!")
			break
		}

		tCert, err := tCertPool.client.getTCertFromDER(tCertDER)
		if err != nil {
			tCertPool.client.node.error("Failed paring TCert [% x]: [%s]", tCertDER, err)

			continue
		}

		// Try to send the tCert to the channel if not full
		select {
		case tCertPool.tCertChannel <- tCert:
			tCertPool.client.node.debug("TCert send to the channel!")
		default:
			tCertPool.client.node.debug("Channell Full!")
			full = true
		}
		if full {
			break
		}
	}

	tCertPool.client.node.debug("Load unused TCerts...done!")

	if !stop {
		ticker := time.NewTicker(1 * time.Second)
		for {
			select {
			case <-tCertPool.done:
				stop = true
				tCertPool.client.node.debug("Done signal.")
			case <-tCertPool.tCertChannelFeedback:
				tCertPool.client.node.debug("Feedback received. Time to check for tcerts")
			case <-ticker.C:
				tCertPool.client.node.debug("Time elapsed. Time to check for tcerts")
			}

			if stop {
				tCertPool.client.node.debug("Quitting filler...")
				break
			}

			if len(tCertPool.tCertChannel) < tCertPool.client.node.conf.getTCertBathSize() {
				tCertPool.client.node.debug("Refill TCert Pool. Current size [%d].",
					len(tCertPool.tCertChannel),
				)

				var numTCerts int = cap(tCertPool.tCertChannel) - len(tCertPool.tCertChannel)
				if len(tCertPool.tCertChannel) == 0 {
					numTCerts = cap(tCertPool.tCertChannel) / 10
				}

				tCertPool.client.node.info("Refilling [%d] TCerts.", numTCerts)

				err := tCertPool.client.getTCertsFromTCA(numTCerts)
				if err != nil {
					tCertPool.client.node.error("Failed getting TCerts from the TCA: [%s]", err)
				}
			}
		}
	}

	tCertPool.client.node.debug("TCert filler stopped.")
}
