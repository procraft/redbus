include deploy/deploy.mk

LOCAL_BIN:=$(CURDIR)/bin
BUILD_ENVPARMS:=CGO_ENABLED=0
UNAME_S:=$(shell uname -s)

GOFMT = gofmt

GOIMPORTS = $(LOCAL_BIN)/goimports
$(LOCAL_BIN)/goimports: REPOSITORY=golang.org/x/tools/cmd/goimports

PKGS=$(shell go list -f '{{.Dir}}' ./... | grep -v /vendor/ | grep -v '/api/')

$(LOCAL_BIN):
	@mkdir -p $@
$(LOCAL_BIN)/%:
	$Q tmp=$$(mktemp -d); \
		(GOPATH=$$tmp GO111MODULE=on go get $(REPOSITORY) && cp $$tmp/bin/* $(LOCAL_BIN)/.) || ret=$$?; \
		rm -rf $$tmp ; exit $$ret

fmt: $(LOCAL_BIN) | $(GOIMPORTS)
	$(GOFMT) -l -w $(PKGS)
	$(GOIMPORTS) -l -w -local 'github.com/prokraft/redbus' $(PKGS)

gen:
	GOBIN=$(LOCAL_BIN) go get github.com/gogo/protobuf/protoc-gen-gofast
	GOBIN=$(LOCAL_BIN) go install github.com/gogo/protobuf/protoc-gen-gofast
	protoc \
		--plugin=protoc-gen-gofast=$(LOCAL_BIN)/protoc-gen-gofast \
		--gofast_out=plugins=grpc:. \
		api/*.proto

build:
	$(BUILD_ENVPARMS) go build $(BUILD_ARGS) -ldflags="$(BUILD_LDFLAGS)" -o $(LOCAL_BIN)/databus ./cmd/databus/databus.go

build-example:
	$(BUILD_ENVPARMS) go build $(BUILD_ARGS) -ldflags="$(BUILD_LDFLAGS)" -o $(LOCAL_BIN)/consumer ./example/golang/consumer/consumer.go
	$(BUILD_ENVPARMS) go build $(BUILD_ARGS) -ldflags="$(BUILD_LDFLAGS)" -o $(LOCAL_BIN)/producer ./example/golang/producer/producer.go

build-all: build build-example

export-env:
	export $(shell sed 's/=.*//' .env)