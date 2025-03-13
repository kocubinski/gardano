FROM kocubinski/cardano-node:10.1.4-3-gbd245026a

RUN mkdir /app
COPY entrypoint.sh /app/entrypoint.sh
COPY scripts /app/scripts
WORKDIR /app
ENTRYPOINT ["/app/entrypoint.sh"]