#!/usr/bin/env bash

ROOT=devnet

echo "Genesis UTxO Address:"
cardano-cli address build --payment-verification-key-file ${ROOT}/utxo-keys/utxo1.vkey --testnet-magic 42
echo
echo

echo "Genesis UTxO Private Key:"
cat ${ROOT}/utxo-keys/utxo1.skey | jq -r '.cborHex'
echo

echo "Test Account Addresses:"
for i in $(seq 1 10); do
    echo -n "${i}) "
    cardano-cli address build --payment-verification-key-file ${ROOT}/accounts/addr${i}.vkey --testnet-magic 42
    echo
done