#
# Test openchain Peers
#
# Tags that can be used and will affect test internals:
#
#  @doNotDecompose will NOT decompose the named compose_yaml after scenario ends.  Useful for setting up environment and reviewing after scenario.
#
#  @chaincodeImagesUpToDate use this if all scenarios chaincode images are up to date, and do NOT require building.  BE SURE!!!

#@chaincodeImagesUpToDate
Feature: lanching 3 peers
    As an openchain developer
    I want to be able to launch a 3 peers

#    @doNotDecompose
#    @wip
	Scenario: chaincode example 02 single peer
	    Given we compose "docker-compose-1.yml"
	    And I wait "1" seconds
	    When requesting "/chain" from "vp0"
	    Then I should get a JSON response with "height" = "1"
	    When I deploy chaincode "github.com/openblockchain/obc-peer/openchain/example/chaincode/chaincode_example02" with ctor "init" to "vp0"
		     | arg1 |  arg2 | arg3 | arg4 |
		     |  a   |  100  |  b   |  200 |
	    Then I should have received a chaincode name
	    Then I wait up to "25" seconds for transaction to be committed to all peers

	    When requesting "/chain" from "vp0"
	    Then I should get a JSON response with "height" = "2"

        When I query chaincode "example2" function name "query" on "vp0":
            |arg1|
            |  a |
	    Then I should get a JSON response with "OK" = "100"


        When I invoke chaincode "example2" function name "invoke" on "vp0"
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

#    @doNotDecompose
#    @wip
	Scenario: chaincode example02 with 5 peers, issue #520
	    Given we compose "docker-compose-5.yml"
	    And I wait "1" seconds
	    When requesting "/chain" from "vp0"
	    Then I should get a JSON response with "height" = "1"

	    When I deploy chaincode "github.com/openblockchain/obc-peer/openchain/example/chaincode/chaincode_example02" with ctor "init" to "vp0"
		     | arg1 |  arg2 | arg3 | arg4 |
		     |  a   |  100  |  b   |  200 |
	    Then I should have received a chaincode name
	    Then I wait up to "25" seconds for transaction to be committed to all peers

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


#    @doNotDecompose
#    @wip
	Scenario Outline: chaincode example02 with 4 peers and 1 obcca, issue #567

	    Given we compose "<ComposeFile>"
	    And I wait "2" seconds
	    And I register with CA supplying username "binhn" and secret "7avZQLwcUe9q" on peers:
             | vp0  |
        And I use the following credentials for querying peers:
		     | peer |   username  |    secret    |
		     | vp0  |  test_user0 | MS9qrN8hFjlE |
		     | vp1  |  test_user1 | jGlNl6ImkuDo |
		     | vp2  |  test_user2 | zMflqOKezFiA |
		     | vp3  |  test_user3 | vWdLCE00vJy0 |

	    When requesting "/chain" from "vp0"
	    Then I should get a JSON response with "height" = "1"
        And I wait "2" seconds
	    When I deploy chaincode "github.com/openblockchain/obc-peer/openchain/example/chaincode/chaincode_example02" with ctor "init" to "vp0"
		     | arg1 |  arg2 | arg3 | arg4 |
		     |  a   |  100  |  b   |  200 |
	    Then I should have received a chaincode name
	    Then I wait up to "<WaitTime>" seconds for transaction to be committed to peers:
            | vp0  | vp1 | vp2 | vp3 |

        When I query chaincode "example2" function name "query" with value "a" on peers:
            | vp0  | vp1 | vp2 | vp3 |
	    Then I should get a JSON response from peers with "OK" = "100"
            | vp0  | vp1 | vp2 | vp3 |

        When I invoke chaincode "example2" function name "invoke" on "vp0"
			|arg1|arg2|arg3|
			| a  | b  | 20 |
	    Then I should have received a transactionID
	    Then I wait up to "10" seconds for transaction to be committed to peers:
            | vp0  | vp1 | vp2 | vp3 |

        When I query chaincode "example2" function name "query" with value "a" on peers:
            | vp0  | vp1 | vp2 | vp3 |
	    Then I should get a JSON response from peers with "OK" = "80"
            | vp0  | vp1 | vp2 | vp3 |

    Examples: Consensus Options
        |          ComposeFile                     |   WaitTime   |
        |   docker-compose-4-consensus-noops.yml   |      60      |
        |   docker-compose-4-consensus-classic.yml |      20      |
        |   docker-compose-4-consensus-batch.yml   |      20      |
        |   docker-compose-4-consensus-sieve.yml   |      30      |


#   @doNotDecompose
#    @wip
	Scenario: basic startup of 3 validating peers
	    Given we compose "docker-compose-3.yml"
	    When requesting "/chain" from "vp0"
	    Then I should get a JSON response with "height" = "1"

 	@tls
#	@doNotDecompose
	Scenario: basic startup of 2 validating peers using TLS
	    Given we compose "docker-compose-2-tls-basic.yml"
	    When requesting "/chain" from "vp0"
	    Then I should get a JSON response with "height" = "1"
