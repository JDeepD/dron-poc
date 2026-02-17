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

Benchmarks ran on M1 Macbook Air (8/256 GB)
<img width="1485" height="1050" alt="workers-bench" src="https://github.com/user-attachments/assets/b2458dab-ecee-4dec-a9e3-b07747de20c6" />
<img width="1989" height="575" alt="task" src="https://github.com/user-attachments/assets/f895ebf3-536a-414a-97ab-9d9a854feaf3" />

