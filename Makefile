

.PHONY: proto fmt clean build run-coordinator run-worker run-client perf-test perf-stress benchmark benchmark-heavy benchmark-e2e benchmark-full kill-all

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

perf-test:
	@echo "==> Running ghz performance test..."
	ghz --insecure \
		--proto common/protobuf/dron_poc/coordinator.proto \
		--import-paths common/protobuf \
		--call dron_poc.CoordinatorService.CreateTask \
		-d '{"name":"perf-test","command":{"value":"echo hello"},"priority":"PRIORITY_HIGH"}' \
		-n $(or $(REQUESTS),10000) \
		-c $(or $(CONCURRENCY),100) \
		localhost:8000

perf-stress:
	@echo "==> Running stress test..."
	ghz --insecure \
		--proto common/protobuf/dron_poc/coordinator.proto \
		--import-paths common/protobuf \
		--call dron_poc.CoordinatorService.CreateTask \
		-d '{"name":"stress-test","command":{"value":"echo stress"},"priority":"PRIORITY_CRITICAL"}' \
		-n 50000 \
		-c 500 \
		localhost:8000

# Custom Go benchmark - measures CreateTask RPC throughput
benchmark:
	@echo "==> Running benchmark..."
	go run ./benchmark/main.go -tasks $(or $(TASKS),1000) -concurrency $(or $(CONCURRENCY),10)

# Heavy benchmark with more tasks
benchmark-heavy:
	@echo "==> Running heavy benchmark..."
	go run ./benchmark/main.go -tasks 10000 -concurrency 50

# End-to-end benchmark (requires workers to be running)
# Usage: Start coordinator, start workers, then run this
# Example: make benchmark-e2e TASKS=200 CONCURRENCY=20 TIMEOUT=120s
benchmark-e2e:
	@echo "==> Running end-to-end benchmark..."
	@echo "==> Make sure coordinator and workers are running!"
	go run ./benchmark/main.go -tasks $(or $(TASKS),100) -concurrency $(or $(CONCURRENCY),10) -timeout $(or $(TIMEOUT),60s) -wait

# Full automated E2E benchmark - starts coordinator, workers, runs benchmark, cleans up
# Usage: make benchmark-full WORKERS=3 TASKS=100 CONCURRENCY=10 TIMEOUT=60s
# WORKERS = number of worker processes to spawn (default: 3, max: 50)
# TASKS = number of tasks to create (default: 100)  
# CONCURRENCY = parallel RPC clients sending tasks (default: 10)
# TIMEOUT = max time to wait for completion (default: 60s)
benchmark-full:
	@echo "╔════════════════════════════════════════════════════════════╗"
	@echo "║           DRON Full E2E Benchmark                         ║"
	@echo "╚════════════════════════════════════════════════════════════╝"
	@echo ""
	@echo "Configuration:"
	@echo "  Workers:     $(or $(WORKERS),3)"
	@echo "  Tasks:       $(or $(TASKS),100)"
	@echo "  Concurrency: $(or $(CONCURRENCY),10)"
	@echo "  Timeout:     $(or $(TIMEOUT),60s)"
	@echo ""
	@mkdir -p bin
	@echo "==> Building binaries..."
	@go build -o bin/coordinator ./coordinator
	@go build -o bin/worker ./worker
	@go build -o bin/benchmark ./benchmark
	@echo "    Build complete!"
	@echo "==> Starting coordinator..."
	@bin/coordinator > /tmp/dron-coordinator.log 2>&1 & echo $$! > /tmp/dron-coordinator.pid
	@sleep 1
	@echo "    Coordinator started (PID: $$(cat /tmp/dron-coordinator.pid))"
	@echo "==> Starting $(or $(WORKERS),3) workers..."
	@for i in $$(seq 1 $(or $(WORKERS),3)); do \
		bin/worker -name worker-$$i -address 127.0.0.1:900$$i > /tmp/dron-worker-$$i.log 2>&1 & echo $$! >> /tmp/dron-workers.pid; \
		echo "    Started worker-$$i"; \
	done
	@sleep 2
	@echo "==> Running benchmark..."
	@echo ""
	@bin/benchmark -tasks $(or $(TASKS),100) -concurrency $(or $(CONCURRENCY),10) -timeout $(or $(TIMEOUT),60s) -wait; \
	EXIT_CODE=$$?; \
	echo ""; \
	echo "==> Cleaning up..."; \
	if [ -f /tmp/dron-coordinator.pid ]; then kill $$(cat /tmp/dron-coordinator.pid) 2>/dev/null || true; rm -f /tmp/dron-coordinator.pid; fi; \
	if [ -f /tmp/dron-workers.pid ]; then while read pid; do kill $$pid 2>/dev/null || true; done < /tmp/dron-workers.pid; rm -f /tmp/dron-workers.pid; fi; \
	rm -f /tmp/dron-*.log 2>/dev/null || true; \
	echo "==> Done!"; \
	exit $$EXIT_CODE

# Kill any running DRON processes
kill-all:
	@echo "==> Killing all DRON processes..."
	@-pkill -f "bin/coordinator" 2>/dev/null || true
	@-pkill -f "bin/worker" 2>/dev/null || true
	@-pkill -f "dron-poc/coordinator" 2>/dev/null || true
	@-pkill -f "dron-poc/worker" 2>/dev/null || true
	@-rm -f /tmp/dron-*.pid /tmp/dron-*.log 2>/dev/null || true
	@echo "==> Done!"