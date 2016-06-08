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
	"fmt"
	"sync"
)

type tCertPoolSingleThreadImpl struct {
	client *clientImpl

	tCerts []tCert
	m      sync.Mutex
}

func (tCertPool *tCertPoolSingleThreadImpl) Start() (err error) {
	tCertPool.m.Lock()
	defer tCertPool.m.Unlock()

	tCertPool.client.debug("Starting TCert Pool...")

	// Load unused TCerts if any
	tCertDERs, err := tCertPool.client.ks.loadUnusedTCerts()
	if err != nil {
		tCertPool.client.error("Failed loading TCerts from cache: [%s]", err)

		return
	}
	if len(tCertDERs) == 0 {
		tCertPool.client.debug("No more TCerts in cache! Load new from TCA.")

		tCertPool.client.getTCertsFromTCA(tCertPool.client.conf.getTCertBatchSize())
	} else {
		tCertPool.client.debug("TCerts in cache found! Loading them...")

		for _, tCertDER := range tCertDERs {
			tCert, err := tCertPool.client.getTCertFromDER(tCertDER)
			if err != nil {
				tCertPool.client.error("Failed paring TCert [% x]: [%s]", tCertDER, err)

				continue
			}
			tCertPool.AddTCert(tCert)
		}
	}

	return
}

func (tCertPool *tCertPoolSingleThreadImpl) Stop() (err error) {
	tCertPool.m.Lock()
	defer tCertPool.m.Unlock()

	tCertPool.client.debug("Found %d unused TCerts...", len(tCertPool.tCerts))

	tCertPool.client.ks.storeUnusedTCerts(tCertPool.tCerts)

	tCertPool.client.debug("Store unused TCerts...done!")

	return
}

func (tCertPool *tCertPoolSingleThreadImpl) GetNextTCerts(nCerts int) (tCerts []tCert, err error) {

	if nCerts < 1 {
		return nil, errors.New("Number of requested TCerts has to be positive!")
	}

	tCertPool.m.Lock()
	defer tCertPool.m.Unlock()

	if len(tCertPool.tCerts) < nCerts {
		// We don't have enough TCerts in the pool - reload from the TCA
		nTCerts2Generate := tCertPool.client.conf.getTCertBatchSize() + nCerts - len(tCertPool.tCerts)
		tCertPool.client.debug("Called tCertPool.client.getTCertsFromTCA(%d)", nTCerts2Generate)
		if err := tCertPool.client.getTCertsFromTCA(nTCerts2Generate); err != nil {
			return nil, fmt.Errorf("Failed loading TCerts from TCA")
		}
	}

	// We should have enough TCerts in the pool by now - but just to be sure that we don't panic (out of range)
	if len(tCertPool.tCerts) < nCerts {
		return nil, fmt.Errorf("Failed to obtain %d in the TCertPool", nCerts)
	}

	tCerts = make([]tCert, nCerts)
	copy(tCerts, tCertPool.tCerts[:nCerts])
	tCertPool.tCerts = tCertPool.tCerts[nCerts:]

	return tCerts, nil
}

func (tCertPool *tCertPoolSingleThreadImpl) AddTCert(tCert tCert) (err error) {
	tCertPool.client.debug("Adding new Cert [% x].", tCert.GetCertificate().Raw)

	tCertPool.tCerts = append(tCertPool.tCerts, tCert)
	return nil
}

func (tCertPool *tCertPoolSingleThreadImpl) init(client *clientImpl) (err error) {
	tCertPool.client = client
	tCertPool.client.debug("Init TCert Pool...")

	return
}
