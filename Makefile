.PHONY: run build run-bin test cover cover-html clean
run:
	mkdir -p sandbox
	cd sandbox && go run ../cmd/agent

build:
	go build -o bin/agent ./cmd/agent

run-bin: build
	./bin/agent

test:
	go test ./... -count=1

cover:
	go test ./... -count=1 -cover | tail -n 1

cover-html:
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html

clean:
	rm -f bin/agent coverage.out coverage.html
