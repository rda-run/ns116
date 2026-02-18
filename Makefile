APP_NAME := ns116
VERSION  := $(shell cat .version)

.PHONY: build run clean docker rpm

build:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -buildvcs=false -ldflags "-s -w -extldflags=-static -X main.version=$(VERSION)" -o $(APP_NAME) .

run:
	go run . --config config.yaml

clean:
	rm -f $(APP_NAME)

docker:
	docker build -t $(APP_NAME):$(VERSION) .

rpm: clean
	mkdir -p bin
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -buildvcs=false -ldflags "-s -w -extldflags=-static -X main.version=$(VERSION)" -o bin/$(APP_NAME) .
	VERSION=$(VERSION) nfpm package --packager rpm
	rm -rf bin
