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

package crypto

import (
	"errors"
	"runtime"
	"strings"
	"sync"
	"time"
)

type tCertPoolEntry struct {
	attributes           []string
	tCertChannel         chan *TCertBlock
	tCertChannelFeedback chan struct{}
	done                 chan struct{}
	client               *clientImpl
	tCertBlock           *TCertBlock
}

//NewTCertPoolEntry creates a new tcert pool entry
func newTCertPoolEntry(client *clientImpl, attributes []string) *tCertPoolEntry {
	tCertChannel := make(chan *TCertBlock, client.conf.getTCertBatchSize()*2)
	tCertChannelFeedback := make(chan struct{}, client.conf.getTCertBatchSize()*2)
	done := make(chan struct{}, 1)
	return &tCertPoolEntry{attributes, tCertChannel, tCertChannelFeedback, done, client, nil}
}

//Start starts the pool entry filler loop.
func (tCertPoolEntry *tCertPoolEntry) Start() (err error) {
	// Start the filler
	go tCertPoolEntry.filler()
	return
}

//Stop stops the pool entry filler loop.
func (tCertPoolEntry *tCertPoolEntry) Stop() (err error) {
	// Stop the filler
	tCertPoolEntry.done <- struct{}{}

	// Store unused TCert
	tCertPoolEntry.client.debug("Store unused TCerts...")

	tCerts := make([]*TCertBlock, 0)
	for {
		if len(tCertPoolEntry.tCertChannel) > 0 {
			tCerts = append(tCerts, <-tCertPoolEntry.tCertChannel)
		} else {
			break
		}
	}

	tCertPoolEntry.client.debug("Found %d unused TCerts...", len(tCerts))

	tCertPoolEntry.client.ks.storeUnusedTCerts(tCerts)

	tCertPoolEntry.client.debug("Store unused TCerts...done!")

	return
}

//AddTCert add a tcert to the poolEntry.
func (tCertPoolEntry *tCertPoolEntry) AddTCert(tCertBlock *TCertBlock) (err error) {
	tCertPoolEntry.tCertChannel <- tCertBlock
	return
}

//GetNextTCert gets the next tcert of the pool.
func (tCertPoolEntry *tCertPoolEntry) GetNextTCert(attributes ...string) (tCertBlock *TCertBlock, err error) {
	for i := 0; i < 3; i++ {
		tCertPoolEntry.client.debug("Getting next TCert... %d out of 3", i)
		select {
		case tCertPoolEntry.tCertBlock = <-tCertPoolEntry.tCertChannel:
			break
		case <-time.After(30 * time.Second):
			tCertPoolEntry.client.error("Failed getting a new TCert. Buffer is empty!")
		}
		if tCertPoolEntry.tCertBlock != nil {
			// Send feedback to the filler
			tCertPoolEntry.client.debug("Send feedback")
			tCertPoolEntry.tCertChannelFeedback <- struct{}{}
			break
		}
	}

	if tCertPoolEntry.tCertBlock == nil {
		// TODO: change error here
		return nil, errors.New("Failed getting a new TCert. Buffer is empty!")
	}

	tCertBlock = tCertPoolEntry.tCertBlock
	tCertPoolEntry.client.debug("Cert [% x].", tCertBlock.tCert.GetCertificate().Raw)

	// Store the TCert permanently
	tCertPoolEntry.client.ks.storeUsedTCert(tCertBlock)

	tCertPoolEntry.client.debug("Getting next TCert...done!")

	return
}

func (tCertPoolEntry *tCertPoolEntry) filler() {
	// Load unused TCerts
	stop := false
	full := false
	tCertPoolEntry.client.debug("Filler()")

	attributeHash := calculateAttributesHash(tCertPoolEntry.attributes)
	for {
		// Check if Stop was called
		select {
		case <-tCertPoolEntry.done:
			tCertPoolEntry.client.debug("Force stop!")
			stop = true
		default:
		}
		if stop {
			break
		}

		tCertDBBlocks, err := tCertPoolEntry.client.ks.loadUnusedTCerts()

		if err != nil {
			tCertPoolEntry.client.error("Failed loading TCert: [%s]", err)
			break
		}
		if tCertDBBlocks == nil {
			tCertPoolEntry.client.debug("No more TCerts in cache!")
			break
		}

		var tCert *TCertBlock
		for _, tCertDBBlock := range tCertDBBlocks {
			if strings.Compare(attributeHash, tCertDBBlock.attributesHash) == 0 {
				tCertBlock, err := tCertPoolEntry.client.getTCertFromDER(tCertDBBlock)
				if err != nil {
					tCertPoolEntry.client.error("Failed paring TCert [% x]: [%s]", tCertDBBlock.tCertDER, err)
					continue
				}
				tCert = tCertBlock
			}
		}

		if tCert != nil {
			// Try to send the tCert to the channel if not full
			select {
			case tCertPoolEntry.tCertChannel <- tCert:
				tCertPoolEntry.client.debug("TCert send to the channel!")
			default:
				tCertPoolEntry.client.debug("Channell Full!")
				full = true
			}
			if full {
				break
			}
		} else {
			tCertPoolEntry.client.debug("No more TCerts in cache!")
			break
		}
	}

	tCertPoolEntry.client.debug("Load unused TCerts...done!")

	if !stop {
		ticker := time.NewTicker(1 * time.Second)
		for {
			select {
			case <-tCertPoolEntry.done:
				stop = true
				tCertPoolEntry.client.debug("Done signal.")
			case <-tCertPoolEntry.tCertChannelFeedback:
				tCertPoolEntry.client.debug("Feedback received. Time to check for tcerts")
			case <-ticker.C:
				tCertPoolEntry.client.debug("Time elapsed. Time to check for tcerts")
			}

			if stop {
				tCertPoolEntry.client.debug("Quitting filler...")
				break
			}

			if len(tCertPoolEntry.tCertChannel) < tCertPoolEntry.client.conf.getTCertBatchSize() {
				tCertPoolEntry.client.debug("Refill TCert Pool. Current size [%d].",
					len(tCertPoolEntry.tCertChannel),
				)

				var numTCerts = cap(tCertPoolEntry.tCertChannel) - len(tCertPoolEntry.tCertChannel)
				if len(tCertPoolEntry.tCertChannel) == 0 {
					numTCerts = cap(tCertPoolEntry.tCertChannel) / 10
					if numTCerts < 1 {
						numTCerts = 1
					}
				}

				tCertPoolEntry.client.info("Refilling [%d] TCerts.", numTCerts)

				err := tCertPoolEntry.client.getTCertsFromTCA(calculateAttributesHash(tCertPoolEntry.attributes), tCertPoolEntry.attributes, numTCerts)
				if err != nil {
					tCertPoolEntry.client.error("Failed getting TCerts from the TCA: [%s]", err)
				}
			}
		}
	}

	tCertPoolEntry.client.debug("TCert filler stopped.")
}

