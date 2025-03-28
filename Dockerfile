FROM kocubinski/cardano-node:10.1.4-4-gee2c96c32

RUN mkdir /app
COPY entrypoint.sh /app/entrypoint.sh
COPY scripts /app/scripts
WORKDIR /app
ENTRYPOINT ["/app/entrypoint.sh"]