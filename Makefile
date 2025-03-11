.PHONY: build test unit-test integration-test clean

build:
	go build -v ./...

test: unit-test integration-test

unit-test:
	SKIP_INTEGRATION=1 go test -v -race ./...

integration-test:
	go test -v -run "Integration" ./...

coverage:
	SKIP_INTEGRATION=1 go test -race -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -html=coverage.out -o coverage.html
	go tool cover -func=coverage.out

clean:
	go clean
	rm -f coverage.out coverage.html

docker-cleanup:
	docker rm -f mysql_integration_test || true