#!/bin/bash
set -e

echo "--> Setting up Environment variables..."
export DOCKER_TLS_VERIFY=1
# Define the following
#export DOCKER_CERT_PATH=$PWD/dockercfg
#HOST=""
#NUM_VAL_PEERS=3
#CA_CERT=""
#KEY=""
#CERT=""

# Make Dockerfile from yaml file
# Reads the file line-by-line,
# Starts from peer->Dockerfile
# Ends at the blank line
DOCKER_HOST="tcp://$HOST:2376"

ParseConfigForDockerfile() {
	IFS=''
	spaces=0
	numSpacesDockerfile=0
	tag=''
	while read line
	do
		spaces=`echo "$line" | awk -F'[^ \t]' '{print length($1)}'`

		# If we are looking at Dockerfile inside peer, print it out
		if [[ $tag == 'Dockerfile' ]]
		then
			if [[ $line == *"COPY src $GOPATH/src"* ]]
			then
				echo "RUN mkdir -p $GOPATH/src/github.com/hyperledger/fabric"
				echo "COPY ./ $GOPATH/src/github.com/hyperledger/fabric/"
			else
				echo $line
			fi
		fi

		# If we number of spaces/tabs are less & we were looking at Dockerfile already,
		# we are done, exit
		if [[ "$spaces" -le "$numSpacesDockerfile" ]] && [[ $tag == "Dockerfile" ]]
		then
			tag=''
			break
		fi

		# Dockerfile lives under peer tag in yaml, get to "peer" first
		if [[ $line == "peer"* ]]
		then
			tag="peer"
		fi

		# If we are in peer tag, look for the "Dockerfile" tag
		if [[ $tag == 'peer' ]] && [[ $line == *"Dockerfile"* ]]
		then
			tag="Dockerfile"
			numSpacesDockerfile=$spaces
		fi

	done < $1
}

PrepareFiles() {
	mkdir -p docker_companion
#	echo -e $COMPANION_HOST
	echo -e $COMPANION_KEY > docker_companion/key.pem
	echo -e $COMPANION_CA_CERT > docker_companion/ca.pem
	echo -e $COMPANION_CERT > docker_companion/cert.pem
}

PrepareDocker() {
	echo "--> Preparing certs";
	mkdir -p $DOCKER_CERT_PATH;
	echo -e $KEY > $DOCKER_CERT_PATH/key.pem
	echo -e $CA_CERT > $DOCKER_CERT_PATH/ca.pem
	echo -e $CERT > $DOCKER_CERT_PATH/cert.pem
	echo `ls -l $DOCKER_CERT_PATH`

	echo "--> Dowloading libapparmor"
	wget http://security.ubuntu.com/ubuntu/pool/main/a/apparmor/libapparmor1_2.8.95~2430-0ubuntu5.3_amd64.deb -O libapparmor.deb
	echo "--> Installing libapparmor"
	sudo dpkg -i libapparmor.deb
	rm libapparmor.deb

	echo "--> Downloading docker binary"
	wget https://get.docker.com/builds/Linux/x86_64/docker-1.9.1 --quiet -O docker
	chmod +x docker
}

#
BuildDockerImage() {
	echo "--> Building Docker Image - HOST=$DOCKER_HOST"
	./docker -H $DOCKER_HOST build -t hyperledger/fabric-peer .
}

# Params - Docker host, PeerID, Host, Port
RestartAsRoot() {
	./docker -H $1 stop -t 0 $2 || true
	./docker -H $1 rm $2 || true
	./docker -H $1 run --name=$2 --restart=unless-stopped -d -it -p $5:5000 -p $4:30303 -e CORE_VM_ENDPOINT=$COMPANION_HOST -e CORE_PEER_ID=$2 -e CORE_PEER_ADDRESSAUTODETECT=false -e CORE_PEER_ADDRESS=$3:$4 -e CORE_PEER_LISTENADDRESS=0.0.0.0:30303 -e CORE_VM_DOCKER_TLS_ENABLED=true -e CORE_VM_DOCKER_TLS_CERT_FILE="/companion_certs/cert.pem" -e CORE_VM_DOCKER_TLS_CACERT_FILE="/companion_certs/cacert.pem" -e  CORE_VM_DOCKER_TLS_KEY_FILE="/companion_certs/key.pem" openchain-peer obc-peer peer
}

# Params - Docker host, PeerID, Host, Port, Root Node
RestartAsPeer() {
	./docker -H $1 stop -t 0 $2 || true
	./docker -H $1 rm $2 || true
	./docker -H $1 run --name=$2 --restart=unless-stopped -d -it -p $6:5000 -p $4:30303 -e CORE_VM_ENDPOINT=$COMPANION_HOST -e CORE_PEER_ID=$2 -e CORE_PEER_ADDRESSAUTODETECT=false -e CORE_PEER_DISCOVERY_ROOTNODE=$5 -e CORE_PEER_ADDRESS=$3:$4 -e CORE_PEER_LISTENADDRESS=0.0.0.0:30303  -e CORE_VM_DOCKER_TLS_ENABLED=true -e CORE_VM_DOCKER_TLS_CERT_FILE="/companion_certs/cert.pem" -e CORE_VM_DOCKER_TLS_CACERT_FILE="/companion_certs/cacert.pem" -e  CORE_VM_DOCKER_TLS_KEY_FILE="/companion_certs/key.pem" openchain-peer obc-peer peer
}

Run() {
	max_peer_id=`./docker -H $DOCKER_HOST ps | grep vp | awk 'NF>1{print $NF}' | sed 's/vp//' | awk '$0>x{x=$0};END{print x}'`

	if [ ! "" == "$max_peer_id" ] && [ $max_peer_id -gt 2 ]
	then
		echo "--> Redeploying the first peer"
		RestartAsPeer $DOCKER_HOST vp1 $HOST 30301 $HOST:3030$max_peer_id 5001
	else
		echo "--> Less than 2 peers active, redeploying the first peer as root"
		RestartAsRoot $DOCKER_HOST vp1 $HOST 30301 5001
	fi

	iter=2
	while [ $iter -lt `expr $NUM_VAL_PEERS + 1` ]
	do
		echo "--> Deploying peer $iter/$NUM_VAL_PEERS"
		RestartAsPeer $DOCKER_HOST vp$iter $HOST 3030$iter $HOST:30301 500$iter
		iter=`expr $iter + 1`;
	done
}

PrepareFiles
ParseConfigForDockerfile core.yaml > Dockerfile
PrepareDocker
BuildDockerImage
Run

rm docker
rm -rf $DOCKER_CERT_PATH
