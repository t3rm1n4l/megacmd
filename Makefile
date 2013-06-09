build:
	go get github.com/t3rm1n4l/go-mega
	go get github.com/t3rm1n4l/megacmd/client
	go build

test:
	./tests/run_tests.sh

