#!/usr/bin/env bash

export CARDANO_NODE_SOCKET_PATH=devnet/main.sock
export CARDANO_NODE_NETWORK_ID=42

if [ -z "$1" ]; then
    echo "Usage: $0 <receiver-address>"
    exit 1
fi

if [ ! -f pparams.json ]; then
    cardano-cli query protocol-parameters > pparams.json
fi

gardano send-tx \
    -socket devnet/main.sock \
    -protocol-parameters-file ./pparams.json \
    -amount 1000000 -fee 170000 \
    -magic 42 \
    -receiver-address $1 \
    -memo "foo+bar-baz"