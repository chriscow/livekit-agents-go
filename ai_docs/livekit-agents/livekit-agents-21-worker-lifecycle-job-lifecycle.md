LiveKit Docs › Worker lifecycle › Job lifecycle

---

# Job lifecycle

> Learn more about the entrypoint function and how to end and clean up LiveKit sessions.

## Lifecycle

When a [worker](https://docs.livekit.io/agents/worker.md) accepts a job request from LiveKit server, it starts a new process and runs your agent code inside. Each job runs in a separate process to isolate agents from each other. If a session instance crashes, it doesn't affect other agents running on the same worker. The job runs until all standard and SIP participants leave the room, or you explicitly shut it down.

## Entrypoint

The `entrypoint` is executed as the main function of the process for each new job run by the worker, effectively handing control over to your code. You should load any necessary app-specific data, call `ctx.connect()` to join the room, and then execute your agent's logic.

This example shows a simple entrypoint that processes incoming audio tracks and publishes a text message to the room.

**Python**:

```python
async def do_something(track: rtc.RemoteAudioTrack):
    audio_stream = rtc.AudioStream(track)
    async for event in audio_stream:
        # Do something here to process event.frame
        pass
    await audio_stream.aclose()

async def entrypoint(ctx: JobContext):
    # an rtc.Room instance from the LiveKit Python SDK
    room = ctx.room

    # set up listeners on the room before connecting
    @room.on("track_subscribed")
    def on_track_subscribed(track: rtc.Track, *_):
        if track.kind == rtc.TrackKind.KIND_AUDIO:
            asyncio.create_task(do_something(track))

    # connect to room
    await ctx.connect(auto_subscribe=AutoSubscribe.AUDIO_ONLY)

    # when connected, room.local_participant represents the agent
    await room.local_participant.send_text('hello world', topic='hello-world')
)
    # iterate through currently connected remote participants
    for rp in room.remote_participants.values():
        print(rp.identity)

```

For more LiveKit Agents examples, see the [GitHub repository](https://github.com/livekit/agents/tree/main/examples). To learn more about publishing and receiving tracks, see the following topics:

- **[Media tracks](https://docs.livekit.io/home/client/tracks.md)**: Use the microphone, speaker, cameras, and screenshare with your agent.

- **[Realtime text and data](https://docs.livekit.io/home/client/data.md)**: Use text and data channels to communicate with your agent.

## Adding custom fields to agent logs

Each job outputs JSON-formatted logs that include the user transcript, turn detection data, job ID, process ID, and more. You can include custom fields in the logs using `ctx.log_fields_context` for additional diagnostic context.

The following example adds worker ID and room name to the logs:

```python
async def entrypoint(ctx: JobContext):
    ctx.log_context_fields = {
      "worker_id": ctx.worker_id,
      "room_name": ctx.room.name,
    }

```

To learn more, see the reference documentation for [JobContext.log_context_fields](https://docs.livekit.io/reference/python/v1/livekit/agents/index.html.md#livekit.agents.JobContext.log_context_fields).

## Passing data to a job

You can customize a job with user or job-specific data using either job metadata, room metadata, or participant attributes.

### Job metadata

Job metadata is a freeform string field defined in the [dispatch request](https://docs.livekit.io/agents/worker/agent-dispatch.md#via-api) and consumed in the `entrypoint`. Use JSON or similar structured data to pass complex information.

For instance, you can pass the user's ID, name, and phone number:

```python
import json

async def entrypoint(ctx: JobContext):
    metadata = json.loads(ctx.job.metadata)
    user_id = metadata["user_id"]
    user_name = metadata["user_name"]
    user_phone = metadata["user_phone"]
    # ...

```

For more information on dispatch, see the following article:

- **[Agent dispatch](https://docs.livekit.io/agents/worker/agent-dispatch.md#via-api)**: Learn how to dispatch an agent with custom metadata.

### Room metadata and participant attributes

You can also use properties such as the room's name, metadata, and participant attributes to customize agent behavior.

Here's an example showing how to access various properties:

```python
async def entrypoint(ctx: JobContext):
  # connect to the room
  await ctx.connect(auto_subscribe=AutoSubscribe.AUDIO_ONLY)

  # wait for the first participant to arrive
  participant = await ctx.wait_for_participant()

  # customize behavior based on the participant
  print(f"connected to room {ctx.room.name} with participant {participant.identity}")

  # inspect the current value of the attribute
  language = participant.attributes.get("user.language")

  # listen to when the attribute is changed
  @ctx.room.on("participant_attributes_changed")
  def on_participant_attributes_changed(changed_attrs: dict[str, str], p: rtc.Participant):
      if p == participant:
        language = p.attributes.get("user.language")
        print(f"participant {p.identity} changed language to {language}")

```

For more information, see the following articles:

- **[Room metadata](https://docs.livekit.io/home/client/state/room-metadata.md)**: Learn how to set and use room metadata.

- **[Participant attributes & metadata](https://docs.livekit.io/home/client/data.md#participant-attributes)**: Learn how to set and use participant attributes and metadata.

## Ending the session

### Disconnecting the agent

You can disconnect an agent after it completes its task and is no longer needed in the room. This allows the other participants in the LiveKit session to continue. Your [shutdown hooks](#post-processing-and-cleanup) run after the `shutdown` function.

**Python**:

```python
async def entrypoint(ctx: JobContext):
    # do some work
    ...

    # disconnect from the room
    ctx.shutdown(reason="Session ended")

```

### Disconnecting everyone

If the session should end for everyone, use the server API [deleteRoom](https://docs.livekit.io/home/server/managing-rooms.md#delete-a-room) to end the session.

The `Disconnected` [room event](https://docs.livekit.io/home/client/events.md) will be sent, and the room will be removed from the server.

**Python**:

```python
from livekit import api

async def entrypoint(ctx: JobContext):
    # do some work
    ...

    api_client = api.LiveKitAPI(
        os.getenv("LIVEKIT_URL"),
        os.getenv("LIVEKIT_API_KEY"),
        os.getenv("LIVEKIT_API_SECRET"),
    )
    await api_client.room.delete_room(api.DeleteRoomRequest(
        room=ctx.job.room.name,
    ))

```

## Post-processing and cleanup

After a session ends, you can perform post-processing or cleanup tasks using shutdown hooks. For example, you might want to save user state in a database.

**Python**:

```python
async def entrypoint(ctx: JobContext):
    async def my_shutdown_hook():
        # save user state
        ...
    ctx.add_shutdown_callback(my_shutdown_hook)

```

> ℹ️ **Note**
> 
> Shutdown hooks should complete within a short amount of time. By default, the framework waits 60 seconds before forcefully terminating the process. You can adjust this timeout using the `shutdown_process_timeout` parameter in [WorkerOptions](https://docs.livekit.io/agents/worker/options.md).

---

This document was rendered at 2025-07-18T19:17:01.721Z.
For the latest version of this document, see [https://docs.livekit.io/agents/worker/job.md](https://docs.livekit.io/agents/worker/job.md).

To explore all LiveKit documentation, see [llms.txt](https://docs.livekit.io/llms.txt).