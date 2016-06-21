
# Test Hyperledger Peers
#
# Tags that can be used and will affect test internals:
#
#  @doNotDecompose will NOT decompose the named compose_yaml after scenario ends.  Useful for setting up environment and reviewing after scenario.
#
#  @chaincodeImagesUpToDate use this if all scenarios chaincode images are up to date, and do NOT require building.  BE SURE!!!

#@chaincodeImagesUpToDate
Feature: Chaincode02 example

#    @doNotDecompose
#    @wip
	Scenario: chaincode example02 with 1 peer
      	    Given we compose "docker-compose-1.yml"
	    When requesting "/chain" from "vp0"
	    Then I should get a JSON response with "height" = "1"

	    When I deploy chaincode "github.com/hyperledger/fabric/examples/chaincode/go/chaincode_example02" with ctor "init" to "vp0"
		     | arg1 |  arg2 | arg3 | arg4 |
		     |  a   |  100  |  b   |  200 |
	    Then I should have received a chaincode name
	    Then I wait up to "60" seconds for transaction to be committed to all peers

        When I query chaincode "example2" function name "query" on all peers:
            |arg1|
            |  a |
	    Then I should get a JSON response from all peers with "OK" = "100"

        When I invoke chaincode "example2" function name "invoke" on "vp0"
			|arg1|arg2|arg3|
			| a  | b  | 20 |
	    Then I should have received a transactionID
	    Then I wait up to "20" seconds for transaction to be committed to all peers

        When I query chaincode "example2" function name "query" on all peers:
            |arg1|
            |  a |
	    Then I should get a JSON response from all peers with "OK" = "80"

#@doNotDecompose
#    @wip
  Scenario: java SimpleSample chaincode example single peer
      Given we compose "docker-compose-1.yml"
      When requesting "/chain" from "vp0"
      Then I should get a JSON response with "height" = "1"
      	    When I deploy chaincode "/opt/gopath/src/github.com/hyperledger/fabric/java-shim" of "JAVA" with ctor "init" to "vp0"
      		     | arg1 |  arg2 | arg3 | arg4 |
      		     |  a   |  100  |  b   |  200 |
      	    Then I should have received a chaincode name
      	    Then I wait up to "60" seconds for transaction to be committed to all peers

      	    When requesting "/chain" from "vp0"
      	    Then I should get a JSON response with "height" = "2"

              When I query chaincode "example2" function name "query" on "vp0":
                  |arg1|
                  |  a |
      	    Then I should get a JSON response with "OK" = "100"


              When I invoke chaincode "example2" function name "transfer" on "vp0"
      			|arg1|arg2|arg3|
      			| a  | b  | 10 |
      	    Then I should have received a transactionID
      	    Then I wait up to "25" seconds for transaction to be committed to all peers

      	    When requesting "/chain" from "vp0"
      	    Then I should get a JSON response with "height" = "3"

              When I query chaincode "example2" function name "query" on "vp0":
                  |arg1|
                  |  a |
      	    Then I should get a JSON response with "OK" = "90"

              When I query chaincode "example2" function name "query" on "vp0":
                  |arg1|
                  |  b |
      	    Then I should get a JSON response with "OK" = "210"
