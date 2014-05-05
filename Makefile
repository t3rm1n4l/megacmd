# Configure a local GOPATH if it's not exported by the user
export GOPATH?=$(CURDIR)/.gopath

build:
	go get github.com/t3rm1n4l/go-mega
	go get github.com/t3rm1n4l/megacmd/client
	go get github.com/t3rm1n4l/go-humanize
	go build

test:
	./tests/run_tests.sh

