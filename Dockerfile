FROM kocubinski/cardano-node:10.1.4-2-gb329f56dc

RUN mkdir /app
COPY entrypoint.sh /app/entrypoint.sh
COPY scripts /app/scripts
WORKDIR /app
ENTRYPOINT ["/app/entrypoint.sh"]