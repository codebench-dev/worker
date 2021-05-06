# CodeBench worker

The worker handles lifecycle of an execution job.

- Get jobs from a Redis queues
- Fire up a Firecracker microVM for each job
- Run job trough the [agent](https://github.com/codebench-esgi/agent)
- Update back a Redis queue for each job during the process, with the following statuses:
  - `received`
  - `running`
  - `failed`
  - `done` + stdout/stderr

## Requirements

- The `firecracker` binary in the `PATH`
- A rootfs in `../agent/rootfs.ext4` with the [agent](https://github.com/codebench-esgi/agent) installed and enabled at boot
- A Linux kernel at `../../linux/vmlinux`.

Both the rootfs and the kernel can be built with scripts in the linked repo.

## Demo

Start Redis:

```sh
docker-compose up -d
```

Start the worker:

```
» go run
Waiting for jobs on redis job queue
```

Add a job to a Redis list (here, `uname -a`)

```
127.0.0.1:6378> RPUSH jobs '{"id":"666", "command": "uname -a"}'
(integer) 1
```

The worker gets the job, starts a goroutine to handle it:

```
INFO[0014] Handling job job="{666 uname -a}"
INFO[0014] Called startVMM(), setting up a VMM on /tmp/.firecracker.sock-2826742-81
INFO[0014] VMM logging disabled.
INFO[0014] VMM metrics disabled.
INFO[0014] refreshMachineConfiguration: [GET /machine-config][200] getMachineConfigurationOK &{CPUTemplate:Uninitialized HtEnabled:0xc00051e25b MemSizeMib:0xc00051e250 VcpuCount:0xc00051e248}
INFO[0014] PutGuestBootSource: [PUT /boot-source][204] putGuestBootSourceNoContent
INFO[0014] Attaching drive ../agent/rootfs.ext4, slot 1, root true.
INFO[0014] Attached drive ../agent/rootfs.ext4: [PUT /drives/{drive_id}][204] putGuestDriveByIdNoContent
INFO[0014] Attaching NIC tap0 (hwaddr 3e:78:01:d3:e9:03) at index 1
2021-05-06T00:17:45.019283004 [anonymous-instance:WARN:vmm/src/lib.rs:571] Could not add stdin event to epoll. Os { code: 1, kind: PermissionDenied, message: "Operation not permitted" }
INFO[0014] startInstance successful: [PUT /actions][204] createSyncActionNoContent
INFO[0014] machine started ip=192.168.127.83
INFO[0019] Job execution finished result="{uname -a Linux 192.168.127.83 5.9.0 #1 SMP Sun May 2 19:10:27 UTC 2021 x86_64 Linux\n}"
INFO[0019] stopping ip=192.168.127.83
WARN[0019] firecracker exited: signal: terminated
```

Done! The job has been handled, the microVM has been shut down and the job output has been published to redis. The output here is `Linux 192.168.127.83 5.9.0 #1 SMP Sun May 2 19:10:27 UTC 2021 x86_64 Linux`.

Here's what happening on Redis during that time:

```
127.0.0.1:6378> MONITOR
OK
1620260250.387595 [0 192.168.16.1:59124] "blpop" "jobs" "0"
1620260264.685236 [0 192.168.16.1:51044] "RPUSH" "jobs" "{\"id\":\"666\", \"command\": \"uname -a\"}"
1620260264.686254 [0 192.168.16.1:59124] "blpop" "jobs" "0"
1620260264.688423 [0 192.168.16.1:37144] "rpush" "666" "received"
1620260270.027252 [0 192.168.16.1:37144] "rpush" "666" "running"
1620260270.063586 [0 192.168.16.1:37144] "rpush" "666" "done" "Linux 192.168.127.83 5.9.0 #1 SMP Sun May 2 19:10:27 UTC 2021 x86_64 Linux\n" ""
```
