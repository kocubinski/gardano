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

echo
echo "Psuedo vault address derived from thorchain validator key dog dog ... dog fossil:"
gardano key-pair -magic 42 -seed b7e03eae2d19ae7250958295e69b11c5e56c5864ea9a1581a50683664ca6dde3