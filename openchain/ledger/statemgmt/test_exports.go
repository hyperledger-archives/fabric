package statemgmt

import (
	crand "crypto/rand"
	"math/rand"
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

func ConstructRandomStateDelta(
	t testing.TB,
	chaincodeIDPrefix string,
	numChaincodes int,
	maxKeySuffix int,
	numKeysToInsert int,
	valueSize int) *StateDelta {
	delta := NewStateDelta()
	for i := 0; i < numKeysToInsert; i++ {
		chaincodeID := chaincodeIDPrefix + "_" + string(rand.Intn(numChaincodes))
		key := "key_" + string(rand.Intn(maxKeySuffix))
		value := make([]byte, valueSize)
		_, err := crand.Read(value)
		if err != nil {
			t.Fatalf("Error while generating random bytes: %s", err)
		}
		delta.Set(chaincodeID, key, value, nil)
	}
	return delta
}
