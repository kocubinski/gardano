# gardano

Gardano implements a very small subset of Cardano's transaction protocol. Currently supported by the API are:

- Payment-only address construction for mainnet and testnets (see CIP-19)
- CIP-20 metadata
- Transaction CBOR encoding
- Transaction signing
- Change and fee calculation
- TTL

To test this library, a local Cardano node can be started locally if the cardano binaries are
installed with `make run`, or by the docker image produced with `make docker` if not.  The docker image is built from a fork of the official Cardno node with a few extra utilities.

See main.go for examples, which demonstrates the usage of github.com/blinklabs-io/gouroboros to
communicate with a running Cardano node via n2c for transaction submission, querying, and
synchronization.  

## Example

```bash
make clean
make run
export CARDANO_SIGNING_KEY_CBOR=(cat devnet/utxo-keys/utxo1.skey | jq -r '.cborHex')
go run . send-tx \
  -socket devnet/main.sock \
  -amount 3455819 \
  -receiver-address addr_test1vzt5qad02z7dlwa0h0gq92kx58s7uwunq9aqfzv6tvg2dvcdmrjm3 \
  --memo foo-bar
```
