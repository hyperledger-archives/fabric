# Java chaincode 

Note: This guide generally assumes you have followed the development environment setup tutorial [here](https://github.com/hyperledger/fabric/blob/master/docs/API/SandboxSetup.md).
## To get started developing Java chaincode: 
1. Ensure you have gradle
  * Download the binary distribution from [http://gradle.org/gradle-download/](http://gradle.org/gradle-download/)
  * Unpack, move to the desired location, and add gradle's bin directory to your system path
  * Ensure `gradle -v` works from the command-line, and shows version 2.12 or greater
  * Optionally, enable the [gradle daemon](https://docs.gradle.org/current/userguide/gradle_daemon.html) for faster builds
2. Ensure you have the Java 1.8 **JDK** installed. Also ensure Java's directory is on your path with `java -version`
  * Additionally, you will need to have the [`JAVA HOME`](https://docs.oracle.com/cd/E19182-01/821-0917/6nluh6gq9/index.html) variable set to your **JDK** installation in your system path
3. From your command line terminal, move to the `devenv` subdirectory of your workspace environment. Log into a Vagrant terminal by executing the following command:

    vagrant ssh

4. Build and run the peer process. To enable security and privacy after setting `security.enabled` and `security.privacy` settings to `true`.

    cd $GOPATH/src/github.com/hyperledger/fabric
    make peer
    peer node start

5. Change to Java shim root folder,

	cd $GOPATH/src/github.com/hyperledger/fabric/core/chaincode/shim/java

 and run 'gradle build'

6. The following steps is for deploying chaincode in non-dev mode.

	* Deploy the chaincode,
```
	peer chaincode deploy -l java -n map -p /opt/gopath/src/github.com/hyperledger/fabric/core/chaincode/shim/java -c '{"Function": "init", "Args": ["a","100", "b", "200"]}'
```

`6d9a704d95284593fe802a5de89f84e86fb975f00830bc6488713f9441b835cf32d9cd07b087b90e5cb57a88360f90a4de39521a5595545ad689cd64791679e9`

		* This command will give the 'name' for this chaincode, and use this value in all the further commands with the -n (name) parameter


		* PS. This may take a few minutes depending on the environment as it deploys the chaincode in the container,

* Invoke a transfer transaction,

```
	/opt/gopath/src/github.com/hyperledger/fabric/core/chaincode/shim/java$ peer chaincode invoke -l java \
	-n 6d9a704d95284593fe802a5de89f84e86fb975f00830bc6488713f9441b835cf32d9cd07b087b90e5cb57a88360f90a4de39521a5595545ad689cd64791679e9 \
	-c '{"Function": "transfer", "Args": ["a","b", "20"]}'
```
`c7dde1d7-fae5-4b68-9ab1-928d61d1e346`

* Query the values of a and b after the transfer

```
	/opt/gopath/src/github.com/hyperledger/fabric/core/chaincode/shim/java$ peer chaincode query -l java \
	-n 6d9a704d95284593fe802a5de89f84e86fb975f00830bc6488713f9441b835cf32d9cd07b087b90e5cb57a88360f90a4de39521a5595545ad689cd64791679e9 \
	-c '{"Function": "query", "Args": ["a"]}'
	{"Name":"a","Amount":"80"}


	/opt/gopath/src/github.com/hyperledger/fabric/core/chaincode/shim/java$ peer chaincode query -l java \
	-n 6d9a704d95284593fe802a5de89f84e86fb975f00830bc6488713f9441b835cf32d9cd07b087b90e5cb57a88360f90a4de39521a5595545ad689cd64791679e9 \
	-c '{"Function": "query", "Args": ["b"]}'
	{"Name":"b","Amount":"220"}
```

* To develop your own chaincodes, simply extend the Chaincode class (demonstrated in the SimpleSample Example under the examples package)


##### Note: The same steps can be used for deploying and testing the chaincode in development mode after starting the peer in dev mode using the command

    peer node start --peer-chaincodedev
