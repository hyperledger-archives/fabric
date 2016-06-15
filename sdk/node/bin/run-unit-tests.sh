#
# Run the unit tests associated with the node.js client sdk
#
main() {
   # Initialization
   init

   {
   # Start member services
   startMemberServices

   # Run tests in network mode
   export DEPLOY_MODE='net'
   runTests

   # Run tests in dev mode
   export DEPLOY_MODE='dev'
   runTests

   # Stop peer and member services
   stopPeer
   stopMemberServices

   } 2>&1 | tee $LOGDIR/log
}

# initialization & cleanup
init() {
   # Initialize variables
   FABRIC=$GOPATH/src/github.com/hyperledger/fabric
   LOGDIR=/tmp/node-sdk-unit-test
   MSEXE=$FABRIC/build/bin/membersrvc
   MSLOGFILE=$LOGDIR/membersrvc.log
   PEEREXE=$FABRIC/build/bin/peer
   PEERLOGFILE=$LOGDIR/peer.log
   UNITTEST=$FABRIC/sdk/node/test/unit
   EXAMPLES=$FABRIC/examples/chaincode/go

   # Clean up if anything remaining from previous run
   stopMemberServices
   stopPeer
   rm -rf /var/hyperledger/production /tmp/keyValStore $LOGDIR
   mkdir $LOGDIR
}

# Run tests
runTests() {
   echo "Begin running tests in $DEPLOY_MODE mode ..."
   # restart peer
   stopPeer
   startPeer
   # Run some tests in network mode
   runRegistrarTests
   runChainTests
   runAssetMgmtTests
   #runAssetMgmtWithRolesTests
   echo "End running tests in network mode"
}

startMemberServices() {
   startProcess $MSEXE $MSLOGFILE "member services"
}

stopMemberServices() {
   killProcess $MSEXE
}

startPeer() {
   if [ "$DEPLOY_MODE" = "net" ]; then
      startProcess "$PEEREXE node start" $PEERLOGFILE "peer"
   else
      startProcess "$PEEREXE node start --peer-chaincodedev" $PEERLOGFILE "peer"
   fi
}

stopPeer() {
   killProcess $PEEREXE
}

# $1 is the name of the example to prepare
preExample() {
  if [ "$DEPLOY_MODE" = "net" ]; then
    prepareExampleForDeployInNetworkMode $1
  else
    startExampleInDevMode $1 $2
  fi
}

# $1 is the name of the example to stop
postExample() {
  if [ "$DEPLOY_MODE" = "net" ]; then
    echo "finished $1"
  else
    stopExampleInDevMode $1
  fi
}

# $1 is name of example to prepare on disk
prepareExampleForDeployInNetworkMode() {
   SRCDIR=$EXAMPLES/$1
   if [ ! -d $SRC_DIR ]; then
      echo "FATAL ERROR: directory does not exist: $SRCDIR"
      exit 1;
   fi
   DSTDIR=$GOPATH/src/github.com/$1
   if [ -d $DSTDIR ]; then
      echo "$DSTDIR already exists"
      return
   fi
   mkdir $DSTDIR
   cd $DSTDIR
   cp $SRCDIR/${1}.go .
   mkdir -p vendor/github.com/hyperledger
   cd vendor/github.com/hyperledger
   echo "cloning github.com/hyperledger/fabric; please wait ..."
   git clone https://github.com/hyperledger/fabric > /dev/null
   cp -r fabric/vendor/github.com/op ..
   cd ../../..
   go build
}

# $1 is the name of the sample to start
startExampleInDevMode() {
   SRCDIR=$EXAMPLES/$1
   if [ ! -d $SRC_DIR ]; then
      echo "FATAL ERROR: directory does not exist: $SRCDIR"
      exit 1;
   fi
   EXE=$SRCDIR/$1
   if [ ! -f $EXE ]; then
      cd $SRCDIR
      go build
   fi
   export CORE_CHAINCODE_ID_NAME=$2
   export CORE_PEER_ADDRESS=0.0.0.0:30303
   startProcess "$EXE" "${EXE}.log" "$1"
}

# $1 is the name of the sample to start
stopExampleInDevMode() {
   killProcess $1
}

runRegistrarTests() {
   echo "BEGIN running registrar tests ..."
   node $UNITTEST/registrar.js
   echo "END running registrar tests"
}

runChainTests() {
   echo "BEGIN running chain-tests ..."
   preExample chaincode_example02 mycc1
   node $UNITTEST/chain-tests.js
   postExample chaincode_example02
   echo "END running chain-tests"
}

runAssetMgmtTests() {
   echo "BEGIN running asset-mgmt tests ..."
   preExample asset_management mycc2
   node $UNITTEST/asset-mgmt.js
   postExample asset_management
   echo "END running asset-mgmt tests"
}

runAssetMgmtWithRolesTests() {
   echo "BEGIN running asset management with roles tests ..."
   preExample asset_management_with_roles
   node $UNITTEST/asset-mgmt-with-roles.js
   postExample asset_management_with_roles
   echo "END running asset management with roles tests"
}

# start process
#   $1 is executable path with any args
#   $2 is the log file
#   $3 is string description of the process
startProcess() {
   $1 > $2 2>&1&
   PID=$!
   sleep 2
   kill -0 $PID > /dev/null 2>&1
   if [ $? -eq 0 ]; then
      echo "$3 is started"
   else
      echo "ERROR: $3 failed to start; see $2"
      exit 1
   fi
}

# kill a process
#   $1 is the executable name
killProcess() {
   PID=`ps -ef | grep "$1" | grep -v "grep" | awk '{print $2}'`
   if [ "$PID" != "" ]; then
      echo "killing PID $PID running $1 ..."
      kill -9 $PID
   fi
}

main
