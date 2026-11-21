run:
	go run ./cmd/check-uptime

build:
	go build -o ./bin/check-uptime ./cmd/check-uptime

format:
	go fmt ./...

# Execute the built binary
preview:
	./bin/check-uptime
