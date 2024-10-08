.PHONY: binaries build run start stop pause unpause push pull dive clean tidy

NAME:=$(shell basename `pwd`)

REPO?=eu.gcr.io/uk-sre-tools-npe-65d5/user/
CTX?=pxt1
TAG?=$(shell git describe --tags --dirty --always)

# derivations
IMAGEBASE:=$(CTX)/$(NAME)

ECHO-SERVER:=cmd/echo-server/echo-server
ECHO-SERVER-DBG:=cmd/echo-server/echo-server-dbg

binaries: $(ECHO-SERVER) $(ECHO-SERVER-DBG)

no-cache: FLAGS += --no-cache

build no-cache:
	docker build $(FLAGS) -t $(IMAGEBASE) .

debug: 
	docker build --target $@ $(FLAGS) -t $(IMAGEBASE)-$@ .

tidy:
	@GOPROXY=https://nrm.us.equifax.com/repository/efxgo/ \
	GOSUMDB='sum.golang.org https://nrm.us.equifax.com/repository/golang-sum-proxy/' \
	go mod tidy

$(ECHO-SERVER):
	cd cmd/echo-server && CGO_ENABLED=0 go build -ldflags="-extldflags=-static"
	chmod 755 $@

$(ECHO-SERVER-DBG):
	cd cmd/echo-server && CGO_ENABLED=0 go build -o $(notdir $@) -ldflags="-extldflags=-static" -gcflags="all=-N -l" .
	chmod 755 $@

run:
	docker run -d \
-p 8080:8080 \
--name $(NAME) $(IMAGEBASE)

start stop pause unpause logs:
	docker $@ $(NAME)

rm: stop
	docker rm $(NAME)

push:
	-gcloud auth print-access-token | docker login -u oauth2accesstoken --password-stdin https://eu.gcr.io
	-docker tag $(IMAGEBASE) $(REPO)$(IMAGEBASE):$(TAG)
	docker push $(REPO)$(IMAGEBASE):$(TAG)

pull:
	-gcloud auth print-access-token | docker login -u oauth2accesstoken --password-stdin https://eu.gcr.io
	docker pull $(REPO)$(IMAGEBASE):$(TAG)
	docker tag $(REPO)$(IMAGEBASE):$(TAG) $(IMAGEBASE)

dive:
	dive $(REPO)$(IMAGEBASE):$(TAG)

clean:
	-rm $(ECHO-SERVER)
	-rm $(ECHO-SERVER-DBG)
	-docker rmi $(REPO)$(IMAGEBASE):$(TAG)
	docker rmi $(IMAGEBASE):$(TAG)

sh:
	docker exec -it $(NAME) /bin/sh

explore:
	docker run -it --rm  --entrypoint /bin/sh $(IMAGEBASE)

explore-debug:
	docker run -it --rm  --entrypoint /bin/sh $(IMAGEBASE)-debug

sha-id:
	@docker image inspect --format='{{.RepoDigests}}' $(REPO)$(IMAGEBASE):$(TAG) | awk -F[:\ ] '{print $$2}' | sed 's/]//'
	

help:
	@echo "Usage: make <target>"
	@echo
	@echo "Available targets are:"
	@echo " help                    show this text"
	@echo
	@echo " build                   create image"
	@echo " run                     run image"
	@echo " sh                      open sh shell in running image"

