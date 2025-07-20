package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"livekit-agents-go/services/llm"
	"livekit-agents-go/services/tools"
	"livekit-agents-go/test/mock"
)

// Test agent with function tools
type ToolsTestAgent struct {
	*BaseAgent
	callLog []string
	state   map[string]interface{}
}

type WeatherParams struct {
	Location string `json:"location"`
	Units    string `json:"units,omitempty"`
}

type WeatherResult struct {
	Location    string  `json:"location"`
	Temperature float64 `json:"temperature"`
	Condition   string  `json:"condition"`
	Units       string  `json:"units"`
}

func NewToolsTestAgent() *ToolsTestAgent {
	return &ToolsTestAgent{
		BaseAgent: NewBaseAgent("tools-test"),
		callLog:   make([]string, 0),
		state:     make(map[string]interface{}),
	}
}

func (a *ToolsTestAgent) GetWeather(ctx context.Context, params WeatherParams) (*WeatherResult, error) {
	a.callLog = append(a.callLog, fmt.Sprintf("GetWeather(%s)", params.Location))
	
	units := params.Units
	if units == "" {
		units = "celsius"
	}
	
	// Mock weather data
	result := &WeatherResult{
		Location:    params.Location,
		Temperature: 22.5,
		Condition:   "sunny",
		Units:       units,
	}
	
	return result, nil
}

func (a *ToolsTestAgent) SetReminder(ctx context.Context, params struct {
	Message string `json:"message"`
	Minutes int    `json:"minutes"`
}) (string, error) {
	a.callLog = append(a.callLog, fmt.Sprintf("SetReminder(%s, %d)", params.Message, params.Minutes))
	
	reminderID := fmt.Sprintf("reminder_%d", len(a.callLog))
	a.state[reminderID] = params
	
	return reminderID, nil
}

func (a *ToolsTestAgent) Calculate(ctx context.Context, params struct {
	Expression string `json:"expression"`
}) (float64, error) {
	a.callLog = append(a.callLog, fmt.Sprintf("Calculate(%s)", params.Expression))
	
	// Simple mock calculation
	switch params.Expression {
	case "2 + 2":
		return 4.0, nil
	case "10 * 5":
		return 50.0, nil
	default:
		return 0.0, fmt.Errorf("unsupported expression: %s", params.Expression)
	}
}

func (a *ToolsTestAgent) NoParamsTool(ctx context.Context) (string, error) {
	a.callLog = append(a.callLog, "NoParamsTool()")
	return "no params result", nil
}

func TestToolDiscoveryAndRegistration(t *testing.T) {
	agent := NewToolsTestAgent()
	
	// Test tool discovery
	discoveredTools, err := tools.DiscoverTools(agent)
	if err != nil {
		t.Fatalf("Failed to discover tools: %v", err)
	}
	
	if len(discoveredTools) == 0 {
		t.Fatal("No tools discovered")
	}
	
	// Verify expected tools are discovered
	expectedTools := map[string]bool{
		"get_weather":    false,
		"set_reminder":   false,
		"calculate":      false,
		"no_params_tool": false,
	}
	
	for _, tool := range discoveredTools {
		name := tool.Name()
		if _, exists := expectedTools[name]; exists {
			expectedTools[name] = true
		}
	}
	
	for toolName, found := range expectedTools {
		if !found {
			t.Errorf("Expected tool %s not discovered", toolName)
		}
	}
	
	// Test tool registration in registry
	registry := tools.NewToolRegistry()
	for _, tool := range discoveredTools {
		err = registry.Register(tool)
		if err != nil {
			t.Fatalf("Failed to register tool %s: %v", tool.Name(), err)
		}
	}
	
	if registry.Count() != len(discoveredTools) {
		t.Errorf("Expected %d tools in registry, got %d", len(discoveredTools), registry.Count())
	}
}

