from openblockchain/baseimage:latest
# Copy GOPATH src and install Peer
RUN mkdir -p /var/openchain/db
WORKDIR $GOPATH/src/github.com/openblockchain/obc-peer/
COPY . .
WORKDIR obc-ca
RUN pwd
RUN CGO_LDFLAGS="-lrocksdb -lstdc++ -lm -lz -lbz2 -lsnappy" go install && cp $GOPATH/src/github.com/openblockchain/obc-peer/obc-ca/obcca.yaml $GOPATH/bin
# RUN CGO_LDFLAGS="-lrocksdb -lstdc++ -lm -lz -lbz2 -lsnappy" go install
