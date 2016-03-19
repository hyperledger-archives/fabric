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
  Scenario: Range query test, single peer, issue #767
    Given we compose "docker-compose-1.yml"
      And I wait "1" seconds
      When requesting "/chain" from "vp0"
      Then I should get a JSON response with "height" = "1"
      When I deploy chaincode "github.com/openblockchain/obc-peer/examples/chaincode/go/map" with ctor "init" to "vp0"
      ||
      ||

      Then I should have received a chaincode name
      Then I wait up to "60" seconds for transaction to be committed to all peers

      When requesting "/chain" from "vp0"
      Then I should get a JSON response with "height" = "2"

      When I invoke chaincode "map" function name "put" on "vp0"
        | arg1 | arg2 |
        | key1  | value1  |
      Then I should have received a transactionID
      Then I wait up to "25" seconds for transaction to be committed to all peers

      When requesting "/chain" from "vp0"
      Then I should get a JSON response with "height" = "3"

      When I query chaincode "map" function name "get" on "vp0":
        | arg1|
        | key1 |
      Then I should get a JSON response with "OK" = "value1"

      When I invoke chaincode "map" function name "put" on "vp0"
        | arg1 | arg2 |
        | key2  | value2  |
      Then I should have received a transactionID
      Then I wait up to "25" seconds for transaction to be committed to all peers

      When requesting "/chain" from "vp0"
      Then I should get a JSON response with "height" = "4"

      When I query chaincode "map" function name "keys" on "vp0":
        ||
        ||
      Then I should get a JSON response with "OK" = "["key2","key1"]"

      When I invoke chaincode "map" function name "remove" on "vp0"
        | arg1 | |
        | key1  | |
      Then I should have received a transactionID
      Then I wait up to "25" seconds for transaction to be committed to all peers

      When requesting "/chain" from "vp0"
      Then I should get a JSON response with "height" = "5"

      When I query chaincode "map" function name "keys" on "vp0":
        ||
        ||
      Then I should get a JSON response with "OK" = "["key2"]"

