#!/bin/bash

if [ ! -f devnet/main.sock ]; then
    echo "Socket not found. Generating devnet config..."
    bash scripts/bootstrap-devnet.sh
fi

bash scripts/devnet-info.sh
bash devnet/node-spo1.sh