# Run Coordinator

Binds to :8000
```sh
make run-coordinator
```

# Run Workers

Binds to whatever is given as ADDR param
```sh
make run-worker NAME=w1 ADDR=localhost:9001
```

# Run Tasks
Provide a name to the task and the command to run

```sh
make run-client ACTION=create NAME="my-task" COMMAND="echo hello" PRIORITY=critical
```

# Benchmarks

<img width="984" height="590" alt="e2e-latency" src="https://github.com/user-attachments/assets/4861f8d0-c5a5-48ca-8712-8147e6adba95" />