#    @doNotDecompose
#    @wip
  Scenario: chaincode shim table API, issue 477
    Given we compose "docker-compose-1.yml"
      And I wait "1" seconds
      When requesting "/chain" from "vp0"
      Then I should get a JSON response with "height" = "1"
      When I deploy chaincode "github.com/openblockchain/obc-peer/openchain/behave_chaincode/go/table" with ctor "init" to "vp0"
      ||
      ||
      Then I should have received a chaincode name
      Then I wait up to "60" seconds for transaction to be committed to all peers
      When requesting "/chain" from "vp0"
      Then I should get a JSON response with "height" = "2"

      When I invoke chaincode "table_test" function name "insertRowTableOne" on "vp0"
        | arg1 | arg2 | arg3 |
        | test1| 10   | 20   |
      Then I should have received a transactionID
      Then I wait up to "25" seconds for transaction to be committed to all peers
      When requesting "/chain" from "vp0"
      Then I should get a JSON response with "height" = "3"

      When I invoke chaincode "table_test" function name "insertRowTableOne" on "vp0"
        | arg1 | arg2 | arg3 |
        | test2| 10   | 20   |
      Then I should have received a transactionID
      Then I wait up to "25" seconds for transaction to be committed to all peers
      When requesting "/chain" from "vp0"
      Then I should get a JSON response with "height" = "4"

      When I query chaincode "table_test" function name "getRowTableOne" on "vp0":
        | arg1 |
        | test1|
      Then I should get a JSON response with "OK" = "{[string:"test1"  int32:10  int32:20 ]}"

      When I invoke chaincode "table_test" function name "insertRowTableTwo" on "vp0"
        | arg1 | arg2 | arg3 | arg3 |
        | foo2 | 34   | 65   | bar8 |
      Then I should have received a transactionID
      Then I wait up to "25" seconds for transaction to be committed to all peers
      When requesting "/chain" from "vp0"
      Then I should get a JSON response with "height" = "5"

      When I query chaincode "table_test" function name "getRowTableTwo" on "vp0":
        | arg1 | arg2 | arg3 |
        | foo2 | 65   | bar8 |
      Then I should get a JSON response with "OK" = "{[string:"foo2"  int32:34  int32:65  string:"bar8" ]}"

      When I invoke chaincode "table_test" function name "replaceRowTableOne" on "vp0"
        | arg1 | arg2 | arg3 |
        | test1| 30   | 40   |
      Then I should have received a transactionID
      Then I wait up to "25" seconds for transaction to be committed to all peers
      When requesting "/chain" from "vp0"
      Then I should get a JSON response with "height" = "6"

      When I query chaincode "table_test" function name "getRowTableOne" on "vp0":
        | arg1 |
        | test1|
      Then I should get a JSON response with "OK" = "{[string:"test1"  int32:30  int32:40 ]}"

      When I invoke chaincode "table_test" function name "deleteRowTableOne" on "vp0"
        | arg1 |
        | test1|
      Then I should have received a transactionID
      Then I wait up to "25" seconds for transaction to be committed to all peers
      When requesting "/chain" from "vp0"
      Then I should get a JSON response with "height" = "7"

      When I query chaincode "table_test" function name "getRowTableOne" on "vp0":
        | arg1 |
        | test1|
      Then I should get a JSON response with "OK" = "{[]}"

      When I query chaincode "table_test" function name "getRowTableOne" on "vp0":
        | arg1 |
        | test2|
      Then I should get a JSON response with "OK" = "{[string:"test2"  int32:10  int32:20 ]}"

      When I invoke chaincode "table_test" function name "insertRowTableOne" on "vp0"
        | arg1 | arg2 | arg3 |
        | test3| 10   | 20   |
      Then I should have received a transactionID
      Then I wait up to "25" seconds for transaction to be committed to all peers
      When requesting "/chain" from "vp0"
      Then I should get a JSON response with "height" = "8"

      When I invoke chaincode "table_test" function name "insertRowTableOne" on "vp0"
        | arg1 | arg2 | arg3 |
        | test4| 10   | 20   |
      Then I should have received a transactionID
      Then I wait up to "25" seconds for transaction to be committed to all peers
      When requesting "/chain" from "vp0"
      Then I should get a JSON response with "height" = "9"

      When I invoke chaincode "table_test" function name "insertRowTableOne" on "vp0"
        | arg1 | arg2 | arg3 |
        | test5| 10   | 20   |
      Then I should have received a transactionID
      Then I wait up to "25" seconds for transaction to be committed to all peers
      When requesting "/chain" from "vp0"
      Then I should get a JSON response with "height" = "10"

      When I query chaincode "table_test" function name "getRowTableOne" on "vp0":
        | arg1 |
        | test3|
      Then I should get a JSON response with "OK" = "{[string:"test3"  int32:10  int32:20 ]}"

      When I query chaincode "table_test" function name "getRowTableOne" on "vp0":
        | arg1 |
        | test4|
      Then I should get a JSON response with "OK" = "{[string:"test4"  int32:10  int32:20 ]}"

      When I query chaincode "table_test" function name "getRowTableOne" on "vp0":
        | arg1 |
        | test5|
      Then I should get a JSON response with "OK" = "{[string:"test5"  int32:10  int32:20 ]}"

      When I invoke chaincode "table_test" function name "insertRowTableTwo" on "vp0"
        | arg1 | arg2 | arg3 | arg3 |
        | foo2 | 35   | 65   | bar10 |
      Then I should have received a transactionID
      Then I wait up to "25" seconds for transaction to be committed to all peers
      When requesting "/chain" from "vp0"
      Then I should get a JSON response with "height" = "11"

      When I invoke chaincode "table_test" function name "insertRowTableTwo" on "vp0"
        | arg1 | arg2 | arg3 | arg3 |
        | foo2 | 36   | 65   | bar11 |
      Then I should have received a transactionID
      Then I wait up to "25" seconds for transaction to be committed to all peers
      When requesting "/chain" from "vp0"
      Then I should get a JSON response with "height" = "12"

      When I invoke chaincode "table_test" function name "insertRowTableTwo" on "vp0"
        | arg1 | arg2 | arg3 | arg3 |
        | foo2 | 37   | 65   | bar12 |
      Then I should have received a transactionID
      Then I wait up to "25" seconds for transaction to be committed to all peers
      When requesting "/chain" from "vp0"
      Then I should get a JSON response with "height" = "13"

      When I invoke chaincode "table_test" function name "insertRowTableTwo" on "vp0"
        | arg1 | arg2 | arg3 | arg3 |
        | foo2 | 38   | 66   | bar10 |
      Then I should have received a transactionID
      Then I wait up to "25" seconds for transaction to be committed to all peers
      When requesting "/chain" from "vp0"
      Then I should get a JSON response with "height" = "14"

      When I query chaincode "table_test" function name "getRowsTableTwo" on "vp0":
        | arg1 | arg2 |
        | foo2 | 65   |
      Then I should get a JSON response with "OK" = "[{"columns":[{"Value":{"String_":"foo2"}},{"Value":{"Int32":37}},{"Value":{"Int32":65}},{"Value":{"String_":"bar12"}}]},{"columns":[{"Value":{"String_":"foo2"}},{"Value":{"Int32":34}},{"Value":{"Int32":65}},{"Value":{"String_":"bar8"}}]},{"columns":[{"Value":{"String_":"foo2"}},{"Value":{"Int32":35}},{"Value":{"Int32":65}},{"Value":{"String_":"bar10"}}]},{"columns":[{"Value":{"String_":"foo2"}},{"Value":{"Int32":36}},{"Value":{"Int32":65}},{"Value":{"String_":"bar11"}}]}]"

      When I query chaincode "table_test" function name "getRowsTableTwo" on "vp0":
        | arg1 | arg2 |
        | foo2 | 66   |
      Then I should get a JSON response with "OK" = "[{"columns":[{"Value":{"String_":"foo2"}},{"Value":{"Int32":38}},{"Value":{"Int32":66}},{"Value":{"String_":"bar10"}}]}]"

      When I query chaincode "table_test" function name "getRowsTableTwo" on "vp0":
        | arg1 |
        | foo2 |
      Then I should get a JSON response with "OK" = "[{"columns":[{"Value":{"String_":"foo2"}},{"Value":{"Int32":37}},{"Value":{"Int32":65}},{"Value":{"String_":"bar12"}}]},{"columns":[{"Value":{"String_":"foo2"}},{"Value":{"Int32":34}},{"Value":{"Int32":65}},{"Value":{"String_":"bar8"}}]},{"columns":[{"Value":{"String_":"foo2"}},{"Value":{"Int32":38}},{"Value":{"Int32":66}},{"Value":{"String_":"bar10"}}]},{"columns":[{"Value":{"String_":"foo2"}},{"Value":{"Int32":35}},{"Value":{"Int32":65}},{"Value":{"String_":"bar10"}}]},{"columns":[{"Value":{"String_":"foo2"}},{"Value":{"Int32":36}},{"Value":{"Int32":65}},{"Value":{"String_":"bar11"}}]}]"

      When I invoke chaincode "table_test" function name "deleteAndRecreateTableOne" on "vp0"
        ||
        ||
      Then I should have received a transactionID
      Then I wait up to "25" seconds for transaction to be committed to all peers
      When requesting "/chain" from "vp0"
      Then I should get a JSON response with "height" = "15"

      When I query chaincode "table_test" function name "getRowTableOne" on "vp0":
        | arg1 |
        | test3|
      Then I should get a JSON response with "OK" = "{[]}"

      When I query chaincode "table_test" function name "getRowTableOne" on "vp0":
        | arg1 |
        | test4|
      Then I should get a JSON response with "OK" = "{[]}"

      When I query chaincode "table_test" function name "getRowTableOne" on "vp0":
        | arg1 |
        | test5|
      Then I should get a JSON response with "OK" = "{[]}"

      When I query chaincode "table_test" function name "getRowTableOne" on "vp0":
        | arg1 |
        | test2|
      Then I should get a JSON response with "OK" = "{[]}"

      When I query chaincode "table_test" function name "getRowTableTwo" on "vp0":
        | arg1 | arg2 | arg3 |
        | foo2 | 65   | bar8 |
      Then I should get a JSON response with "OK" = "{[string:"foo2"  int32:34  int32:65  string:"bar8" ]}"

      When I invoke chaincode "table_test" function name "insertRowTableThree" on "vp0"
        | arg1 | arg2 | arg3 | arg4 | arg5 | arg6 | arg7 |
        | foo2 | -38  | -66  | 77   | 88   | hello| true |
      Then I should have received a transactionID
      Then I wait up to "25" seconds for transaction to be committed to all peers
      When requesting "/chain" from "vp0"
      Then I should get a JSON response with "height" = "16"

      When I query chaincode "table_test" function name "getRowTableThree" on "vp0":
        | arg1 |
        | foo2 |
      Then I should get a JSON response with "OK" = "{[string:"foo2"  int32:-38  int64:-66  uint32:77  uint64:88  bytes:"hello"  bool:true ]}"


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
	    Then I wait up to "60" seconds for transaction to be committed to all peers

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


