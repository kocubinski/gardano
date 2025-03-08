FROM ghcr.io/intersectmbo/cardano-node:10.1.4

COPY devnet /devnet
WORKDIR /
ENTRYPOINT ["/devnet/run/all.sh"]