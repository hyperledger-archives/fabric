#
# Test openchain state transfer protocol
#
# Tags that can be used and will affect test internals:
#
#  @doNotDecompose will NOT decompose the named compose_yaml after scenario ends.  Useful for setting up environment and reviewing after scenario.
#
#  @chaincodeImagesUpToDate use this if all scenarios chaincode images are up to date, and do NOT require building.  BE SURE!!!

#@chaincodeImagesUpToDate
Feature: Verify state transfer amongst validating peers

#    @doNotDecompose
#    @wip
	Scenario Outline: state transfer with chaincode example02, 5 peers (vp0-4) and 1 obcca.

# vp04 will not start until we signal it. If state transfer works correctly, vp04 will catch up with the other peers some time after we start it.

	    Given we compose "<ComposeFile>"
	    And I wait "5" seconds
	    And I register with CA supplying username "binhn" and secret "7avZQLwcUe9q" on peers:
             | vp0  |
        And I use the following credentials for querying peers:
		     | peer |   username  |    secret    |
		     | vp0  |  test_user0 | MS9qrN8hFjlE |
		     | vp1  |  test_user1 | jGlNl6ImkuDo |
		     | vp2  |  test_user2 | zMflqOKezFiA |
		     | vp3  |  test_user3 | vWdLCE00vJy0 |
           | vp4  |  test_user4 | AWdbCz0tvJ15 |

	    When requesting "/chain" from "vp0"
	    Then I should get a JSON response with "height" = "1"

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

       Then I start container "vp04"
       And I wait "5" seconds

       When I invoke chaincode "example2" function name "invoke" on "vp0"
        |arg1|arg2|arg3|
        | a  | b  | 20 |
       Then I should have received a transactionID

       When I invoke chaincode "example2" function name "invoke" on "vp0"
		  |arg1|arg2|arg3|
		  | a  | b  | 20 |
       Then I should have received a transactionID

       When I query chaincode "example2" function name "query" with value "a" on peers:
        | vp0  | vp1 | vp2 | vp3 | vp4 |
       Then I should get a JSON response from peers with "OK" = "40"
        | vp0  | vp1 | vp2 | vp3 | vp4 |

    Examples: Consensus Options
        |          ComposeFile                                    |   WaitTime   |
        |   docker-compose-5-consensus-sieve-state_transfer.yml   |      10      |
#        |   docker-compose-4-consensus-classic.yml |      30      |
#        |   docker-compose-4-consensus-batch.yml   |      30      |
#        |   docker-compose-4-consensus-sieve.yml   |      10      |


# 
