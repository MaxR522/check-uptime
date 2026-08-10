run:
	go run ./cmd

build:
	go build -o ./bin/cmd ./cmd

format:
	go fmt ./...

# Execute the built binary
preview:
	./bin/cmd