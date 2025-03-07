#!/usr/bin/env bash

set -e
set -u
set -o pipefail

UNAME=$(uname -s) SED=
case $UNAME in
  Darwin )      SED="gsed";;
  Linux )       SED="sed";;
esac

UNAME=$(uname -s) DATE=
case $UNAME in
  Darwin )      DATE="gdate";;
  Linux )       DATE="date";;
esac

CARDANO_CLI="${CARDANO_CLI:-cardano-cli}"
NETWORK_MAGIC=42
SECURITY_PARAM=10
INIT_SUPPLY=12000000
START_TIME="$(${DATE} -d "now + 30 seconds" +%s)"
ROOT=devnet
mkdir -p "${ROOT}"

cat > "${ROOT}/byron.genesis.spec.json" <<EOF
{
  "heavyDelThd":     "300000000000",
  "maxBlockSize":    "2000000",
  "maxTxSize":       "4096",
  "maxHeaderSize":   "2000000",
  "maxProposalSize": "700",
  "mpcThd": "20000000000000",
  "scriptVersion": 0,
  "slotDuration": "1000",
  "softforkRule": {
    "initThd": "900000000000000",
    "minThd": "600000000000000",
    "thdDecrement": "50000000000000"
  },
  "txFeePolicy": {
    "multiplier": "43946000000",
    "summand": "155381000000000"
  },
  "unlockStakeEpoch": "18446744073709551615",
  "updateImplicit": "10000",
  "updateProposalThd": "100000000000000",
  "updateVoteThd": "1000000000000"
}
EOF

$CARDANO_CLI byron genesis genesis \
  --protocol-magic ${NETWORK_MAGIC} \
  --start-time "${START_TIME}" \
  --k ${SECURITY_PARAM} \
  --n-poor-addresses 0 \
  --n-delegate-addresses 1 \
  --total-balance ${INIT_SUPPLY} \
  --delegate-share 1 \
  --avvm-entry-count 0 \
  --avvm-entry-balance 0 \
  --protocol-parameters-file "${ROOT}/byron.genesis.spec.json" \
  --genesis-output-dir "${ROOT}/byron-gen-command"

cp scripts/alonzo-babbage-test-genesis.json "${ROOT}/genesis.alonzo.spec.json"
cp scripts/conway-babbage-test-genesis.json "${ROOT}/genesis.conway.spec.json"

cp scripts/configuration.yaml "${ROOT}/"
$SED -i "${ROOT}/configuration.yaml" \
     -e 's/Protocol: RealPBFT/Protocol: Cardano/' \
     -e '/Protocol/ aPBftSignatureThreshold: 0.6' \
     -e 's/minSeverity: Info/minSeverity: Debug/' \
     -e 's|GenesisFile: genesis.json|ByronGenesisFile: genesis/byron/genesis.json|' \
     -e '/ByronGenesisFile/ aShelleyGenesisFile: genesis/shelley/genesis.json' \
     -e '/ByronGenesisFile/ aAlonzoGenesisFile: genesis/shelley/genesis.alonzo.json' \
     -e '/ByronGenesisFile/ aConwayGenesisFile: genesis/shelley/genesis.conway.json' \
     -e 's/RequiresNoMagic/RequiresMagic/' \
     -e 's/LastKnownBlockVersion-Major: 0/LastKnownBlockVersion-Major: 6/' \
     -e 's/LastKnownBlockVersion-Minor: 2/LastKnownBlockVersion-Minor: 0/'

  echo "TestShelleyHardForkAtEpoch: 0" >> "${ROOT}/configuration.yaml"
  echo "TestAllegraHardForkAtEpoch: 0" >> "${ROOT}/configuration.yaml"
  echo "TestMaryHardForkAtEpoch: 0" >> "${ROOT}/configuration.yaml"
  echo "TestAlonzoHardForkAtEpoch: 0" >> "${ROOT}/configuration.yaml"
  echo "TestBabbageHardForkAtEpoch: 0" >> "${ROOT}/configuration.yaml"
  echo "TestConwayHardForkAtEpoch: 0" >> "${ROOT}/configuration.yaml"
  echo "ExperimentalProtocolsEnabled: True" >> "${ROOT}/configuration.yaml"

# Because in Babbage the overlay schedule and decentralization parameter
# are deprecated, we must use the "create-staked" cli command to create
# SPOs in the ShelleyGenesis
$CARDANO_CLI legacy genesis create-staked --genesis-dir "${ROOT}" \
  --testnet-magic "${NETWORK_MAGIC}" \
  --gen-pools 1 \
  --supply            2000000000000 \
  --supply-delegated   240000000002 \
  --gen-stake-delegs 1 \
  --gen-utxo-keys 1


NODE="node-spo1"
mkdir -p "${ROOT}/${NODE}"

# Move all genesis related files
mkdir -p "${ROOT}/genesis/byron"
mkdir -p "${ROOT}/genesis/shelley"

