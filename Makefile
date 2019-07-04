PROJECT_NAME := k8s-ec2-srcdst
GOFILES:=$(shell find . -name '*.go' | grep -v -E '(./vendor)')
VERSION?=$(shell git describe --tags --dirty)
IMAGE_TAG:=ottoyiu/${PROJECT_NAME}:${VERSION}

all: clean check bin image

image:
	docker build -t ${IMAGE_TAG} .

push_image:
	docker push ${IMAGE_TAG}

bin: bin/linux/${PROJECT_NAME}

bin/%: LDFLAGS=-X github.com/ottoyiu/${PROJECT_NAME}.Version=${VERSION}
bin/%: $(GOFILES)
	mkdir -p $(dir $@)
	CGO_ENABLED=0 GOOS=$(word 1, $(subst /, ,$*)) GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o "$@" github.com/ottoyiu/k8s-ec2-srcdst/cmd/k8s-ec2-srcdst/...

gofmt:
	gofmt -w -s pkg/
	gofmt -w -s cmd/

test:
	go test github.com/ottoyiu/${PROJECT_NAME}/pkg/... -args -v=1 -logtostderr
	go test github.com/ottoyiu/${PROJECT_NAME}/cmd/... -args -v=1 -logtostderr

check:
	@find . -name vendor -prune -o -name '*.go' -exec gofmt -s -d {} +
	@go vet $(shell go list ./... | grep -v '/vendor/')
	@go test -v $(shell go list ./... | grep -v '/vendor/')

clean:
		rm -rf bin

.PHONY: all bin check test 
