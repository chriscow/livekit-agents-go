LiveKit Docs › Building voice agents › Tool definition & use

---

# Tool definition and use

> Let your agents call external tools and more.

## Overview

LiveKit Agents has full support for LLM tool use. This feature allows you to create a custom library of tools to extend your agent's context, create interactive experiences, and overcome LLM limitations. Within a tool, you can:

- Generate [agent speech](https://docs.livekit.io/agents/build/audio.md) with `session.say()` or `session.generate_reply()`.
- Call methods on the frontend using [RPC](https://docs.livekit.io/home/client/data/rpc.md).
- Handoff control to another agent as part of a [workflow](https://docs.livekit.io/agents/build/workflows.md).
- Store and retrieve session data from the `context`.
- Anything else that a Python function can do.
- [Call external APIs or lookup data for RAG](https://docs.livekit.io/agents/build/external-data.md).

## Tool definition

Add tools to your agent class with the `@function_tool` decorator. The LLM has access to them automatically.

```python
from livekit.agents import function_tool, Agent, RunContext


class MyAgent(Agent):
    @function_tool()
    async def lookup_weather(
        self,
        context: RunContext,
        location: str,
    ) -> dict[str, Any]:
        """Look up weather information for a given location.
        
        Args:
            location: The location to look up weather information for.
        """

        return {"weather": "sunny", "temperature_f": 70}

```

> 💡 **Best practices**
> 
> A good tool definition is key to reliable tool use from your LLM. Be specific about what the tool does, when it should or should not be used, what the arguments are for, and what type of return value to expect.

### Name and description

By default, the tool name is the name of the function, and the description is its docstring. Override this behavior with the `name` and `description` arguments to the `@function_tool` decorator.

### Arguments

The tool arguments are copied automatically by name from the function arguments. Type hints for arguments are included, if present.

Place additional information about the tool arguments, if needed, in the tool description.

### Return value

The tool return value is automatically converted to a string before being sent to the LLM. The LLM generates a new reply or additional tool calls based on the return value. Return `None` or nothing at all to complete the tool silently without requiring a reply from the LLM.

You can return an `Agent` instance to initiate a [handoff within a workflow](https://docs.livekit.io/agents/build/workflows.md#tool-handoff). You can additionally return the result of the original tool call to the LLM by returning a tuple that includes both the `Agent` instance and the result. The tool call and subsequent LLM reply are completed prior to the handoff.

When a handoff occurs, prompt the LLM to inform the user:

```python
@function_tool()
async def my_tool(context: RunContext):
    return SomeAgent(), "Transferring the user to SomeAgent"

```

### RunContext

Tools include support for a special `context` argument. This contains access to the current `session`, `function_call`, `speech_handle`, and `userdata`. Consult the documentation on [speech](https://docs.livekit.io/agents/build/audio.md) and [state within workflows](https://docs.livekit.io/agents/build/workflows.md) for more information about how to use these features.

### Adding tools dynamically

You can exercise more control over the tools available by setting the `tools` argument directly.

To share a tool between multiple agents, define it outside of their class and then provide it to each. The `RunContext` is especially useful for this purpose to access the current session, agent, and state.

Tools set in the `tools` value are available alongside any registered within the class using the `@function_tool` decorator.

```python
from livekit.agents import function_tool, Agent, RunContext

@function_tool()
async def lookup_user(
    context: RunContext,
    user_id: str,
) -> dict:
    """Look up a user's information by ID."""

    return {"name": "John Doe", "email": "john.doe@example.com"}


class AgentA(Agent):
    def __init__(self):
        super().__init__(
            tools=[lookup_user],
            # ...
        )


class AgentB(Agent):
    def __init__(self):
        super().__init__(
            tools=[lookup_user],
            # ...
        )

```

Use `agent.update_tools()` to update available tools after creating an agent. This replaces _all_ tools, including those registered automatically within the agent class. To reference existing tools before replacement, access the `agent.tools` property:

```python
# add a tool
await agent.update_tools(agent.tools + [tool_a])

# remove a tool
await agent.update_tools(agent.tools - [tool_a]) 

# replace all tools
await agent.update_tools([tool_a, tool_b]) 

```

### Creating tools programmatically

To create a tool on the fly, use `function_tool` as a function rather than as a decorator. You must supply a name, description, and callable function. This is useful to compose specific tools based on the same underlying code or load them from external sources such as a database or Model Context Protocol (MCP) server.

In the following example, the app has a single function to set any user profile field but gives the agent one tool per field for improved reliability:

```python
from livekit.agents import function_tool, RunContext

class Assistant(Agent):
    def _set_profile_field_func_for(self, field: str):
        async def set_value(context: RunContext, value: str):
            # custom logic to set input
            return f"field {field} was set to {value}"

        return set_value

    def __init__(self):
        super().__init__(
            tools=[
                function_tool(self._set_profile_field_func_for("phone"),
                              name="set_phone_number",
                              description="Call this function when user has provided their phone number."),
                function_tool(self._set_profile_field_func_for("email"),
                              name="set_email",
                              description="Call this function when user has provided their email."),
                # ... other tools ...
            ],
            # instructions, etc ...
        )

```

### Creating tools from raw schema

For advanced use cases, you can create tools directly from a [raw function calling schema](https://platform.openai.com/docs/guides/function-calling?api-mode=responses). This is useful when integrating with existing function definitions, loading tools from external sources, or working with schemas that don't map cleanly to Python function signatures.

Use the `raw_schema` parameter in the `@function_tool` decorator to provide the full function schema:

```python
from livekit.agents import function_tool, RunContext

raw_schema = {
    "type": "function",
    "name": "get_weather",
    "description": "Get weather for a given location.",
    "parameters": {
        "type": "object",
        "properties": {
            "location": {
                "type": "string",
                "description": "City and country e.g. New York"
            }
        },
        "required": [
            "location"
        ],
        "additionalProperties": False
    }
}

@function_tool(raw_schema=raw_schema)
async def get_weather(raw_arguments: dict[str, object], context: RunContext):
    location = raw_arguments["location"]
    
    # Your implementation here
    return f"The weather of {location} is ..."

```

When using raw schemas, function parameters are passed to your handler as a dictionary named `raw_arguments`. You can extract values from this dictionary using the parameter names defined in your schema.

You can also create tools programmatically using `function_tool` as a function with a raw schemas:

```python
from livekit.agents import function_tool

def create_database_tool(table_name: str, operation: str):
    schema = {
        "type": "function",
        "name": f"{operation}_{table_name}",
        "description": f"Perform {operation} operation on {table_name} table",
        "parameters": {
            "type": "object",
            "properties": {
                "record_id": {
                    "type": "string",
                    "description": f"ID of the record to {operation}"
                }
            },
            "required": ["record_id"]
        }
    }
    
    async def handler(raw_arguments: dict[str, object], context: RunContext):
        record_id = raw_arguments["record_id"]
        # Perform database operation
        return f"Performed {operation} on {table_name} for record {record_id}"
    
    return function_tool(handler, raw_schema=schema)

# Create tools dynamically
user_tools = [
    create_database_tool("users", "read"),
    create_database_tool("users", "update"),  
    create_database_tool("users", "delete")
]

class DataAgent(Agent):
    def __init__(self):
        super().__init__(
            instructions="You are a database assistant.",
            tools=user_tools,
        )

```

## Error handling

Raise the `ToolError` exception to return an error to the LLM in place of a response. You can include a custom message to describe the error and/or recovery options.

```python
@function_tool()
async def lookup_weather(
    self,
    context: RunContext,
    location: str,
) -> dict[str, Any]:
    if location == "mars":
        raise ToolError("This location is coming soon. Please join our mailing list to stay updated.")
    else:
        return {"weather": "sunny", "temperature_f": 70}

```

## Model Context Protocol (MCP)

LiveKit Agents has full support for [MCP](https://modelcontextprotocol.io/) servers to load tools from external sources.

To use it, first install the `mcp` optional dependencies:

```shell
pip install livekit-agents[mcp]~=1.0

```

Then pass the MCP server URL to the `AgentSession` or `Agent` constructor. The tools will be automatically loaded like any other tool.

```python
from livekit.agents import mcp

session = AgentSession(
    #... other arguments ...
    mcp_servers=[
        mcp.MCPServerHTTP(
            "https://your-mcp-server.com"
        )       
    ]       
)

```

```python
from livekit.agents import mcp

agent = Agent(
    #... other arguments ...
    mcp_servers=[
        mcp.MCPServerHTTP(
            "https://your-mcp-server.com"
        )       
    ]       
)

```

## Forwarding to the frontend

Forward tool calls to a frontend app using [RPC](https://docs.livekit.io/home/client/data/rpc.md). This is useful when the data needed to fulfill the function call is only available at the frontend. You may also use RPC to trigger actions or UI updates in a structured way.

For instance, here's a function that accesses the user's live location from their web browser:

### Agent implementation

```python
from livekit.agents import function_tool, get_job_context, RunContext

@function_tool()
async def get_user_location(
    context: RunContext,
    high_accuracy: bool
):
    """Retrieve the user's current geolocation as lat/lng.
    
    Args:
        high_accuracy: Whether to use high accuracy mode, which is slower but more precise
    
    Returns:
        A dictionary containing latitude and longitude coordinates
    """
    try:
        room = get_job_context().room
        participant_identity = next(iter(room.remote_participants))
        response = await room.local_participant.perform_rpc(
            destination_identity=participant_identity,
            method="getUserLocation",
            payload=json.dumps({
                "highAccuracy": high_accuracy
            }),
            response_timeout=10.0 if high_accuracy else 5.0,
        )
        return response
    except Exception:
        raise ToolError("Unable to retrieve user location")

```

### Frontend implementation

The following example uses the JavaScript SDK. The same pattern works for other SDKs. For more examples, see the [RPC documentation](https://docs.livekit.io/home/client/data/rpc.md).

```typescript
import { RpcError, RpcInvocationData } from 'livekit-client';

localParticipant.registerRpcMethod(
    'getUserLocation',
    async (data: RpcInvocationData) => {
        try {
            let params = JSON.parse(data.payload);
            const position: GeolocationPosition = await new Promise((resolve, reject) => {
                navigator.geolocation.getCurrentPosition(resolve, reject, {
                    enableHighAccuracy: params.highAccuracy ?? false,
                    timeout: data.responseTimeout,
                });
            });

            return JSON.stringify({
                latitude: position.coords.latitude,
                longitude: position.coords.longitude,
            });
        } catch (error) {
            throw new RpcError(1, "Could not retrieve user location");
        }
    }
);

```

## Slow tool calls

For best practices on providing feedback to the user during long-running tool calls, see the section on [user feedback](https://docs.livekit.io/agents/build/external-data.md#user-feedback) in the External data and RAG guide.

## External tools and MCP

To load tools from an external source as a Model Context Protocol (MCP) server, use the `function_tool` function and register the tools with the `tools` property or `update_tools()` method. See the following example for a complete MCP implementation:

- **[MCP Agent](https://github.com/livekit-examples/python-agents-examples/tree/main/mcp)**: A voice AI agent with an integrated Model Context Protocol (MCP) server for the LiveKit API.

## Examples

The following additional examples show how to use tools in different ways:

- **[Use of enum](https://github.com/livekit/agents/blob/main/examples/voice_agents/annotated_tool_args.py)**: Example showing how to annotate arguments with enum.

- **[Dynamic tool creation](https://github.com/livekit/agents/blob/main/examples/voice_agents/dynamic_tool_creation.py)**: Complete example with dynamic tool lists.

- **[MCP Agent](https://github.com/livekit-examples/python-agents-examples/tree/main/mcp)**: A voice AI agent with an integrated Model Context Protocol (MCP) server for the LiveKit API.

- **[LiveKit Docs RAG](https://github.com/livekit-examples/python-agents-examples/tree/main/rag)**: An agent that can answer questions about LiveKit with lookups against the docs website.

## Further reading

The following articles provide more information about the topics discussed in this guide:

- **[RPC](https://docs.livekit.io/home/client/data/rpc.md)**: Complete documentation on function calling between LiveKit participants.

- **[Agent speech](https://docs.livekit.io/agents/build/audio.md)**: More information about precise control over agent speech output.

- **[Workflows](https://docs.livekit.io/agents/build/workflows.md)**: Read more about handing off control to other agents.

- **[External data and RAG](https://docs.livekit.io/agents/build/external-data.md)**: Best practices for adding context and taking external actions.

---

This document was rendered at 2025-07-18T19:12:05.339Z.
For the latest version of this document, see [https://docs.livekit.io/agents/build/tools.md](https://docs.livekit.io/agents/build/tools.md).

To explore all LiveKit documentation, see [llms.txt](https://docs.livekit.io/llms.txt).