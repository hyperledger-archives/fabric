package statemgmt

import (
	"testing"

	"github.com/openblockchain/obc-peer/openchain/ledger/testutil"
)

// AssertIteratorContains - tests wether the iterator (itr) contains expected results (provided in map)
func AssertIteratorContains(t *testing.T, itr RangeScanIterator, expected map[string][]byte) {
	count := 0
	actual := make(map[string][]byte)
	for itr.Next() {
		count++
		k, v := itr.GetKeyValue()
		actual[k] = v
	}

	t.Logf("Results from iterator: %s", actual)
	testutil.AssertEquals(t, count, len(expected))
	for k, v := range expected {
		testutil.AssertEquals(t, actual[k], v)
	}
}
