# Output executable name
EXECUTABLE=megacmd

# Override settings from user configuration (if available)
-include config.mk

build:
	go build -o $(EXECUTABLE)

clean:
	rm -f  $(EXECUTABLE)
	rm -rf tests/junk
	rm -f  tests/t.json

test: build
	./tests/run_tests.sh $(EXECUTABLE)

test_release:
	goreleaser --skip-publish --rm-dist --snapshot

release:
	goreleaser --rm-dist