func TestToolExecution(t *testing.T) {
	agent := NewToolsTestAgent()
	registry := tools.NewToolRegistry()
	
	// Discover and register tools
	discoveredTools, err := tools.DiscoverTools(agent)
	if err != nil {
		t.Fatalf("Failed to discover tools: %v", err)
	}
	
	for _, tool := range discoveredTools {
		registry.Register(tool)
	}
	
	ctx := context.Background()
	
	// Test GetWeather tool
	weatherTool, exists := registry.Lookup("get_weather")
	if !exists {
		t.Fatal("GetWeather tool not found")
	}
	
	weatherParams := WeatherParams{
		Location: "San Francisco",
		Units:    "fahrenheit",
	}
	weatherArgs, _ := json.Marshal(weatherParams)
	
	result, err := weatherTool.Call(ctx, weatherArgs)
	if err != nil {
		t.Fatalf("GetWeather tool call failed: %v", err)
	}
	
	var weatherResult WeatherResult
	err = json.Unmarshal(result, &weatherResult)
	if err != nil {
		t.Fatalf("Failed to unmarshal weather result: %v", err)
	}
	
	if weatherResult.Location != "San Francisco" {
		t.Errorf("Expected location 'San Francisco', got '%s'", weatherResult.Location)
	}
	if weatherResult.Units != "fahrenheit" {
		t.Errorf("Expected units 'fahrenheit', got '%s'", weatherResult.Units)
	}
	
	// Verify method was called
	if len(agent.callLog) != 1 || agent.callLog[0] != "GetWeather(San Francisco)" {
		t.Errorf("Expected call log ['GetWeather(San Francisco)'], got: %v", agent.callLog)
	}
	
	// Test Calculate tool
	calcTool, exists := registry.Lookup("calculate")
	if !exists {
		t.Fatal("Calculate tool not found")
	}
	
	calcParams := struct {
		Expression string `json:"expression"`
	}{Expression: "2 + 2"}
	calcArgs, _ := json.Marshal(calcParams)
	
	result, err = calcTool.Call(ctx, calcArgs)
	if err != nil {
		t.Fatalf("Calculate tool call failed: %v", err)
	}
	
	var calcResult float64
	err = json.Unmarshal(result, &calcResult)
	if err != nil {
		t.Fatalf("Failed to unmarshal calculation result: %v", err)
	}
	
	if calcResult != 4.0 {
		t.Errorf("Expected calculation result 4.0, got %f", calcResult)
	}
	
	// Test no params tool
	noParamsTool, exists := registry.Lookup("no_params_tool")
	if !exists {
		t.Fatal("NoParamsTool not found")
	}
	
	result, err = noParamsTool.Call(ctx, nil)
	if err != nil {
		t.Fatalf("NoParamsTool call failed: %v", err)
	}
	
	var noParamsResult string
	err = json.Unmarshal(result, &noParamsResult)
	if err != nil {
		t.Fatalf("Failed to unmarshal no params result: %v", err)
	}
	
	if noParamsResult != "no params result" {
		t.Errorf("Expected 'no params result', got '%s'", noParamsResult)
	}
}

func TestToolIntegrationWithAgentSession(t *testing.T) {
	// Initialize mock services
	mock.RegisterMockPlugin()
	
	agent := NewToolsTestAgent()
	
	// Create agent session
	ctx := context.Background()
	session := NewAgentSessionWithInstructions(ctx, "You are a helpful assistant with access to tools.")
	
	// Discover and register tools
	discoveredTools, err := tools.DiscoverTools(agent)
	if err != nil {
		t.Fatalf("Failed to discover tools: %v", err)
	}
	
	for _, tool := range discoveredTools {
		err = session.ToolRegistry.Register(tool)
		if err != nil {
			t.Fatalf("Failed to register tool: %v", err)
		}
	}
	
	// Verify tools are available in session
	if session.ToolRegistry.Count() != len(discoveredTools) {
		t.Errorf("Expected %d tools in session registry, got %d", len(discoveredTools), session.ToolRegistry.Count())
	}
	
	toolNames := session.ToolRegistry.Names()
	if len(toolNames) == 0 {
		t.Error("No tool names found in session registry")
	}
	
	// Test that tools are available through the registry
	sessionTools := session.ToolRegistry.List()
	if len(sessionTools) != len(discoveredTools) {
		t.Errorf("Expected %d tools in registry, got %d", len(discoveredTools), len(sessionTools))
	}
	
	// Verify tool schemas are available
	for _, tool := range sessionTools {
		if tool.Schema() == nil {
			t.Errorf("Tool %s has nil schema", tool.Name())
		}
	}
}

func TestFunctionCallExecution(t *testing.T) {
	mock.RegisterMockPlugin()
	
	agent := NewToolsTestAgent()
	ctx := context.Background()
	session := NewAgentSessionWithInstructions(ctx, "You are a helpful assistant.")
	
	// Register tools
	discoveredTools, _ := tools.DiscoverTools(agent)
	for _, tool := range discoveredTools {
		session.ToolRegistry.Register(tool)
	}
	
	// Simulate function call from LLM using the correct ToolCall structure
	toolCall := llm.ToolCall{
		ID:   "call_123",
		Type: "function",
		Function: llm.Function{
			Name:      "get_weather",
			Arguments: `{"location": "New York", "units": "celsius"}`,
		},
	}
	
	// Execute function call using the private method (we'll test this indirectly)
	// Since executeFunctionCall is private, let's test the tool execution directly
	tool, exists := session.ToolRegistry.Lookup("get_weather")
	if !exists {
		t.Fatal("get_weather tool not found in registry")
	}
	
	result, err := tool.Call(ctx, []byte(toolCall.Function.Arguments))
	if err != nil {
		t.Fatalf("Tool call execution failed: %v", err)
	}
	
	if result == nil {
		t.Fatal("Tool call result is nil")
	}
	
	// Verify result contains expected data
	var weatherResult WeatherResult
	err = json.Unmarshal(result, &weatherResult)
	if err != nil {
		t.Fatalf("Failed to unmarshal function result: %v", err)
	}
	
	if weatherResult.Location != "New York" {
		t.Errorf("Expected location 'New York', got '%s'", weatherResult.Location)
	}
	
	// Verify agent method was called
	if len(agent.callLog) != 1 {
		t.Errorf("Expected 1 function call, got %d", len(agent.callLog))
	}
	
	expected := "GetWeather(New York)"
	if agent.callLog[0] != expected {
		t.Errorf("Expected call log entry '%s', got '%s'", expected, agent.callLog[0])
	}
}

