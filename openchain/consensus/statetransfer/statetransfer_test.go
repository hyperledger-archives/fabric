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

package statetransfer

import (
	"sort"
	"testing"
)

func TestBlockRangeOrdering(t *testing.T) {
	lowRange := &blockRange{
		highBlock: 10,
		lowBlock:  5,
	}

	highRange := &blockRange{
		highBlock: 15,
		lowBlock:  12,
	}

	bigRange := &blockRange{
		highBlock: 15,
		lowBlock:  9,
	}

	slice := blockRangeSlice([]*blockRange{lowRange, highRange, bigRange})

	sort.Sort(slice)

	if slice[0] != bigRange {
		t.Fatalf("Big range should come first")
	}

	if slice[1] != highRange {
		t.Fatalf("High range should come second")
	}

	if slice[2] != lowRange {
		t.Fatalf("Low range should come third")
	}
}