mv "${ROOT}/byron-gen-command/genesis.json" "${ROOT}/genesis/byron/genesis-wrong.json"
mv "${ROOT}/genesis.alonzo.json" "${ROOT}/genesis/shelley/genesis.alonzo.json"
mv "${ROOT}/genesis.conway.json" "${ROOT}/genesis/shelley/genesis.conway.json"
mv "${ROOT}/genesis.json" "${ROOT}/genesis/shelley/genesis.json"

jq --raw-output '.protocolConsts.protocolMagic = 42' "${ROOT}/genesis/byron/genesis-wrong.json" > "${ROOT}/genesis/byron/genesis.json"

rm "${ROOT}/genesis/byron/genesis-wrong.json"

cp "${ROOT}/genesis/shelley/genesis.json" "${ROOT}/genesis/shelley/copy-genesis.json"

jq -M '. + {slotLength:0.1, securityParam:10, activeSlotsCoeff:0.1, securityParam:10, epochLength:500, maxLovelaceSupply:10000000000000, updateQuorum:2}' "${ROOT}/genesis/shelley/copy-genesis.json" > "${ROOT}/genesis/shelley/copy2-genesis.json"
jq --raw-output '.protocolParams.protocolVersion.major = 7 | .protocolParams.minFeeA = 44 | .protocolParams.minFeeB = 155381 | .protocolParams.minUTxOValue = 1000000 | .protocolParams.decentralisationParam = 0.7 | .protocolParams.rho = 0.1 | .protocolParams.tau = 0.1' "${ROOT}/genesis/shelley/copy2-genesis.json" > "${ROOT}/genesis/shelley/genesis.json"

rm "${ROOT}/genesis/shelley/copy2-genesis.json"
rm "${ROOT}/genesis/shelley/copy-genesis.json"

mv "${ROOT}/pools/vrf1.skey" "${ROOT}/${NODE}/vrf.skey"

mv "${ROOT}/pools/opcert1.cert" "${ROOT}/${NODE}/opcert.cert"

mv "${ROOT}/pools/kes1.skey" "${ROOT}/${NODE}/kes.skey"

#Byron related

mv "${ROOT}/byron-gen-command/delegate-keys.000.key" "${ROOT}/${NODE}/byron-delegate.key"
mv "${ROOT}/byron-gen-command/delegation-cert.000.json" "${ROOT}/${NODE}/byron-delegation.cert"

echo 3001 > "${ROOT}/${NODE}/port"

# Make topology files
cat > "${ROOT}/${NODE}/topology.json" <<EOF
{
   "Producers": [
     {
       "addr": "127.0.0.1",
       "port": 3001,
       "valency": 1
     }
   ]
 }
EOF

# Generate Test Accounts
mkdir -p "${ROOT}/accounts"
for i in $(seq 1 10); do
    $CARDANO_CLI address key-gen \
    --verification-key-file "${ROOT}/accounts/addr${i}.vkey" \
    --signing-key-file "${ROOT}/accounts/addr${i}.skey"
done

SPO_NODES=${NODE}

for NODE in ${SPO_NODES}; do
  RUN_FILE="${ROOT}/${NODE}.sh"
  cat << EOF > "${RUN_FILE}"
#!/usr/bin/env bash

CARDANO_NODE="\${CARDANO_NODE:-cardano-node}"

\$CARDANO_NODE run \\
  --config                          '${ROOT}/configuration.yaml' \\
  --topology                        '${ROOT}/${NODE}/topology.json' \\
  --database-path                   '${ROOT}/${NODE}/db' \\
  --socket-path                     '${ROOT}/${NODE}/node.sock' \\
  --shelley-kes-key                 '${ROOT}/${NODE}/kes.skey' \\
  --shelley-vrf-key                 '${ROOT}/${NODE}/vrf.skey' \\
  --byron-delegation-certificate    '${ROOT}/${NODE}/byron-delegation.cert' \\
  --byron-signing-key               '${ROOT}/${NODE}/byron-delegate.key' \\
  --shelley-operational-certificate '${ROOT}/${NODE}/opcert.cert' \\
  --port                            $(cat "${ROOT}/${NODE}/port") \\
  | tee -a '${ROOT}/${NODE}/node.log'
EOF

  chmod a+x "${RUN_FILE}"

  echo "${RUN_FILE}"
done

mkdir -p "${ROOT}/run"

echo "#!/usr/bin/env bash" > "${ROOT}/run/all.sh"
echo "" >> "${ROOT}/run/all.sh"

for NODE in ${SPO_NODES}; do
  echo "$ROOT/${NODE}.sh &" >> "${ROOT}/run/all.sh"
done
echo "" >> "${ROOT}/run/all.sh"
echo "wait" >> "${ROOT}/run/all.sh"

chmod a+x "${ROOT}/run/all.sh"

echo "CARDANO_NODE_SOCKET_PATH=${ROOT}/${NODE}/node.sock "

(cd "$ROOT"; ln -s node-spo1/node.sock main.sock)

