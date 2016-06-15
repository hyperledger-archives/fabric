/*
Copyright IBM Corp. 2016 All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

		 http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package rest

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hyperledger/fabric/core/ledger"
	"github.com/hyperledger/fabric/protos"
)

func performHTTPGet(t *testing.T, url string) []byte {
	response, err := http.Get(url)
	if err != nil {
		t.Fatalf("Error attempt to access /chain: %v", err)
	}
	body, err := ioutil.ReadAll(response.Body)
	response.Body.Close()
	if err != nil {
		t.Fatalf("Error reading HTTP resposne body: %v", err)
	}
	return body
}

func parseRESTResult(t *testing.T, body []byte) restResult {
	var res restResult
	err := json.Unmarshal(body, &res)
	if err != nil {
		t.Fatalf("Invalid JSON response: %v", err)
	}
	return res
}

func initGlobalServerOpenchain(t *testing.T) {
	var err error
	serverOpenchain, err = NewOpenchainServerWithPeerInfo(new(peerInfo))
	if err != nil {
		t.Fatalf("Error creating OpenchainServer: %s", err)
	}
}

func TestServerOpenchainREST_API_GetBlockchainInfo(t *testing.T) {
	// Construct a ledger with 0 blocks.
	ledger := ledger.InitTestLedger(t)

	initGlobalServerOpenchain(t)

	// Start the HTTP REST test server
	httpServer := httptest.NewServer(buildOpenchainRESTRouter())
	defer httpServer.Close()

	body := performHTTPGet(t, httpServer.URL+"/chain")
	res := parseRESTResult(t, body)
	if res.Error == "" {
		t.Errorf("Expected an error when retrieving empty blockchain, but got none")
	}

	// add 3 blocks to the ledger
	buildTestLedger1(ledger, t)

	body3 := performHTTPGet(t, httpServer.URL+"/chain")
	var blockchainInfo3 protos.BlockchainInfo
	err := json.Unmarshal(body3, &blockchainInfo3)
	if err != nil {
		t.Fatalf("Invalid JSON response: %v", err)
	}
	if blockchainInfo3.Height != 3 {
		t.Errorf("Expected blockchain height to be 3 but got %v", blockchainInfo3.Height)
	}

	// add 5 more blocks more to the ledger
	buildTestLedger2(ledger, t)

	body8 := performHTTPGet(t, httpServer.URL+"/chain")
	var blockchainInfo8 protos.BlockchainInfo
	err = json.Unmarshal(body8, &blockchainInfo8)
	if err != nil {
		t.Fatalf("Invalid JSON response: %v", err)
	}
	if blockchainInfo8.Height != 8 {
		t.Errorf("Expected blockchain height to be 8 but got %v", blockchainInfo8.Height)
	}
}

func TestServerOpenchainREST_API_GetBlockByNumber(t *testing.T) {
	// Construct a ledger with 0 blocks.
	ledger := ledger.InitTestLedger(t)

	initGlobalServerOpenchain(t)

	// Start the HTTP REST test server
	httpServer := httptest.NewServer(buildOpenchainRESTRouter())
	defer httpServer.Close()

	body := performHTTPGet(t, httpServer.URL+"/chain/blocks/0")
	res := parseRESTResult(t, body)
	if res.Error == "" {
		t.Errorf("Expected an error when retrieving block 0 of an empty blockchain, but got none")
	}

	// add 3 blocks to the ledger
	buildTestLedger1(ledger, t)

	// Retrieve the first block from the blockchain (block number = 0)
	body0 := performHTTPGet(t, httpServer.URL+"/chain/blocks/0")
	var block0 protos.Block
	err := json.Unmarshal(body0, &block0)
	if err != nil {
		t.Fatalf("Invalid JSON response: %v", err)
	}

	// Retrieve the 3rd block from the blockchain (block number = 2)
	body2 := performHTTPGet(t, httpServer.URL+"/chain/blocks/2")
	var block2 protos.Block
	err = json.Unmarshal(body2, &block2)
	if err != nil {
		t.Fatalf("Invalid JSON response: %v", err)
	}
	if len(block2.Transactions) != 2 {
		t.Errorf("Expected block to contain 2 transactions but got %v", len(block2.Transactions))
	}

	// Retrieve the 5th block from the blockchain (block number = 4), which
	// should fail because the ledger has only 3 blocks.
	body4 := performHTTPGet(t, httpServer.URL+"/chain/blocks/4")
	res4 := parseRESTResult(t, body4)
	if res4.Error == "" {
		t.Errorf("Expected an error when retrieving non-existing block, but got none")
	}
}

func TestServerOpenchainREST_API_GetTransactionByUUID(t *testing.T) {
	startTime := time.Now().Unix()

	// Construct a ledger with 3 blocks.
	ledger := ledger.InitTestLedger(t)
	buildTestLedger1(ledger, t)

	initGlobalServerOpenchain(t)

	// Start the HTTP REST test server
	httpServer := httptest.NewServer(buildOpenchainRESTRouter())
	defer httpServer.Close()

	body := performHTTPGet(t, httpServer.URL+"/transactions/NON-EXISTING-UUID")
	res := parseRESTResult(t, body)
	if res.Error == "" {
		t.Errorf("Expected an error when retrieving non-existing transaction, but got none")
	}

	block1, err := ledger.GetBlockByNumber(1)
	if err != nil {
		t.Fatalf("Can't fetch first block from ledger: %v", err)
	}
	firstTx := block1.Transactions[0]

	body1 := performHTTPGet(t, httpServer.URL+"/transactions/"+firstTx.Uuid)
	var tx1 protos.Transaction
	err = json.Unmarshal(body1, &tx1)
	if err != nil {
		t.Fatalf("Invalid JSON response: %v", err)
	}
	if tx1.Uuid != firstTx.Uuid {
		t.Errorf("Expected transaction uuid to be '%v' but got '%v'", firstTx.Uuid, tx1.Uuid)
	}
	if tx1.Timestamp.Seconds < startTime {
		t.Errorf("Expected transaction timestamp (%v) to be after the start time (%v)", tx1.Timestamp.Seconds, startTime)
	}

	badBody := performHTTPGet(t, httpServer.URL+"/transactions/with-\"-chars-in-the-URL")
	badRes := parseRESTResult(t, badBody)
	if badRes.Error == "" {
		t.Errorf("Expected a proper error when retrieive transaction with bad UUID")
	}
}

func TestServerOpenchainREST_API_GetEnrollmentID(t *testing.T) {
	initGlobalServerOpenchain(t)

	// Start the HTTP REST test server
	httpServer := httptest.NewServer(buildOpenchainRESTRouter())
	defer httpServer.Close()

	body := performHTTPGet(t, httpServer.URL+"/registrar/NON-EXISTING-USER")
	res := parseRESTResult(t, body)
	if res.Error == "" {
		t.Errorf("Expected an error when retrieving non-existing user, but got none")
	}

	body = performHTTPGet(t, httpServer.URL+"/registrar/BAD-\"-CHARS")
	res = parseRESTResult(t, body)
	if res.Error == "" {
		t.Errorf("Expected an error when retrieving non-existing user, but got none")
	}

}

func TestServerOpenchainREST_API_GetPeers(t *testing.T) {
	initGlobalServerOpenchain(t)

	// Start the HTTP REST test server
	httpServer := httptest.NewServer(buildOpenchainRESTRouter())
	defer httpServer.Close()

	body := performHTTPGet(t, httpServer.URL+"/network/peers")
	var msg protos.PeersMessage
	err := json.Unmarshal(body, &msg)
	if err != nil {
		t.Fatalf("Invalid JSON response: %v", err)
	}
	if len(msg.Peers) != 1 {
		t.Errorf("Expected a list of 1 peer but got %d peers", len(msg.Peers))
	}
	if msg.Peers[0].ID.Name != "jdoe" {
		t.Errorf("Expected a 'jdoe' peer but got '%s'", msg.Peers[0].ID.Name)
	}
}
