RUNNING THE NODE CLIENT SDK TESTS
=================================

A. Prerequisites
----------------

1. Make sure that the chain is running with security enabled. For this, set "security" to "true" in the "peer/core.yaml" and "membersrvc/membersrvc.yaml" configurations files.
2. Make sure that the chain is using the same hash algorithm (default "SHA3") and the same security level (default "256") by setting the corresponding fields in "peer/core.yaml" and "membersrvc/membersrvc.yaml" respectively.
3. Build the node client sdk by running "make node-sdk" from the command line.

B. Running the Tests
--------------------

1. Start the membership services by running "rm -rf /var/hyperledger/production; cd ./membershipsrvc && ./membershipsrvc" from the command line.
2. Start the peer by running "cd peer && ./peer node start --peer-chaincodedev" from the command line.
3. Start the chaincode by running "cd examples/chaincode/go/chaincode_example02 && CORE_CHAINCODE_ID_NAME=mycc CORE_PEER_ADDRESS=0.0.0.0:30303 ./chaincode_example02" from the command line.
3. Run "test/unit/chain-test.js" and "test/unit/chain-test-conf.js" from the command line repeatedly for different hashes (SHA2, SHA3) and security levels (256, 384) by configuring the chain and node SDK accordingly followed by entering "rm -rf /tmp/keyValStore; node <test name>".

C. Coverage Tests
-----------------

1. Install "Istanbul" by entering "sudo npm install -g istanbul" from the command line.
2. Install "Istanbul-Combine" by entering "sudo npm install -g istanbul-combine" from the command line.
3. Run "test/unit/chain-test.js" and "test/unit/chain-test-conf.js" from the command line repeatedly for different hashes (SHA2, SHA3) and security levels (256, 384) by configuring the chain and client SDK accordingly (cf. Sections A and B) followed by entering "rm -rf /tmp/keyValStore; istanbul cover <test name>" for each test. After each test uniquely rename the "coverage" folder (e.g., "coverage-a", "coverage-b", "coverage-c").
4. Combine the various coverage reports by running "istanbul-combine -d coverage -r lcov -r html coverage-[a..z]/coverage.json" from the command line.
5. Enjoy the HTML report by opening coverage/lcov/index.html" in your favourite browser.

