TAG := kocubinski/cardano-devnet:0.1.7

devnet:
	./scripts/bootstrap-devnet.sh

docker: 
	docker build --tag $(TAG) .

push:
	docker push $(TAG)

#	-v $(PWD)/devnet:/app/devnet \

docker-run: docker-env
	docker run -it \
	-p 7007:7007 \
	-e FUND_ACCOUNT=addr_test1vr8aq48kt8t8xxkecd6fuvj5zmx8ufaer9eqpt0pz8k9k4cntrghw \
	-e FUND_AMOUNT=1500000000000 \
	$(TAG)

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
