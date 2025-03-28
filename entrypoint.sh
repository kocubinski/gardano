#!/bin/bash

set -o errexit 

FUND_AMOUNT=${FUND_AMOUNT:-100000000000}

if [ ! -f devnet/node-spo1.sh ]; then
    echo "Socket not found. Generating devnet config..."
    bash scripts/bootstrap-devnet.sh
fi

bash devnet/node-spo1.sh &

export CARDANO_NODE_SOCKET_PATH=devnet/node-spo1/node.sock
export CARDANO_NODE_NETWORK_ID=42

while true; do
    if [ -S "$CARDANO_NODE_SOCKET_PATH" ]; then
        break
    fi
    echo "Waiting for node to start..."
    sleep 1
done

if [[ -z "${FUND_ACCOUNT}" ]]; then
    echo "FUND_ACCOUNT not set. Skipping..."
else
    echo "Funding account ${FUND_ACCOUNT}..."
    bash scripts/fund-account.sh ${FUND_ACCOUNT} ${FUND_AMOUNT}
fi

socat TCP-LISTEN:7007,reuseaddr,fork UNIX-CLIENT:devnet/node-spo1/node.sock &

wait