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
	git push --tags
	goreleaser --rm-dist

# Get the build dependencies
build_dep:
	go get -u github.com/kisielk/errcheck
	go get -u golang.org/x/tools/cmd/goimports
	go get -u github.com/golang/lint/golint

# Do source code quality checks
check:
	go vet
	errcheck
	goimports -d . | grep . ; test $$? -eq 1
	-#golint
