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
	"github.com/openblockchain/obc-peer/openchain/db"
	"github.com/openblockchain/obc-peer/openchain/ledger/statemgmt"
	"github.com/openblockchain/obc-peer/openchain/ledger/util"
)

func fetchDataNodeFromDB(dataKey *dataKey) (*dataNode, error) {
	openchainDB := db.GetDBHandle()
	nodeBytes, err := openchainDB.GetFromStateCF(dataKey.getEncodedBytes())
	if err != nil {
		return nil, err
	}
	if util.IsNil(nodeBytes) {
		return nil, nil
	}
	return unmarshalDataNode(dataKey, nodeBytes), nil
}

func fetchBucketNodeFromDB(bucketKey *bucketKey) (*bucketNode, error) {
	openchainDB := db.GetDBHandle()
	nodeBytes, err := openchainDB.GetFromStateCF(bucketKey.getEncodedBytes())
	if err != nil {
		return nil, err
	}
	if util.IsNil(nodeBytes) {
		return nil, nil
	}
	return unmarshalBucketNode(bucketKey, nodeBytes), nil
}

type rawKey []byte

func fetchDataNodesFromDBFor(bucketKey *bucketKey) (dataNodes, error) {
	logger.Debug("Fetching from DB data nodes for bucket [%s]", bucketKey)
	openchainDB := db.GetDBHandle()
	itr := openchainDB.GetStateCFIterator()
	defer itr.Close()
	minimumDataKeyBytes := minimumPossibleDataKeyBytesFor(bucketKey)

	var dataNodes dataNodes

	itr.Seek(minimumDataKeyBytes)

	for ; itr.Valid(); itr.Next() {
		kSlice := itr.Key()
		defer kSlice.Free()
		vSlice := itr.Value()
		defer vSlice.Free()
		k := kSlice.Data()
		v := vSlice.Data()
		// making a copy of key-value bytes because, underlying key bytes are reused by itr.
		//Did not observe the same behaviour for value but for now making copy for both key and value bytes
		keyBytes := statemgmt.Copy(k)
		valueBytes := statemgmt.Copy(v)

		dataKey := newDataKeyFromEncodedBytes(keyBytes)
		logger.Debug("Retrieved data key [%s] from DB for bucket [%s]", dataKey, bucketKey)
		if !dataKey.getBucketKey().equals(bucketKey) {
			logger.Debug("Data key [%s] from DB does not belong to bucket = [%s]. Stopping further iteration and returning results [%v]", dataKey, bucketKey, dataNodes)
			return dataNodes, nil
		}
		dataNode := unmarshalDataNode(dataKey, valueBytes)

		logger.Debug("Data node [%s] from DB belongs to bucket = [%s]. Including the key in results...", dataNode, bucketKey)
		dataNodes = append(dataNodes, dataNode)
	}
	logger.Debug("Returning results [%v]", dataNodes)
	return dataNodes, nil
}
