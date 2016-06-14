#
# Run the unit tests associated with the node.js client sdk
#
main() {
   # Initialization
   init

   # Start member services, then run tests in network mode & dev mode
   {
   startMemberServices
   #runTestsInNetworkMode
   runTestsInDevMode
   #stopPeer
   #stopMemberServices

   } | tee $LOGDIR/log 2>&1
}

# initialization & cleanup
init() {
   # Initialize variables
   FABRIC=$GOPATH/src/github.com/hyperledger/fabric
   LOGDIR=/tmp/node-sdk-unit-test
   MSEXE=$FABRIC/membersrvc/membersrvc
   MSLOGFILE=$LOGDIR/membersrvc.log
   PEEREXE=$FABRIC/peer/peer
   PEERLOGFILE=$LOGDIR/peer.log
   UNITTEST=$FABRIC/sdk/node/test/unit
   EXAMPLES=$FABRIC/examples/chaincode/go

   # Clean up if anything remaining from previous run
   stopMemberServices
   stopPeer
   rm -rf /var/hyperledger/production /tmp/keyValStore $LOGDIR
   mkdir $LOGDIR
}

# Run tests in network mode
runTestsInNetworkMode() {
   echo "Begin running tests in network mode ..."
   # start/restart peer in network mode
   stopPeer
   startPeerInNetworkMode
   export DEPLOY_MODE='net'
   # Run some tests in network mode
   #runRegistrarTests
   #runChainTests
   runAssetMgmtTests
   echo "End running tests in network mode"
}

# Run tests in dev mode
runTestsInDevMode() {
   echo "Begin running tests in dev mode ..."
   # start/restart peer in dev mode
   stopPeer
   startPeerInDevMode
   export DEPLOY_MODE='dev'
   # Run some tests in dev mode
   # runAssetMgmtWithRolesTests
   runRegistrarTests
   runChainTests
   runAssetMgmtTests
   echo "End running tests in dev mode"
}

startMemberServices() {
   startProcess $MSEXE $MSLOGFILE "member services"
}

stopMemberServices() {
   killProcess $MSEXE
}

startPeerInNetworkMode() {
   startProcess "$PEEREXE node start" $PEERLOGFILE "peer"
}

startPeerInDevMode() {
   startProcess "$PEEREXE node start --peer-chaincodedev" $PEERLOGFILE "peer"
}

stopPeer() {
   killProcess $PEEREXE
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
   cd ../../..
   go get github.com/op/go-logging
   govendor add github.com/op/go-logging
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
   export CORE_CHAINCODE_ID_NAME=$1
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
   prepare chaincode_example02
   echo "BEGIN running chain-tests ..."
   node $UNITTEST/chain-tests.js
   echo "END running chain-tests"
}

runAssetMgmtTests() {
   prepare asset_management
   echo "BEGIN running asset-mgmt tests ..."
   node $UNITTEST/asset-mgmt.js
   echo "END running asset-mgmt tests"
}

runAssetMgmtWithRolesTests() {
   echo "BEGIN running asset management with roles tests ..."
   prepare asset_management_with_roles
   node $UNITTEST/asset-mgmt-with-roles.js
   stopExampleInDevMode asset_management_with_roles
   echo "END running asset management with roles tests"
}

prepare() {
  if [ "$DEPLOY_MODE" = "net" ]; then
    prepareExampleForDeployInNetworkMode $1
  else
    startExampleInDevMode $1
  fi
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
