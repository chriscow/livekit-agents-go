LiveKit Docs › Building voice agents › Workflows

---

# Workflows

> How to model repeatable, accurate tasks with multiple agents.

## Overview

Agents are composable within a single session to model complex tasks with a high degree of reliability. Some specific scenarios where this is useful include:

- Acquiring recording consent at the beginning of a call.
- Collecting specific structured information, such as an address or a credit card number.
- Moving through a series of questions, one at a time.
- Leaving a voicemail message when the user is unavailable.
- Including multiple personas with unique traits within a single session.

## Defining an agent

Extend the `Agent` class to define a custom agent.

```python
from livekit.agents import Agent

class HelpfulAssistant(Agent):
    def __init__(self):
        super().__init__(instructions="You are a helpful voice AI assistant.")

    async def on_enter(self) -> None:
        await self.session.say("Hello, how can I help you today?")

```

You can also create an instance of `Agent` class directly:

```python
agent = Agent(instructions="You are a helpful voice AI assistant.")

```

## Setting the active agent

Specify the initial agent in the `AgentSession` constructor:

```python
session = AgentSession(
    agent=HelpfulAssistant()
    # ...
)

```

To set a new agent, use the `update_agent` method:

```python
session.update_agent(HelpfulAssistant())

```

## Handing off from tool call

Return a different agent from within a tool call to hand off control automatically. This allows the LLM to make decisions about when handoff should occur. For more information, see [tool return value](https://docs.livekit.io/agents/build/tools.md#return-value).

```python
from livekit.agents import Agent, function_tool, get_job_context

class ConsentCollector(Agent):
    def __init__(self):
        super().__init__(
            instructions="""Your are a voice AI agent with the singular task to collect positive 
            recording consent from the user. If consent is not given, you must end the call."""
        )

    async def on_enter(self) -> None:
        await self.session.say("May I record this call for quality assurance purposes?")

    @function_tool()
    async def on_consent_given(self):
        """Use this tool to indicate that consent has been given and the call may proceed."""

        # Perform a handoff, immediately transfering control to the new agent
        return HelpfulAssistant()

    @function_tool()
    async def end_call(self) -> None:
        """Use this tool to indicate that consent has not been given and the call should end."""
        await self.session.say("Thank you for your time, have a wonderful day.")
        job_ctx = get_job_context()
        await job_ctx.api.room.delete_room(api.DeleteRoomRequest(room=job_ctx.room.name))

```

## Context preservation

By default, each new agent starts with a fresh conversation history for their LLM prompt. To include the prior conversation, set the `chat_ctx` parameter in the `Agent` constructor. You can either copy the prior agent's `chat_ctx`, or construct a new one based on custom business logic to provide the appropriate context.

```python
from livekit.agents import ChatContext, function_tool, Agent

class HelpfulAssistant(Agent):
    def __init__(self, chat_ctx: ChatContext):
        super().__init__(
            instructions="You are a helpful voice AI assistant.",
            chat_ctx=chat_ctx
        )

class ConsentCollector(Agent):
    # ...

    @function_tool()
    async def on_consent_given(self):
        """Use this tool to indicate that consent has been given and the call may proceed."""

        # Pass the chat context during handoff
        return HelpfulAssistant(chat_ctx=self.session.chat_ctx)

```

The complete conversation history for the session is always available in `session.history`.

## Passing state

To store custom state within your session, use the `userdata` attribute. The type of userdata is up to you, but the recommended approach is to use a `dataclass`.

```python
from livekit.agents import AgentSession
from dataclasses import dataclass

@dataclass
class MySessionInfo:
    user_name: str | None = None
    age: int | None = None

```

To add userdata to your session, pass it in the constructor. You must also specify the type of userdata on the `AgentSession` itself.

```python
session = AgentSession[MySessionInfo](
    userdata=MySessionInfo(),
    # ... tts, stt, llm, etc.
)

```

Userdata is available as `session.userdata`, and is also available within function tools on the `RunContext`. The following example shows how to use userdata in an agent workflow that starts with the `IntakeAgent`.

```python
class IntakeAgent(Agent):
    def __init__(self):
        super().__init__(
            instructions="""Your are an intake agent. Learn the user's name and age."""
        )
        
    @function_tool()
    async def record_name(self, context: RunContext[MySessionInfo], name: str):
        """Use this tool to record the user's name."""
        context.userdata.user_name = name
        return self._handoff_if_done()
    
    @function_tool()
    async def record_age(self, context: RunContext[MySessionInfo], age: int):
        """Use this tool to record the user's age."""
        context.userdata.age = age
        return self._handoff_if_done()
    
    def _handoff_if_done(self):
        if self.session.userdata.user_name and self.session.userdata.age:
            return HelpfulAssistant()
        else:
            return None

class HelpfulAssistant(Agent):
    def __init__(self):
        super().__init__(instructions="You are a helpful voice AI assistant.")

    async def on_enter(self) -> None:
        userdata: MySessionInfo = self.session.userdata
        await self.session.generate_reply(
            instructions=f"Greet {userdata.user_name} and tell them a joke about being {userdata.age} years old."
        )

```

## Overriding plugins

You can override any of the plugins used in the session by setting the corresponding attributes in your `Agent` constructor. For instance, you can change the voice for a specific agent by overriding the `tts` attribute:

```python
from livekit.agents import Agent
from livekit.plugins import cartesia

class AssistantManager(Agent):
    def __init__(self):
        super().__init__(
            instructions="You are manager behind a team of helpful voice assistants.",
            tts=cartesia.TTS(voice="6f84f4b8-58a2-430c-8c79-688dad597532")
        )

```

## Examples

These examples show how to build more complex workflows with multiple agents:

- **[Drive-thru agent](https://github.com/livekit/agents/blob/main/examples/drive-thru)**: A complex food ordering agent with tasks, tools, and a complete evaluation suite.

- **[Front-desk agent](https://github.com/livekit/agents/blob/main/examples/frontdesk)**: A calendar booking agent with tasks, tools, and evaluations.

- **[Medical Office Triage](https://github.com/livekit-examples/python-agents-examples/tree/main/complex-agents/medical_office_triage)**: Agent that triages patients based on symptoms and medical history.

- **[Restaurant Agent](https://github.com/livekit/agents/blob/main/examples/voice_agents/restaurant_agent.py)**: A restaurant front-of-house agent that can take orders, add items to a shared cart, and checkout.

## Further reading

For more information on concepts touched on in this article, see the following related articles:

- **[Tool definition and use](https://docs.livekit.io/agents/build/tools.md)**: Complete guide to defining and using tools in your agents.

- **[Nodes](https://docs.livekit.io/agents/build/nodes.md)**: Add custom behavior to any component of the voice pipeline.

- **[Agent speech](https://docs.livekit.io/agents/build/audio.md)**: Customize the speech output of your agents.

- **[Testing & evaluation](https://docs.livekit.io/agents/build/testing.md)**: Test every aspect of your agents with a custom test suite.

---

This document was rendered at 2025-07-18T19:10:57.652Z.
For the latest version of this document, see [https://docs.livekit.io/agents/build/workflows.md](https://docs.livekit.io/agents/build/workflows.md).

To explore all LiveKit documentation, see [llms.txt](https://docs.livekit.io/llms.txt).