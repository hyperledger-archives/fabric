from openblockchain/baseimage:latest

# Copy GOPATH src and install Peer
RUN mkdir -p /var/openchain/db
RUN mkdir -p /var/openchain/production
WORKDIR $GOPATH/src/github.com/openblockchain/obc-peer/
COPY . .
RUN CGO_LDFLAGS="-lrocksdb -lstdc++ -lm -lz -lbz2 -lsnappy" go install && cp $GOPATH/src/github.com/openblockchain/obc-peer/openchain.yaml $GOPATH/bin
RUN cp $GOPATH/src/github.com/openblockchain/obc-peer/openchain/consensus/obcpbft/config.yaml $GOPATH/bin
# RUN CGO_LDFLAGS="-lrocksdb -lstdc++ -lm -lz -lbz2 -lsnappy" go install
# RUN cd obc-ca && go install
