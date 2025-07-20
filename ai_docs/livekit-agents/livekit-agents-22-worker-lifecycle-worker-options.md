LiveKit Docs › Worker lifecycle › Worker options

---

# Worker options

> Learn about the options available for creating a worker.

## WorkerOptions parameters

The interface for creating a worker is through the `WorkerOptions` class. The following only includes some of the available parameters. For the complete list, see the [WorkerOptions reference](https://docs.livekit.io/reference/python/v1/livekit/agents/index.html.md#livekit.agents.WorkerOptions).

**Python**:

```python
opts = WorkerOptions(
    # entrypoint function is called when a job is assigned to this worker
    # this is the only required parameter to WorkerOptions
    # https://docs.livekit.io/agents/worker/job/#entrypoint
    entrypoint_fnc,
    # inspect the request and decide if the current worker should handle it.
    request_fnc,
    # a function to perform any necessary initialization in a new process.
    prewarm_fnc,
    # a function that reports the current system load, whether CPU or RAM, etc.
    load_fnc,
    # the maximum value of load_fnc, above which new processes will not spawn
    load_threshold,
    # whether the agent can subscribe to tracks, publish data, update metadata, etc.
    permissions,
    # amount of time to wait for existing jobs to finish when SIGTERM or SIGINT is received
    drain_timeout,
    # the type of worker to create, either JT_ROOM or JT_PUBLISHER
    worker_type=WorkerType.ROOM,
)

# start the worker
cli.run_app(opts)

```

> 🔥 **Caution**
> 
> For security purposes, set the LiveKit API key and secret as environment variables rather than as `WorkerOptions` parameters.

### Entrypoint

The `entrypoint_fnc` is the main function called for each new job, and is the heart of your agent app. To learn more, see the [entrypoint documentation](https://docs.livekit.io/agents/worker/job.md#entrypoint) in the job lifecycle article.

**Python**:

```python
async def entrypoint(ctx: JobContext):
    # connect to the room
    await ctx.connect()
    # handle the session
    ...

```

### Request handler

The `request_fnc` function is executed each time that the server has a job for the agent. The framework expects workers to explicitly accept or reject each job request. If you accept the request, your entrypoint function is called. If the request is rejected, it's sent to the next available worker.

By default, if left blank, the behavior is to auto-accept all requests dispatched to the worker.

**Python**:

```python
async def request_fnc(req: JobRequest):
    # accept the job request
    await req.accept(
        # the agent's name (Participant.name), defaults to ""
        name="agent",
        # the agent's identity (Participant.identity), defaults to "agent-<jobid>"
        identity="identity",
        # attributes to set on the agent participant upon join
        attributes={"myagent": "rocks"},
    )

    # or reject it
    # await req.reject()

opts = WorkerOptions(entrypoint_fnc=entrypoint, request_fnc=request_fnc)

```

### Prewarm function

For isolation and performance reasons, the framework runs each agent job in its own process. Agents often need access to model files that take time to load. To address this, you can use a `prewarm` function to warm up the process before assigning any jobs to it. You can control the number of processes to keep warm using the `num_idle_processes` parameter.

**Python**:

```python
def prewarm_fnc(proc: JobProcess):
    # load silero weights and store to process userdata
    proc.userdata["vad"] = silero.VAD.load()


async def entrypoint(ctx: JobContext):
    # access the loaded silero instance
    vad: silero.VAD = ctx.proc.userdata["vad"]

opts = WorkerOptions(entrypoint_fnc=entrypoint, prewarm_fnc=prewarm_fnc)

```

### Load function

`load_fnc` and `load_threshold` control when a worker should stop accepting new jobs.

- `load_fnc`: A function that returns the current load of the worker as a float between 0 and 1.0.
- `load_threshold`: The maximum load value at which the worker will still accept new jobs.

If load_fnc is not provided, it defaults to using the worker's average CPU utilization over a 5-second window.

The following example shows how to define a custom load function that limits the worker to 9 concurrent jobs, independent of CPU usage:

```python
from livekit.agents import Worker, WorkerOptions

def compute_load(worker: Worker) -> float:
    return min(len(worker.active_jobs) / 10, 1.0)

opts = WorkerOptions(
    load_fnc=compute_load,
    load_threshold=0.9,
)

```

### Drain timeout

Since agent sessions are stateful, they should not be terminated abruptly when the process is shutting down. The Agents framework supports graceful termination: when a `SIGTERM` or `SIGINT` is received, the worker enters a `draining` state. In this state, it stops accepting new jobs but allows existing ones to complete, up to a configured timeout.

The `drain_timeout` parameter sets the maximum time to wait for active jobs to finish. It defaults to 30 minutes.

### Permissions

By default, agents can both publish to and subscribe from the other participants in the same room. However, you can customize these permissions by setting the `permissions` parameter in `WorkerOptions`. To see the full list of parameters, see the [WorkerPermissions reference](https://docs.livekit.io/reference/python/v1/livekit/agents/index.html.md#livekit.agents.WorkerPermissions).

**Python**:

```python
opts = WorkerOptions(
    ...
    permissions=WorkerPermissions(
        can_publish=True,
        can_subscribe=True,
        can_publish_data=True,
        # when set to true, the agent won't be visible to others in the room.
        # when hidden, it will also not be able to publish tracks to the room as it won't be visible.
        hidden=False,
    ),
)

```

### Worker type

You can choose to start a new instance of the agent for each room or for each publisher in the room. This can be set when you register your worker:

**Python**:

```python
opts = WorkerOptions(
    ...
    # when omitted, the default is WorkerType.ROOM
    worker_type=WorkerType.ROOM,
)

```

---

**Node.js**:

```ts
const opts = new WorkerOptions({
  // when omitted, the default is JobType.JT_ROOM
  workerType: JobType.JT_ROOM,
});

```

The `WorkerType` enum has two options:

- `ROOM`: Create a new instance of the agent for each room.
- `PUBLISHER`: Create a new instance of the agent for each publisher in the room.

If the agent is performing resource-intensive operations in a room that could potentially include multiple publishers (for example, processing incoming video from a set of security cameras), you can set `worker_type` to `JT_PUBLISHER` to ensure that each publisher has its own instance of the agent.

For `PUBLISHER` jobs, call the `entrypoint` function once for each publisher in the room. The `JobContext.publisher` object contains a `RemoteParticipant` representing that publisher.

## Starting the worker

To spin up a worker with the configuration defined using `WorkerOptions`, call the CLI:

**Python**:

```python
if __name__ == "__main__":
    cli.run_app(opts)

```

---

**Node.js**:

```ts
cli.runApp(opts);

```

The Agents worker CLI provides two subcommands: `start` and `dev`. The former outputs raw JSON data to stdout, and is recommended for production. `dev` is recommended to use for development, as it outputs human-friendly colored logs, and supports hot reloading on Python.

---

This document was rendered at 2025-07-18T19:17:20.888Z.
For the latest version of this document, see [https://docs.livekit.io/agents/worker/options.md](https://docs.livekit.io/agents/worker/options.md).

To explore all LiveKit documentation, see [llms.txt](https://docs.livekit.io/llms.txt).