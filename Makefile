

.PHONY: proto fmt clean build run-coordinator run-worker run-client

proto:
	@echo "==> Generating protobuf files"
	# protoc -I=. --go_out=./common ./common.proto
	protoc -I=common/protobuf --go_out=. --go_opt=paths=source_relative \
            --go-grpc_out=. --go-grpc_opt=paths=source_relative common/protobuf/dron_poc/*.proto
	@echo "==> Done"

fmt:
	@echo "==> Formatting code..."
	go fmt ./...
	@echo "==> Format complete!"

clean:
	@echo "==> Cleaning up protobufs"
	rm -rf ./common/dron-proto/*.pb.go
	rm -rf ./bin
	@echo "==> Done"

build:
	@echo "==> Building binaries..."
	@mkdir -p bin
	go build -o bin/coordinator ./coordinator
	go build -o bin/worker ./worker
	go build -o bin/client ./client
	@echo "==> Build complete!"

run-coordinator:
	@echo "==> Starting coordinator..."
	go run ./coordinator/main.go

run-worker:
	@echo "==> Starting worker..."
	@if [ -z "$(NAME)" ]; then \
		echo "Usage: make run-worker NAME=<name> ADDR=<address>"; \
		exit 1; \
	fi
	go run ./worker/main.go -name $(NAME) -address $(ADDR)

run-client:
	@echo "==> Running client..."
	@if [ -z "$(ACTION)" ] || [ -z "$(NAME)" ] || [ -z "$(COMMAND)" ]; then \
		echo "Usage: make run-client ACTION=<action> NAME=<name> COMMAND='<command>' [PRIORITY=<priority>]"; \
		echo "  PRIORITY: low, normal (default), high, critical"; \
		exit 1; \
	fi
	go run ./client/main.go $(ACTION) -name $(NAME) -command "$(COMMAND)" -priority $(or $(PRIORITY),normal)