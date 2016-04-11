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

package buckettree

import (
	"github.com/hyperledger/fabric/core/db"
	"github.com/hyperledger/fabric/core/ledger/perfstat"
	"sync"
	"time"
)

var enableBucketCache = false

type bucketCache struct {
	c    map[bucketKey]*bucketNode
	lock sync.RWMutex
}

func newBucketCache() *bucketCache {
	return &bucketCache{c: make(map[bucketKey]*bucketNode)}
}

func (cache *bucketCache) loadAllBucketNodesFromDB() {
	if !enableBucketCache {
		return
	}
	openchainDB := db.GetDBHandle()
	itr := openchainDB.GetStateCFIterator()
	defer itr.Close()
	itr.Seek([]byte{byte(0)})
	count := 0
	cache.lock.Lock()
	defer cache.lock.Unlock()
	for ; itr.Valid(); itr.Next() {
		key := itr.Key().Data()
		if key[0] != byte(0) {
			itr.Key().Free()
			itr.Value().Free()
			break
		}
		bucketKey := decodeBucketKey(itr.Key().Data())
		nodeBytes := itr.Value().Data()
		bucketNode := unmarshalBucketNode(&bucketKey, nodeBytes)
		cache.putWithoutLock(bucketKey, bucketNode)
		itr.Key().Free()
		itr.Value().Free()
		count++
	}
	logger.Info("Loaded buckets data in cache. Total buckets in DB = [%d]", count)
}

func (cache *bucketCache) putWithoutLock(key bucketKey, node *bucketNode) {
	if !enableBucketCache {
		return
	}
	cache.c[key] = node
}

func (cache *bucketCache) get(key bucketKey) (*bucketNode, error) {
	defer perfstat.UpdateTimeStat("timeSpent", time.Now())
	if !enableBucketCache {
		return fetchBucketNodeFromDB(&key)
	}
	cache.lock.RLock()
	defer cache.lock.RUnlock()
	bucketNode := cache.c[key]
	if bucketNode == nil {
		return fetchBucketNodeFromDB(&key)
	}
	return bucketNode, nil
}

func (cache *bucketCache) removeWithoutLock(key bucketKey) {
	if !enableBucketCache {
		return
	}
	delete(cache.c, key)
}
