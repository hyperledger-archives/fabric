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
	"bytes"
	"fmt"

	"github.com/golang/protobuf/proto"
	"github.com/openblockchain/obc-peer/openchain/ledger/statemgmt"
)

type dataKey struct {
	bucketKey    *bucketKey
	compositeKey []byte
}

// dataKey methods
func newDataKey(chaincodeID string, key string) *dataKey {
	logger.Debug("Enter - newDataKey. chaincodeID=[%s], key=[%s]", chaincodeID, key)
	compositeKey := statemgmt.ConstructCompositeKey(chaincodeID, key)
	bucketHash := conf.computeBucketHash(compositeKey)
	// Adding one because - we start number buckets 1 onwards
	bucketNumber := int(bucketHash)%conf.getNumBucketsAtLowestLevel() + 1
	dataKey := &dataKey{newBucketKeyAtLowestLevel(bucketNumber), compositeKey}
	logger.Debug("Exit - newDataKey=[%s]", dataKey)
	return dataKey
}

func minimumPossibleDataKeyBytesFor(bucketKey *bucketKey) []byte {
	min := proto.EncodeVarint(uint64(bucketKey.bucketNumber))
	min = append(min, byte(0))
	return min
}

func newDataKeyFromEncodedBytes(encodedBytes []byte) *dataKey {
	bucketNum, l := proto.DecodeVarint(encodedBytes)
	if !bytes.Equal(encodedBytes[l:l+1], []byte{byte(0)}) {
		panic(fmt.Errorf("[%#v] is not a valid data key", encodedBytes))
	}
	compositeKey := encodedBytes[l+1:]
	return &dataKey{newBucketKeyAtLowestLevel(int(bucketNum)), compositeKey}
}

func (dataKey *dataKey) getBucketKey() *bucketKey {
	return dataKey.bucketKey
}

func (dataKey *dataKey) getEncodedBytes() []byte {
	encodedBytes := proto.EncodeVarint(uint64(dataKey.bucketKey.bucketNumber))
	encodedBytes = append(encodedBytes, byte(0))
	encodedBytes = append(encodedBytes, dataKey.compositeKey...)
	return encodedBytes
}

func (dataKey *dataKey) String() string {
	return fmt.Sprintf("bucketKey=[%s], compositeKey=[%s]", dataKey.bucketKey, string(dataKey.compositeKey))
}

func (dataKey1 *dataKey) clone() *dataKey {
	clone := &dataKey{dataKey1.bucketKey.clone(), dataKey1.compositeKey}
	return clone
}
