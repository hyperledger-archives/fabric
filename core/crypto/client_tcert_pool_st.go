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
	"fmt"
	"sync"
	"sort"
	"encoding/hex"
	utils "github.com/hyperledger/fabric/core/crypto/utils"
)


type TCertBlock struct {
	tCert tCert
	attributesHash string
}

type TCertDBBlock struct {
	 tCertDER []byte
	attributesHash string
	preK0 []byte
}


type tCertPoolSingleThreadImpl struct {
	client *clientImpl
	
	empty bool

	length    map[string]int
	
	tCerts map[string][]*TCertBlock
	
	m      sync.Mutex
}

func (tCertPool *tCertPoolSingleThreadImpl) Start() (err error) {
	tCertPool.m.Lock()
	defer tCertPool.m.Unlock()

	tCertPool.client.debug("Starting TCert Pool...")

    
	// Load unused TCerts if any
	tCertDBBlocks, err := tCertPool.client.ks.loadUnusedTCerts()
	if err != nil {
		tCertPool.client.error("Failed loading TCerts from cache: [%s]", err)

		return
	}


	if len(tCertDBBlocks) > 0 {
		
		tCertPool.client.debug("TCerts in cache found! Loading them...")

		for _, tCertDBBlock := range tCertDBBlocks {
			tCertBlock, err := tCertPool.client.getTCertFromDER(tCertDBBlock)
			if err != nil {
				tCertPool.client.error("Failed paring TCert [% x]: [%s]", tCertDBBlock.tCertDER, err)

				continue
			}
			tCertPool.AddTCert(tCertBlock)
		}
	}  //END-IF

	return
}

func (tCertPool *tCertPoolSingleThreadImpl) Stop() (err error) {
	tCertPool.m.Lock()
	defer tCertPool.m.Unlock()

	//tCertPool.client.debug("Found %d unused TCerts...", tCertPool.len)


	for k := range tCertPool.tCerts {
		certList := tCertPool.tCerts[k]
		certListLen := tCertPool.length[k]
		tCertPool.client.ks.storeUnusedTCerts(certList[:certListLen])
    }


	tCertPool.client.debug("Store unused TCerts...done!")

	return
}



func (tCertPool *tCertPoolSingleThreadImpl) CalculateAttributesHash(attributes map[string]string) (attrHash string) {
	
	keys := make([]string, len(attributes))
	 
	 for k := range attributes {
        keys = append(keys, k)
    }
	 
	  sort.Strings(keys)
	 
	  
	  values := make([]byte, len(keys))
	  
	 for _,k := range keys {
	 	
	 	vb := []byte(k)
	 	for _,bval := range vb {
	 		 values = append(values, bval)
	 	}
	 	
	 	vb = []byte(attributes[k])
	 	for _,bval := range vb {
	 		 values = append(values, bval)
	 	}
    }
	
	
	attributesHash := utils.Hash(values)
	
	return  hex.EncodeToString(attributesHash)
	
	
}


func (tCertPool *tCertPoolSingleThreadImpl) GetNextTCert(attributes map[string]string) (tCert *TCertBlock, err error) {
	
	tCertPool.m.Lock()
	defer tCertPool.m.Unlock()

	attributesHash := tCertPool.CalculateAttributesHash(attributes)
	
	poolLen := tCertPool.length[attributesHash]
	
	if  poolLen <= 0 {
		// Reload
		if err := tCertPool.client.getTCertsFromTCA(attributesHash, attributes, tCertPool.client.conf.getTCertBatchSize()); err != nil {

			return nil, fmt.Errorf("Failed loading TCerts from TCA")
		}
	}


   tCert = tCertPool.tCerts[attributesHash][tCertPool.length[attributesHash]  -1]
   

	tCertPool.length[attributesHash] = tCertPool.length[attributesHash]  -1

	return
}



func (tCertPool *tCertPoolSingleThreadImpl) AddTCert(tCertBlock *TCertBlock) (err error) {
	
	
	tCertPool.client.debug("Adding new Cert [% x].", tCertBlock.tCert.GetCertificate().Raw)
	
	if tCertPool.length[tCertBlock.attributesHash] <= 0 {
		tCertPool.length[tCertBlock.attributesHash]  = 0
	}

	tCertPool.length[tCertBlock.attributesHash] = tCertPool.length[tCertBlock.attributesHash] + 1
	
	if tCertPool.tCerts[tCertBlock.attributesHash] == nil {
		
		tCertPool.tCerts[tCertBlock.attributesHash]= make([]*TCertBlock, tCertPool.client.conf.getTCertBatchSize())
	
	} 
	
	tCertPool.tCerts[tCertBlock.attributesHash][tCertPool.length[tCertBlock.attributesHash]  -1] =  tCertBlock


	return nil
}

func (tCertPool *tCertPoolSingleThreadImpl) init(client *clientImpl) (err error) {
	
	tCertPool.client = client

	tCertPool.client.debug("Init TCert Pool...")

   tCertPool.tCerts = make(map[string][]*TCertBlock)

	tCertPool.length = make(map[string]int)
	
	return
}