#    @doNotDecompose
#    @wip
    @issue_567
	Scenario Outline: chaincode example02 with 4 peers and 1 obcca, issue #567

	    Given we compose "<ComposeFile>"
	    And I wait "3" seconds
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
        |   docker-compose-4-consensus-classic.yml |      60      |
        |   docker-compose-4-consensus-batch.yml   |      60      |
        |   docker-compose-4-consensus-sieve.yml   |      60      |


    #@doNotDecompose
    @wip
    #@skip
	Scenario Outline: chaincode example02 with 4 peers and 1 obcca, issue #680 (State transfer)

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

	    When requesting "/chain" from "vp0"
	        Then I should get a JSON response with "height" = "1"


            # Deploy
	    When I deploy chaincode "github.com/openblockchain/obc-peer/openchain/example/chaincode/chaincode_example02" with ctor "init" to "vp0"
                    | arg1 |  arg2 | arg3 | arg4 |
                    |  a   |  100  |  b   |  200 |
	        Then I should have received a chaincode name
	        Then I wait up to "<WaitTime>" seconds for transaction to be committed to peers:
                     | vp0  | vp1 | vp2 |

            # Build up a sizable blockchain, that vp3 will need to validate at startup
            When I invoke chaincode "example2" function name "invoke" on "vp0" "30" times
                    |arg1|arg2|arg3|
                    | b  | a  | 1  |
	        Then I should have received a transactionID
	        Then I wait up to "10" seconds for transaction to be committed to peers:
                    | vp0  | vp1 | vp2 | vp3 |

            When I query chaincode "example2" function name "query" with value "a" on peers:
                    | vp0  | vp1 | vp2 | vp3 |
	        Then I should get a JSON response from peers with "OK" = "130"
                    | vp0  | vp1 | vp2 | vp3 |

        # STOPPING vp3!!!!!!!!!!!!!!!!!!!!!!!!!!
        Given I stop peers:
            | vp3  |

        # Invoke a transaction to get vp3 out of sync
        When I invoke chaincode "example2" function name "invoke" on "vp0"
			|arg1|arg2|arg3|
			| a  | b  | 10 |
	    Then I should have received a transactionID
	    Then I wait up to "10" seconds for transaction to be committed to peers:
            | vp0  | vp1 | vp2 |

        When I query chaincode "example2" function name "query" with value "a" on peers:
            | vp0  | vp1 | vp2 |
	    Then I should get a JSON response from peers with "OK" = "120"
            | vp0  | vp1 | vp2 |

        # Now start vp3 again and run 8 more transactions
        Given I start peers:
            | vp3  |
        And I wait "5" seconds

        # Invoke 6 more txs, this will trigger a state transfer, set a target, and execute new outstanding transactions
        When I invoke chaincode "example2" function name "invoke" on "vp0" "6" times
			|arg1|arg2|arg3|
			| a  | b  | 10 |
	    Then I should have received a transactionID
	    Then I wait up to "60" seconds for transaction to be committed to peers:
            | vp0  | vp1 | vp2 | vp3 |
        When I query chaincode "example2" function name "query" with value "a" on peers:
            | vp0  | vp1 | vp2 | vp3 |
	    Then I should get a JSON response from peers with "OK" = "60"
            | vp0  | vp1 | vp2 | vp3 |


    Examples: Consensus Options
        |          ComposeFile                       |   WaitTime   |
        |   docker-compose-4-consensus-classic.yml   |      60      |
        |   docker-compose-4-consensus-batch.yml     |      60      |
        |   docker-compose-4-consensus-sieve.yml     |      60      |


