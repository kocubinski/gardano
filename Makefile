TAG := kocubinski/cardano-devnet:0.1.4

devnet:
	./scripts/bootstrap-devnet.sh

docker: devnet
	docker build --tag $(TAG) .

push:
	docker push $(TAG)

docker-run: docker-env
	docker run -it  -p 7007:7007 -v $(PWD)/devnet:/devnet $(TAG)

run: devnet
	./devnet/run/all.sh

docker-env:
	@echo "export CARDANO_NODE_SOCKET_PATH=$(PWD)/devnet.sock" > devnet.env
	@echo "export CARDANO_NODE_NETWORK_ID=42" >> devnet.env

env:
	@echo "export CARDANO_NODE_SOCKET_PATH=$(PWD)/devnet/main.sock" > devnet.env
	@echo "export CARDANO_NODE_NETWORK_ID=42" >> devnet.env

socket:
	socat UNIX-LISTEN:devnet.sock,fork TCP:localhost:7007

clean:
	rm -rf devnet.env devnet.sock devnet/
