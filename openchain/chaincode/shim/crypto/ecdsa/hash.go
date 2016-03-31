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

package ecdsa

import (
	"fmt"
	"golang.org/x/crypto/sha3"
	"hash"
)

func computeHash(msg []byte, bitsize int) ([]byte, error) {
	var hash hash.Hash

	switch bitsize {
	case 224:
		hash = sha3.New224()
	case 256:
		hash = sha3.New256()
	case 384:
		hash = sha3.New384()
	case 512:
		hash = sha3.New512()
	case 521:
		hash = sha3.New512()
	default:
		return nil, fmt.Errorf("Invalid bitsize. It was [%d]. Expected [224, 256, 384, 512, 521]", bitsize)
	}

	hash.Write(msg)
	return hash.Sum(nil), nil
}
