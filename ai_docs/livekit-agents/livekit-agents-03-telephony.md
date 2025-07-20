LiveKit Docs › Getting started › Telephony integration

---

# Agents telephony integration

> Enable your voice AI agent to make and receive phone calls.

## Overview

It's easy to integrate LiveKit Agents with telephony systems using Session Initiation Protocol (SIP). You can choose to support inbound calls, outbound calls, or both. LiveKit also provides features including DTMF, SIP REFER, and more.

Telephony integration requires no significant changes to your existing agent code, as phone calls are simply bridged into LiveKit rooms using a special participant type.

## Getting started

1. Follow the [Voice AI quickstart](https://docs.livekit.io/agents/start/voice-ai.md) to get a simple agent up and running.
2. Set up a SIP trunk for your project.
3. Return to this guide to enable inbound and outbound calls.

- **[Voice AI quickstart](https://docs.livekit.io/agents/start/voice-ai.md)**: Follow the Voice AI quickstart to get your agent up and running.

- **[SIP trunk setup](https://docs.livekit.io/sip/quickstarts/configuring-sip-trunk.md)**: Configure your SIP trunk provider to route calls in LiveKit.

## Agent dispatch

LiveKit recommends using explicit agent dispatch for telephony integrations to ensure no unexpected automatic dispatch occurs given the complexity of inbound and outbound calling.

To enable explicit dispatch, give your agent a name. This disables automatic dispatch.

** Filename: `agent.py`**

```python
# ... your existing agent code ...

if __name__ == "__main__":
    agents.cli.run_app(agents.WorkerOptions(
        entrypoint_fnc=entrypoint,

        # agent_name is required for explicit dispatch
        agent_name="my-telephony-agent"
    ))

```

> 💡 **Full examples**
> 
> See the docs on [agent dispatch](https://docs.livekit.io/agents/worker/agent-dispatch.md) for more complete examples.

## Inbound calls

After you configure your [inbound trunk](https://docs.livekit.io/sip/trunk-inbound.md) follow these steps to enable inbound calling for your agent.

### Dispatch rules

The following rule routes all inbound calls to a new room and dispatches your agent to that room:

** Filename: `dispatch-rule.json`**

```json
{
    "rule": {
        "dispatchRuleIndividual": {
            "roomPrefix": "call-"
        }
    },
    "roomConfig": {
        "agents": [{
            "agentName": "my-telephony-agent"
        }]
    }
}

```

Create this rule with the following command:

```shell
lk sip dispatch create dispatch-rule.json

```

### Answering the phone

Call the `generate_reply` method of your `AgentSession` to greet the caller after picking up. This code goes after `session.start`:

```python
await session.generate_reply(
    instructions="Greet the user and offer your assistance."
)

```

### Call your agent

After you start your agent with the following command, dial the number you set up earlier to hear your agent answer the phone.

```shell
python agent.py dev

```

## Outbound calls

After setting up your [outbound trunk](https://docs.livekit.io/sip/trunk-outbound.md), you may place outbound calls by dispatching an agent and then creating a SIP participant.

The following guide describes how to modify the [voice AI quickstart](https://docs.livekit.io/agents/start/voice-ai.md) for outbound calling. Alternatively, see the following complete example on GitHub:

- **[Outbound caller example](https://github.com/livekit-examples/outbound-caller-python)**: Complete example of an outbound calling agent.

### Dialing a number

Add the following code so your agent reads the phone number and places an outbound call by creating a SIP participant after connection.

You should also remove the initial greeting or place it behind an `if` statement to ensure the agent waits for the user to speak first when placing an outbound call.

> ℹ️ **SIP trunk ID**
> 
> You must fill in the `sip_trunk_id` for this example to work. You can get this from LiveKit CLI with `lk sip outbound list`.

** Filename: `agent.py`**

```python
# add these imports at the top of your file
from livekit import api
import json

# ... any existing code / imports ...

def entrypoint(ctx: agents.JobContext):
    await ctx.connect()

    # If a phone number was provided, then place an outbound call
    # By having a condition like this, you can use the same agent for inbound/outbound telephony as well as web/mobile/etc.
    dial_info = json.loads(ctx.job.metadata)
    phone_number = dial_info["phone_number"]

    # The participant's identity can be anything you want, but this example uses the phone number itself
    sip_participant_identity = phone_number
    if phone_number is not None:
        # The outbound call will be placed after this method is executed
        try:
            await ctx.api.sip.create_sip_participant(api.CreateSIPParticipantRequest(
                # This ensures the participant joins the correct room
                room_name=ctx.room.name,

                # This is the outbound trunk ID to use (i.e. which phone number the call will come from)
                # You can get this from LiveKit CLI with `lk sip outbound list`
                sip_trunk_id='ST_xxxx',

                # The outbound phone number to dial and identity to use
                sip_call_to=phone_number,
                participant_identity=sip_participant_identity,

                # This will wait until the call is answered before returning
                wait_until_answered=True,
            ))

            print("call picked up successfully")
        except api.TwirpError as e:
            print(f"error creating SIP participant: {e.message}, "
                  f"SIP status: {e.metadata.get('sip_status_code')} "
                  f"{e.metadata.get('sip_status')}")
            ctx.shutdown()
    
    # .. create and start your AgentSession as normal ...

    # Add this guard to ensure the agent only speaks first in an inbound scenario.
    # When placing an outbound call, its more customary for the recipient to speak first
    # The agent will automatically respond after the user's turn has ended.
    if phone_number is None:
        await session.generate_reply(
            instructions="Greet the user and offer your assistance."
        )

```

### Make a call with your agent

Use either the LiveKit CLI or the Python API to instruct your agent to place an outbound phone call.

In this example, the job's metadata includes the phone number to call. You can extend this to include more information if needed for your use case.

**LiveKit CLI**:

The following command creates a new room and dispatches your agent to it with the phone number to call. Ensure the agent name matches the name you set earlier in the [agent dispatch](#agent-dispatch) section.

```shell
lk dispatch create \
    --new-room \
    --agent-name my-telephony-agent \
    --metadata '{"phone_number": "+15105550123"}' # insert your own phone number here

```

---

**Python**:

```python
await lkapi.agent_dispatch.create_dispatch(
    api.CreateAgentDispatchRequest(
        # Use the agent name you set in the WorkerOptions
        agent_name="my-telephony-agent", 

        # The room name to use. This should be unique for each call
        room=f"outbound-{''.join(str(random.randint(0, 9)) for _ in range(10))}",

        # Here we use JSON to pass the phone number, and could add more information if needed.
        metadata='{"phone_number": "+15105550123"}'
    )
)

```

### Voicemail detection

Your agent may still encounter an automated system such as an answering machine or voicemail. You can give your LLM the ability to detect a likely voicemail system via tool call, and then perform special actions such as leaving a message and [hanging up](#hangup).

** Filename: `agent.py`**

```python
import asyncio # add this import at the top of your file

class Assistant(Agent):
    ## ... existing init code ...
        
    @function_tool
    async def detected_answering_machine(self):
        """Call this tool if you have detected a voicemail system, AFTER hearing the voicemail greeting"""
        await self.session.generate_reply(
            instructions="Leave a voicemail message letting the user know you'll call back later."
        )
        await asyncio.sleep(0.5) # Add a natural gap to the end of the voicemail message
        await hangup_call()

```

## Hangup

To end a call for all participants, use the `delete_room` API. If only the agent session ends, the user will continue to hear silence until they hang up. The example below shows a basic `hangup_call` function you can use as a starting point.

** Filename: `agent.py`**

```python
# Add these imports at the top of your file
from livekit import api, rtc
from livekit.agents import get_job_context

# Add this function definition anywhere
async def hangup_call():
    ctx = get_job_context()
    if ctx is None:
        # Not running in a job context
        return
    
    await ctx.api.room.delete_room(
        api.DeleteRoomRequest(
            room=ctx.room.name,
        )
    )

class MyAgent(Agent):
    ...

    # to hang up the call as part of a function call
    @function_tool
    async def end_call(self, ctx: RunContext):
        """Called when the user wants to end the call"""
        # let the agent finish speaking
        current_speech = ctx.session.current_speech
        if current_speech:
            await current_speech.wait_for_playout()

        await hangup_call()

```

## Transferring call to another number

In case the agent needs to transfer the call to another number or SIP destination, you can use the `transfer_sip_participant` API.

This is a "cold" transfer, where the agent hands the call off to another party without staying on the line. The current session ends after the transfer is complete.

** Filename: `agent.py`**

```python
class Assistant(Agent):
    ## ... existing init code ...

    @function_tool()
    async def transfer_call(self, ctx: RunContext):
        """Transfer the call to a human agent, called after confirming with the user"""

        transfer_to = "+15105550123"
        participant_identity = "+15105550123"

        # let the message play fully before transferring
        await ctx.session.generate_reply(
            instructions="Inform the user that you're transferring them to a different agent."
        )

        job_ctx = get_job_context()
        try:
            await job_ctx.api.sip.transfer_sip_participant(
                api.TransferSIPParticipantRequest(
                    room_name=job_ctx.room.name,
                    participant_identity=participant_identity,
                    # to use a sip destination, use `sip:user@host` format
                    transfer_to=f"tel:{transfer_to}",
                )
            )
        except Exception as e:
            print(f"error transferring call: {e}")
            # give the LLM that context
            return "could not transfer call"

```

> ℹ️ **SIP REFER**
> 
> You must enable SIP REFER on your SIP trunk provider to use `transfer_sip_participant`. For Twilio, you must also enable `Enable PSTN Transfer`.

## Recipes

The following recipes are particular helpful to learn more about telephony integration.

- **[Company Directory](https://docs.livekit.io/recipes/company-directory.md)**: Build a AI company directory agent. The agent can respond to DTMF tones and voice prompts, then redirect callers.

- **[SIP Warm Handoff](https://github.com/livekit-examples/python-agents-examples/tree/main/telephony/warm_handoff.py)**: Transfer calls from an AI agent to a human operator seamlessly.

- **[SIP Lifecycle](https://github.com/livekit-examples/python-agents-examples/tree/main/telephony/sip_lifecycle.py)**: Complete lifecycle management for SIP calls.

- **[Survey Caller](https://github.com/livekit-examples/python-agents-examples/tree/main/telephony/survey_caller/)**: Automated survey calling system.

## Further reading

The following guides provide more information on building voice agents for telephony.

- **[Workflows](https://docs.livekit.io/agents/build/workflows.md)**: Orchestrate detailed workflows such as collecting credit card information over the phone.

- **[Tool definition & use](https://docs.livekit.io/agents/build/tools.md)**: Extend your agent's capabilities with tools.

- **[Telephony documentation](https://docs.livekit.io/sip.md)**: Full documentation on the LiveKit SIP integration and features.

- **[Agent speech](https://docs.livekit.io/agents/build/audio.md)**: Customize and perfect your agent's verbal interactions.

---

This document was rendered at 2025-07-18T19:09:29.139Z.
For the latest version of this document, see [https://docs.livekit.io/agents/start/telephony.md](https://docs.livekit.io/agents/start/telephony.md).

To explore all LiveKit documentation, see [llms.txt](https://docs.livekit.io/llms.txt).