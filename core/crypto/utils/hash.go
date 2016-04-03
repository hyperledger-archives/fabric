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

package utils

import (
	"crypto/hmac"
	"github.com/hyperledger/fabric/core/crypto/conf"
	"hash"
)

// NewHash returns a new hash function
func NewHash() hash.Hash {
	return conf.GetDefaultHash()()
}

// Hash hashes the msh using the predefined hash function
func Hash(msg []byte) []byte {
	hash := NewHash()
	hash.Write(msg)
	return hash.Sum(nil)
}

// HMAC hmacs x using key key
func HMAC(key, x []byte) []byte {
	mac := hmac.New(conf.GetDefaultHash(), key)
	mac.Write(x)

	return mac.Sum(nil)
}

// HMACTruncated hmacs x using key key and truncate to truncation
func HMACTruncated(key, x []byte, truncation int) []byte {
	mac := hmac.New(conf.GetDefaultHash(), key)
	mac.Write(x)

	return mac.Sum(nil)[:truncation]
}