#    @doNotDecompose
    @issue_724
	Scenario Outline: chaincode example02 with 4 peers and 1 obcca, issue #724

	    Given we compose "<ComposeFile>"
	    And I wait "3" seconds
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

        Given I stop peers:
           | vp0  |  vp1   | vp2  | vp3  |
        And I wait "1" seconds

        Given I start peers:
           | vp0  |  vp1   | vp2  | vp3  |
        And I wait "5" seconds

        When I query chaincode "example2" function name "query" with value "a" on peers:
            | vp3  |
	    Then I should get a JSON response from peers with "OK" = "100"
            | vp3  |

    Examples: Consensus Options
        |          ComposeFile                     |   WaitTime   |
        |   docker-compose-4-consensus-noops.yml   |      60      |


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

#@doNotDecompose
#@wip
#@skip
   Scenario Outline: 4 peers and 1 obcca, consensus still works if one backup replica fails

      Given we compose "<ComposeFile>"
      And I wait "10" seconds
      And I use the following credentials for querying peers:
         | peer |   username  |    secret    |
         | vp0  |  test_user0 | MS9qrN8hFjlE |
         | vp1  |  test_user1 | jGlNl6ImkuDo |
         | vp2  |  test_user2 | zMflqOKezFiA |
         | vp3  |  test_user3 | vWdLCE00vJy0 |
      And I register with CA supplying username "test_user0" and secret "MS9qrN8hFjlE" on peers:
         | vp0 |

      When requesting "/chain" from "vp0"
       Then I should get a JSON response with "height" = "1"

      # Deploy
      When I deploy chaincode "github.com/openblockchain/obc-peer/openchain/example/chaincode/chaincode_example02" with ctor "init" to "vp0"
         | arg1 |  arg2 | arg3 | arg4 |
         |  a   |  100  |  b   |  200 |
       Then I should have received a chaincode name
       Then I wait up to "<WaitTime>" seconds for transaction to be committed to peers:
         | vp0  | vp1 | vp2 | vp3 |

      # get things started. All peers up and executing Txs
      When I invoke chaincode "example2" function name "invoke" on "vp0" "5" times
           |arg1|arg2|arg3|
           | a  | b  | 1  |
       Then I should have received a transactionID
       Then I wait up to "3" seconds for transaction to be committed to peers:
           | vp0  | vp1 | vp2 | vp3 |

      When I query chaincode "example2" function name "query" with value "a" on peers:
           | vp0  | vp1 | vp2 | vp3 |
       Then I should get a JSON response from peers with "OK" = "95"
           | vp0  | vp1 | vp2 | vp3 |

      # STOP vp2
      Given I stop peers:
         | vp2  |
       And I wait "3" seconds

      # continue invoking Txs
      When I invoke chaincode "example2" function name "invoke" on "vp0" "5" times
         |arg1|arg2|arg3|
         | a  | b  | 1 |
       Then I should have received a transactionID
       Then I wait up to "3" seconds for transaction to be committed to peers:
           | vp0  | vp1 | vp3 |

      When I query chaincode "example2" function name "query" with value "a" on peers:
        | vp0  | vp1 | vp3 |
       Then I should get a JSON response from peers with "OK" = "90"
        | vp0 | vp1 | vp3 |

   Examples: Consensus Options
       |          ComposeFile                       |   WaitTime   |
       |   docker-compose-4-consensus-classic.yml   |      60      |
       |   docker-compose-4-consensus-batch.yml     |      60      |
       |   docker-compose-4-consensus-sieve.yml     |      60      |

