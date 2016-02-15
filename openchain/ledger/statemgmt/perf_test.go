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

package statemgmt

import (
	"flag"
	"github.com/openblockchain/obc-peer/openchain/ledger/testutil"
	"github.com/openblockchain/obc-peer/openchain/util"
	"testing"
)

func BenchmarkCryptoHash(b *testing.B) {
	flags := flag.NewFlagSet("testParams", flag.ExitOnError)
	numBytesPointer := flags.Int("NumBytes", -1, "Number of Bytes")
	flags.Parse(testParams)
	numBytes := *numBytesPointer
	if numBytes == -1 {
		b.Fatal("Missing value for parameter NumBytes")
	}

	randomBytes := testutil.ConstructRandomBytes(b, numBytes)
	//b.Logf("byte size=%d, b.N=%d", len(randomBytes), b.N)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		util.ComputeCryptoHash(randomBytes)
	}
}
