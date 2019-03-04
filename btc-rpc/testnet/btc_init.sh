#!/bin/bash

mkdir -p $HOME/.bitcoin1
mkdir -p $HOME/.bitcoin2

echo "Creating bitcoin.conf's"

    cat <<EOF > $HOME/.bitcoin1/bitcoin.conf
regtest=1
disablewallet=0
server=1
maxmempool=2000
onlynet=ipv4
txindex=1
printtoconsole=1
checkmempool=1
debug=1
[regtest]
rpcport=8332
rpcallowip=0.0.0.0/0
rpcuser=${RPCUSER:-bitcoinrpc}
rpcpassword=${RPCPASSWORD:-`dd if=/dev/urandom bs=33 count=1 2>/dev/null | base64`}
EOF

    cat <<EOF > $HOME/.bitcoin2/bitcoin.conf
regtest=1
disablewallet=0
server=1
listen=0
maxmempool=2000
printtoconsole=1
onlynet=ipv4
txindex=1
checkmempool=1
debug=1
[regtest]
connect=127.0.0.1:18444
rpcallowip=0.0.0.0/0
rpcport=8334
rpcuser=${RPCUSER:-bitcoinrpc}
rpcpassword=${RPCPASSWORD:-`dd if=/dev/urandom bs=33 count=1 2>/dev/null | base64`}
EOF

mkdir -p $HOME/.bitcoin2/regtest
cp -rf $HOME/wallet.dat $HOME/.bitcoin2/regtest

echo "Initialization completed successfully"

exec nohup bitcoind -datadir=$HOME/.bitcoin2 | sed  's/^/[bitcoin2] /' & > /dev/null \
    && bitcoind  -datadir=$HOME/.bitcoin1 | sed  's/^/[bitcoin1] /'

