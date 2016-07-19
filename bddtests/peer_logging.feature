#
# Test Logging Features of Peers
#
# Tags that can be used and will affect test internals:
#  @doNotDecompose will NOT decompose the named compose_yaml after scenario ends.  Useful for setting up environment and reviewing after scenario.
#  @chaincodeImagesUpToDate use this if all scenarios chaincode images are up to date, and do NOT require building.  BE SURE!!!

Feature: Peer Logging
    As a Fabric Developer
    I want to verify my Peers log correctly

    Scenario: Invoke is attempted after deploy in Dev Mode
    Given we compose "docker-compose-1-devmode.yml"
    When I deploy chaincode with name "testCC" and with ctor "init" to "vp0"
        | arg1 |  arg2 | arg3 | arg4 |
	    |  a   |  100  |  b   |  200 |
    And I invoke chaincode "example02" function name "invoke" on "vp0"
        |arg1|arg2|arg3|
		| a  | b  | 10 |
    Then ensure after 2 seconds there are no errors in the logs for peer vp0

    Scenario: Query is attempted after deploy in Dev Mode
    Given we compose "docker-compose-1-devmode.yml"
    When I deploy chaincode with name "testCC" and with ctor "init" to "vp0"
        | arg1 |  arg2 | arg3 | arg4 |
	    |  a   |  100  |  b   |  200 |
    And I query chaincode "example02" function name "query" on "vp0":
        |arg1|
        |  a |
    Then ensure after 2 seconds there are no errors in the logs for peer vp0

    Scenario: Invoke is attempted before deploy in Dev Mode
    Given we compose "docker-compose-1-devmode.yml"
    When I invoke chaincode "example02" function name "invoke" on "vp0"
        |arg1|arg2|arg3|
		| a  | b  | 10 |
    Then I wait up to 5 seconds for an error in the logs for peer vp0

    Scenario: Query is attempted before deploy in Dev Mode
    Given we compose "docker-compose-1-devmode.yml"
    When I query chaincode "example02" function name "query" on "vp0":
        |arg1|
        |  a |
    Then I wait up to 5 seconds for an error in the logs for peer vp0