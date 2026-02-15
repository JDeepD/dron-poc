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
make run-client CMD='create -name t1 -command "echo hello"'
```