#@doNotDecompose
#@wip
#@skip
 Scenario Outline: 4 peers and 1 obcca, consensus fails if 2 backup replicas fail

    Given we compose "<ComposeFile>"
    And I wait "10" seconds
    And I use the following credentials for querying peers:
       | peer |   username  |    secret    |
       | vp0  |  test_user0 | MS9qrN8hFjlE |
       | vp1  |  test_user1 | jGlNl6ImkuDo |
       | vp2  |  test_user2 | zMflqOKezFiA |
       | vp3  |  test_user3 | vWdLCE00vJy0 |
    And I register with CA supplying username "test_user0" and secret "MS9qrN8hFjlE" on peers:
       | vp0 |

    When requesting "/chain" from "vp0"
     Then I should get a JSON response with "height" = "1"

    # Deploy
    When I deploy chaincode "github.com/openblockchain/obc-peer/openchain/example/chaincode/chaincode_example02" with ctor "init" to "vp0"
       | arg1 |  arg2 | arg3 | arg4 |
       |  a   |  100  |  b   |  200 |
     Then I should have received a chaincode name
     Then I wait up to "<WaitTime>" seconds for transaction to be committed to peers:
       | vp0  | vp1 | vp2 | vp3 |

    # get things started. All peers up and executing Txs
    When I invoke chaincode "example2" function name "invoke" on "vp0" "5" times
         |arg1|arg2|arg3|
         | a  | b  | 1  |
     Then I should have received a transactionID
     Then I wait up to "5" seconds for transaction to be committed to peers:
         | vp0  | vp1 | vp2 | vp3 |

    When I query chaincode "example2" function name "query" with value "a" on peers:
         | vp0  | vp1 | vp2 | vp3 |
     Then I should get a JSON response from peers with "OK" = "95"
         | vp0  | vp1 | vp2 | vp3 |

    # STOP vp2
    Given I stop peers:
       | vp1  | vp2 |
     And I wait "3" seconds

    # continue invoking Txs
    When I invoke chaincode "example2" function name "invoke" on "vp0" "5" times
       |arg1|arg2|arg3|
       | a  | b  | 1 |
     And I wait "5" seconds

    When I query chaincode "example2" function name "query" with value "a" on peers:
      | vp0 | vp3 |
     Then I should get a JSON response from peers with "OK" = "95"
      | vp0 | vp3 |

 Examples: Consensus Options
     |          ComposeFile                       |   WaitTime   |
     |   docker-compose-4-consensus-classic.yml   |      60      |
     |   docker-compose-4-consensus-batch.yml     |      60      |
     |   docker-compose-4-consensus-sieve.yml     |      60      |

     #@doNotDecompose
     #@wip
     #@skip
      Scenario Outline: 4 peers and 1 obcca, consensus still works if 1 peer (vp3) is byzantine

         Given we compose "<ComposeFile>"
         And I wait "10" seconds
         And I use the following credentials for querying peers:
            | peer |   username  |    secret    |
            | vp0  |  test_user0 | MS9qrN8hFjlE |
            | vp1  |  test_user1 | jGlNl6ImkuDo |
            | vp2  |  test_user2 | zMflqOKezFiA |
            | vp3  |  test_user3 | vWdLCE00vJy0 |
         And I register with CA supplying username "test_user0" and secret "MS9qrN8hFjlE" on peers:
            | vp0 |

         When requesting "/chain" from "vp0"
          Then I should get a JSON response with "height" = "1"

         # Deploy
         When I deploy chaincode "github.com/openblockchain/obc-peer/openchain/example/chaincode/chaincode_example02" with ctor "init" to "vp0"
            | arg1 |  arg2 | arg3 | arg4 |
            |  a   |  100  |  b   |  200 |
          Then I should have received a chaincode name
          Then I wait up to "<WaitTime>" seconds for transaction to be committed to peers:
            | vp0  | vp1 | vp2 |

         When I invoke chaincode "example2" function name "invoke" on "vp0" "50" times
              |arg1|arg2|arg3|
              | a  | b  | 1  |
          Then I should have received a transactionID
          Then I wait up to "5" seconds for transaction to be committed to peers:
              | vp0  | vp1 | vp2 |

         When I query chaincode "example2" function name "query" with value "a" on peers:
              | vp0  | vp1 | vp2 |
          Then I should get a JSON response from peers with "OK" = "50"
              | vp0  | vp1 | vp2 |

      Examples: Consensus Options
          |          ComposeFile                                   |   WaitTime   |
          |   docker-compose-4-consensus-classic-1-byzantine.yml   |      60      |
          |   docker-compose-4-consensus-batch-1-byzantine.yml     |      60      |
          |   docker-compose-4-consensus-sieve-1-byzantine.yml     |      60      |
