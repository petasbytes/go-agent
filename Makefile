.PHONY: run test cover cover-html
run:
	go run ./cmd/agent

test:
	go test ./... -count=1

cover:
	go test ./... -count=1 cover | tail -n 1

cover-html:
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html
