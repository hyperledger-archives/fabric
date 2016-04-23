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
	"crypto/sha256"
	"crypto/sha512"
	"golang.org/x/crypto/sha3"
	"hash"
	
	"github.com/hyperledger/fabric/core/crypto/conf"


)

func getHashSHA2(bitsize int) (hash.Hash, error) {
		switch bitsize {
		case 224:
			return sha256.New224(), nil
		case 256:
			return  sha256.New(), nil
		case 384:
			return sha512.New384(), nil
		case 512:
		    return sha512.New(), nil
		case 521:
			return sha512.New(),nil
		default:
			return nil, fmt.Errorf("Invalid bitsize. It was [%d]. Expected [224, 256, 384, 512, 521]", bitsize)
	}
}

func getHashSHA3(bitsize int) (hash.Hash, error) {
	switch bitsize {
		case 224:
			return sha3.New224(), nil
		case 256:
			return  sha3.New256(), nil
		case 384:
			return sha3.New384(), nil
		case 512:
		    return sha3.New512(), nil
		case 521:
			return sha3.New512(),nil
		default:
			return nil, fmt.Errorf("Invalid bitsize. It was [%d]. Expected [224, 256, 384, 512, 521]", bitsize)
	}
}


func computeHash(msg []byte, bitsize int) ([]byte, error) {
	var hash hash.Hash
    var err error;
	switch conf.GetHashAlgorithm() {
		case "SHA2":
			hash, err =  getHashSHA2(bitsize)
		case "SHA3":
			hash, err =  getHashSHA3(bitsize)
		default:
			return nil, fmt.Errorf("Invalid hash algorithm "+conf.GetHashAlgorithm())
		}

	if err != nil { 
		return nil, err
	}
	
	hash.Write(msg)
	return hash.Sum(nil), nil
}
