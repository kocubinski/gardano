#!/usr/bin/env bash

set -o errexit -o nounset -o pipefail

GENESIS_AMOUNT=1800000000000
# use a static fee for simplicity
FEE=170000

if [ -z "$2" ]; then
    echo "Usage: <receiver-address> <amount>"
    exit 1
fi

export CARDANO_NODE_SOCKET_PATH CARDANO_NODE_NETWORK_ID

ADDR=$(cardano-cli address build --payment-verification-key-file devnet/utxo-keys/utxo1.vkey)

cardano-cli query protocol-parameters > pparams.json

# there should be one tx with 1800000000000 lovelace
RES=$(cardano-cli query utxo --address "$ADDR")
if [ $(echo "$RES" | wc -l) -ne 3 ]; then
    echo "Genesis UTxO not found"
    exit 1
fi
TX=$(echo "$RES" | tail -n 1)
tx_hash=$(echo "$TX" | awk '{print $1}')
tx_idx=$(echo "$TX" | awk '{print $2}')

cardano-cli conway transaction build \
    --tx-in "$tx_hash#$tx_idx" \
    --tx-out "$1+$2" \
    --change-address "$ADDR" \
    --out-file tx.raw

cardano-cli conway transaction sign \
    --tx-body-file tx.raw \
    --signing-key-file devnet/utxo-keys/utxo1.skey \
    --out-file tx.signed

cardano-cli conway transaction submit --tx-file tx.signed

echo "Funded $1 with $2 lovelace from $ADDR"