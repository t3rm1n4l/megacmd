# Output executable name
EXECUTABLE=megacmd

# Configure a local GOPATH if it's not exported by the user
LOCAL_GOPATH=$(CURDIR)/.gopath

# Override settings from user configuration (if available)
-include config.mk

# Export GOPATH if not found
export GOPATH?=$(LOCAL_GOPATH)

build:
	go get github.com/t3rm1n4l/go-mega
	go get github.com/t3rm1n4l/megacmd/client
	go get github.com/t3rm1n4l/go-humanize
	go build -o $(EXECUTABLE)

clean:
	rm -rf $(LOCAL_GOPATH)
	rm -f  $(EXECUTABLE)
	rm -rf tests/junk
	rm -f  tests/t.json

test: build
	./tests/run_tests.sh $(EXECUTABLE)

test_release:
	goreleaser --skip-publish --rm-dist --snapshot

release:
	goreleaser --rm-dist

