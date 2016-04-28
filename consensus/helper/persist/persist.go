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

package persist

import (
	"github.com/hyperledger/fabric/core/db"
)

type PersistHelper struct{}

func (h *PersistHelper) StoreState(key string, value []byte) error {
	db := db.GetDBHandle()
	return db.Put(db.PersistCF, []byte("consensus."+key), value)
}

func (h *PersistHelper) DelState(key string) {
	db := db.GetDBHandle()
	db.Delete(db.PersistCF, []byte("consensus."+key))
}

func (h *PersistHelper) ReadState(key string) ([]byte, error) {
	db := db.GetDBHandle()
	return db.Get(db.PersistCF, []byte("consensus."+key))
}

func (h *PersistHelper) ReadStateSet(prefix string) (map[string][]byte, error) {
	db := db.GetDBHandle()
	prefixRaw := []byte("consensus." + prefix)

	ret := make(map[string][]byte)
	it := db.GetIterator(db.PersistCF)
	defer it.Close()
	for it.Seek(prefixRaw); it.ValidForPrefix(prefixRaw); it.Next() {
		key := string(it.Key().Data())
		key = key[len("consensus."):len(key)]
		// copy data from the slice!
		ret[key] = append([]byte(nil), it.Value().Data()...)

	}
	return ret, nil
}