// The Multi-threaded tCertPool is currently not used.
// It plays only a role in testing.
type tCertPoolMultithreadingImpl struct {
	client       *clientImpl
	poolEntries  map[string]*tCertPoolEntry
	entriesMutex *sync.Mutex
}

//Start starts the pool processing.
func (tCertPool *tCertPoolMultithreadingImpl) Start() (err error) {
	// Start the filler, initializes a poolEntry without attributes.
	var attributes []string
	poolEntry, err := tCertPool.getPoolEntry(attributes)
	if err != nil {
		return err
	}
	return poolEntry.Start()
}

func (tCertPool *tCertPoolMultithreadingImpl) lockEntries() {
	tCertPool.entriesMutex.Lock()
}

func (tCertPool *tCertPoolMultithreadingImpl) releaseEntries() {
	tCertPool.entriesMutex.Unlock()
	runtime.Gosched()
}

//Stop stops the pool.
func (tCertPool *tCertPoolMultithreadingImpl) Stop() (err error) {
	// Stop the filler
	tCertPool.lockEntries()
	defer tCertPool.releaseEntries()
	for _, entry := range tCertPool.poolEntries {
		err := entry.Stop()
		if err != nil {
			return err
		}
	}
	return
}

//Returns a tCertPoolEntry for the attributes "attributes", if the tCertPoolEntry doesn't exists a new tCertPoolEntry will be create for that attributes.
func (tCertPool *tCertPoolMultithreadingImpl) getPoolEntryFromHash(attributeHash string) *tCertPoolEntry {
	tCertPool.lockEntries()
	defer tCertPool.releaseEntries()
	poolEntry := tCertPool.poolEntries[attributeHash]
	return poolEntry

}

//Returns a tCertPoolEntry for the attributes "attributes", if the tCertPoolEntry doesn't exists a new tCertPoolEntry will be create for that attributes.
func (tCertPool *tCertPoolMultithreadingImpl) getPoolEntry(attributes []string) (*tCertPoolEntry, error) {
	tCertPool.client.debug("Getting pool entry %v \n", attributes)
	attributeHash := calculateAttributesHash(attributes)
	tCertPool.lockEntries()
	defer tCertPool.releaseEntries()
	poolEntry := tCertPool.poolEntries[attributeHash]
	if poolEntry == nil {
		tCertPool.client.debug("New pool entry %v \n", attributes)

		poolEntry = newTCertPoolEntry(tCertPool.client, attributes)
		tCertPool.poolEntries[attributeHash] = poolEntry
		if err := poolEntry.Start(); err != nil {
			return nil, err
		}
		tCertPool.client.debug("Pool entry started %v \n", attributes)

	}
	return poolEntry, nil
}

//GetNextTCert returns a TCert from the pool valid to the passed attributes. If no TCert is available TCA is invoked to generate it.
func (tCertPool *tCertPoolMultithreadingImpl) GetNextTCerts(nCerts int, attributes ...string) ([]*TCertBlock, error) {
	blocks := make([]*TCertBlock, nCerts)
	for i := 0; i < nCerts; i++ {
		block, err := tCertPool.getNextTCert(attributes...)
		if err != nil {
			return nil, err
		}
		blocks[i] = block
	}
	return blocks, nil
}

func (tCertPool *tCertPoolMultithreadingImpl) getNextTCert(attributes ...string) (tCertBlock *TCertBlock, err error) {
	poolEntry, err := tCertPool.getPoolEntry(attributes)
	if err != nil {
		return nil, err
	}
	tCertPool.client.debug("Requesting tcert to the pool entry. %v", calculateAttributesHash(attributes))
	return poolEntry.GetNextTCert(attributes...)
}

//AddTCert adds a TCert into the pool is invoked by the client after TCA is called.
func (tCertPool *tCertPoolMultithreadingImpl) AddTCert(tCertBlock *TCertBlock) (err error) {
	poolEntry := tCertPool.getPoolEntryFromHash(tCertBlock.attributesHash)
	if poolEntry == nil {
		return errors.New("No pool entry found for that attributes.")
	}
	tCertPool.client.debug("Adding %v \n.", tCertBlock.attributesHash)
	poolEntry.AddTCert(tCertBlock)

	return
}

func (tCertPool *tCertPoolMultithreadingImpl) init(client *clientImpl) (err error) {
	tCertPool.client = client
	tCertPool.poolEntries = make(map[string]*tCertPoolEntry)
	tCertPool.entriesMutex = &sync.Mutex{}
	return
}
