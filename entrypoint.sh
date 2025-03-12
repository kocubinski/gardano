#!/bin/bash

if [ ! -f devnet/node-spo1.sh ]; then
    echo "Socket not found. Generating devnet config..."
    bash scripts/bootstrap-devnet.sh
fi

# bash scripts/devnet-info.sh
bash devnet/node-spo1.sh &
socat TCP-LISTEN:7007,reuseaddr,fork UNIX-CLIENT:devnet/node-spo1/node.sock &

wait