func TestToolErrorHandling(t *testing.T) {
	agent := NewToolsTestAgent()
	registry := tools.NewToolRegistry()
	
	discoveredTools, _ := tools.DiscoverTools(agent)
	for _, tool := range discoveredTools {
		registry.Register(tool)
	}
	
	ctx := context.Background()
	
	// Test invalid tool name
	_, exists := registry.Lookup("nonexistent_tool")
	if exists {
		t.Error("Found non-existent tool")
	}
	
	// Test invalid arguments
	calcTool, _ := registry.Lookup("calculate")
	invalidArgs := `{"invalid": "json"}`
	
	result, err := calcTool.Call(ctx, []byte(invalidArgs))
	if err != nil {
		// This is expected - the tool should handle invalid arguments gracefully
		t.Logf("Expected error for invalid arguments: %v", err)
	}
	
	// Test calculation with unsupported expression
	unsupportedArgs := `{"expression": "unsupported operation"}`
	result, err = calcTool.Call(ctx, []byte(unsupportedArgs))
	if err == nil {
		t.Error("Expected error for unsupported expression")
	}
	
	// Verify result is nil when error occurs
	if result != nil {
		t.Error("Expected nil result when error occurs")
	}
}

func TestConcurrentToolExecution(t *testing.T) {
	agent := NewToolsTestAgent()
	registry := tools.NewToolRegistry()
	
	discoveredTools, _ := tools.DiscoverTools(agent)
	for _, tool := range discoveredTools {
		registry.Register(tool)
	}
	
	ctx := context.Background()
	weatherTool, _ := registry.Lookup("get_weather")
	
	// Execute multiple concurrent tool calls
	numCalls := 10
	results := make([][]byte, numCalls)
	errors := make([]error, numCalls)
	
	done := make(chan int, numCalls)
	
	for i := 0; i < numCalls; i++ {
		go func(index int) {
			params := WeatherParams{
				Location: fmt.Sprintf("City_%d", index),
				Units:    "celsius",
			}
			args, _ := json.Marshal(params)
			
			results[index], errors[index] = weatherTool.Call(ctx, args)
			done <- index
		}(i)
	}
	
	// Wait for all calls to complete
	for i := 0; i < numCalls; i++ {
		<-done
	}
	
	// Verify all calls succeeded
	successCount := 0
	for i := 0; i < numCalls; i++ {
		if errors[i] == nil {
			successCount++
		}
	}
	
	if successCount != numCalls {
		t.Errorf("Expected %d successful calls, got %d", numCalls, successCount)
	}
	
	// Verify all calls were logged
	if len(agent.callLog) != numCalls {
		t.Errorf("Expected %d calls logged, got %d", numCalls, len(agent.callLog))
	}
}

func TestToolSchemaGeneration(t *testing.T) {
	agent := NewToolsTestAgent()
	
	agentType := reflect.TypeOf(agent)
	var getWeatherMethod reflect.Method
	
	// Find GetWeather method
	for i := 0; i < agentType.NumMethod(); i++ {
		method := agentType.Method(i)
		if method.Name == "GetWeather" {
			getWeatherMethod = method
			break
		}
	}
	
	// Create method tool and verify schema
	tool, err := tools.NewMethodTool("get_weather", "Get weather information", getWeatherMethod, agent)
	if err != nil {
		t.Fatalf("Failed to create method tool: %v", err)
	}
	
	schema := tool.Schema()
	if schema == nil {
		t.Fatal("Tool schema is nil")
	}
	
	if schema.Type != "object" {
		t.Errorf("Expected schema type 'object', got '%s'", schema.Type)
	}
	
	if schema.Properties == nil {
		t.Fatal("Schema properties is nil")
	}
	
	// Verify required properties exist
	requiredFields := []string{"location"}
	for _, field := range requiredFields {
		if _, exists := schema.Properties[field]; !exists {
			t.Errorf("Required field '%s' not found in schema", field)
		}
	}
}

func BenchmarkToolDiscovery(b *testing.B) {
	agent := NewToolsTestAgent()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tools.DiscoverTools(agent)
	}
}

func BenchmarkToolExecution(b *testing.B) {
	agent := NewToolsTestAgent()
	registry := tools.NewToolRegistry()
	
	discoveredTools, _ := tools.DiscoverTools(agent)
	for _, tool := range discoveredTools {
		registry.Register(tool)
	}
	
	weatherTool, _ := registry.Lookup("get_weather")
	params := WeatherParams{Location: "Benchmark City", Units: "celsius"}
	args, _ := json.Marshal(params)
	ctx := context.Background()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		weatherTool.Call(ctx, args)
	}
}