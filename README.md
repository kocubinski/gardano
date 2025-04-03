# gardano

Gardano is an implements a very small subset of the Cardano transaction protocol. Also included
in this project is a docker image that can be used to run a Cardano node. This image is based on
the official Cardano image.

See main.go for usage.

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
