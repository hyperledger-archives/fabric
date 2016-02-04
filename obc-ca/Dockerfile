from golang:1.5.1
ENV GO15VENDOREXPERIMENT=1
# Install RocksDB
RUN cd /opt  && git clone --branch v4.1 --single-branch --depth 1 https://github.com/facebook/rocksdb.git && cd rocksdb && make shared_lib
ENV LD_LIBRARY_PATH=/opt/rocksdb:$LD_LIBRARY_PATH
RUN apt-get update && apt-get install -y libsnappy-dev zlib1g-dev libbz2-dev
# Copy GOPATH src and install Peer
RUN mkdir -p /var/openchain/db
WORKDIR $GOPATH/src/github.com/openblockchain/obc-peer/
COPY . .
WORKDIR obc-ca
RUN pwd
RUN CGO_CFLAGS="-I/opt/rocksdb/include" CGO_LDFLAGS="-L/opt/rocksdb -lrocksdb -lstdc++ -lm -lz -lbz2 -lsnappy" go install && cp $GOPATH/src/github.com/openblockchain/obc-peer/obc-ca/obcca.yaml $GOPATH/bin
# RUN CGO_CFLAGS="-I/opt/rocksdb/include" CGO_LDFLAGS="-L/opt/rocksdb -lrocksdb -lstdc++ -lm -lz -lbz2 -lsnappy" go install
