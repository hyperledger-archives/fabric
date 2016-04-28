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

package conf

import (
	"hash"
)

var (
	defaultHash func() hash.Hash
	hashAlgorithm string
	hashLength int
)

// GetDefaultHash returns the default hash function used by the crypto layer
func GetDefaultHash() func() hash.Hash {
	return defaultHash
}

// GetHashAlgorithm returns the defualt hash algorithm used.
func GetHashAlgorithm() string {
	return hashAlgorithm
}

// GetHashLength returns the default hash length used.
func GetHashLength() int {
	return hashLength
}
