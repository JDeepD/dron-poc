

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
	@echo "==> Done"
