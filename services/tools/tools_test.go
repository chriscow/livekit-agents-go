package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/chriscow/minds"
)

// Test fixtures for different parameter types
type SimpleParams struct {
	Name  string `json:"name"`
	Value int    `json:"value"`
}

type ComplexParams struct {
	Items       []string      `json:"items"`
	Config      *SimpleParams `json:"config,omitempty"`
	IsActive    bool          `json:"is_active"`
	Temperature float64       `json:"temperature"`
}

type EmptyParams struct{}

// Mock agent for testing tool discovery
type TestAgent struct {
	callLog    []string
	callCount  int
	shouldFail bool
}

func (a *TestAgent) SimpleMethod(ctx context.Context, params SimpleParams) (string, error) {
	a.callLog = append(a.callLog, "SimpleMethod")
	a.callCount++
	if a.shouldFail {
		return "", nil
	}
	return "result: " + params.Name, nil
}

func (a *TestAgent) ComplexMethod(ctx context.Context, params ComplexParams) (*ComplexParams, error) {
	a.callLog = append(a.callLog, "ComplexMethod")
	a.callCount++
	if a.shouldFail {
		return nil, nil
	}
	
	result := &ComplexParams{
		Items:       append(params.Items, "processed"),
		Config:      params.Config,
		IsActive:    !params.IsActive,
		Temperature: params.Temperature * 2.0,
	}
	return result, nil
}

func (a *TestAgent) NoParamsMethod(ctx context.Context) (string, error) {
	a.callLog = append(a.callLog, "NoParamsMethod")
	a.callCount++
	return "no params result", nil
}

func (a *TestAgent) NoContextMethod(params SimpleParams) (string, error) {
	a.callLog = append(a.callLog, "NoContextMethod")
	return "no context result", nil
}

func (a *TestAgent) NoReturnMethod(ctx context.Context, params SimpleParams) {
	a.callLog = append(a.callLog, "NoReturnMethod")
}

// Lifecycle methods that should be excluded
func (a *TestAgent) OnEnter() error {
	return nil
}

func (a *TestAgent) Start(ctx context.Context) error {
	return nil
}

func (a *TestAgent) privateMethod(ctx context.Context) string {
	return "private"
}

// Custom FunctionTool implementation for testing
type mockTool struct {
	name        string
	description string
	callCount   int
	shouldError bool
}

func (mt *mockTool) Name() string {
	return mt.name
}

func (mt *mockTool) Description() string {
	return mt.description
}

func (mt *mockTool) Call(ctx context.Context, args []byte) ([]byte, error) {
	mt.callCount++
	if mt.shouldError {
		return nil, nil
	}
	return []byte(`{"success": true}`), nil
}

func (mt *mockTool) Schema() *minds.Definition {
	return &minds.Definition{
		Type: "object",
		Properties: map[string]minds.Definition{
			"test": {Type: "string"},
		},
	}
}

func TestToolRegistry_Basic(t *testing.T) {
	registry := NewToolRegistry()
	
	if registry == nil {
		t.Fatal("NewToolRegistry returned nil")
	}
	
	if count := registry.Count(); count != 0 {
		t.Errorf("Expected empty registry, got count: %d", count)
	}
	
	if tools := registry.List(); len(tools) != 0 {
		t.Errorf("Expected empty tool list, got: %v", tools)
	}
	
	if names := registry.Names(); len(names) != 0 {
		t.Errorf("Expected empty names list, got: %v", names)
	}
}

func TestToolRegistry_Register(t *testing.T) {
	registry := NewToolRegistry()
	tool := &mockTool{name: "test_tool", description: "Test tool"}
	
	// Test successful registration
	err := registry.Register(tool)
	if err != nil {
		t.Fatalf("Failed to register tool: %v", err)
	}
	
	if count := registry.Count(); count != 1 {
		t.Errorf("Expected count 1, got: %d", count)
	}
	
	// Test lookup
	foundTool, exists := registry.Lookup("test_tool")
	if !exists {
		t.Error("Tool not found after registration")
	}
	if foundTool != tool {
		t.Error("Retrieved tool does not match registered tool")
	}
	
	// Test duplicate registration
	err = registry.Register(tool)
	if err == nil {
		t.Error("Expected error when registering duplicate tool")
	}
	
	// Test nil tool registration
	err = registry.Register(nil)
	if err == nil {
		t.Error("Expected error when registering nil tool")
	}
	
	// Test empty name tool
	emptyNameTool := &mockTool{name: "", description: "No name"}
	err = registry.Register(emptyNameTool)
	if err == nil {
		t.Error("Expected error when registering tool with empty name")
	}
}

func TestToolRegistry_Lookup(t *testing.T) {
	registry := NewToolRegistry()
	tool1 := &mockTool{name: "tool1", description: "First tool"}
	tool2 := &mockTool{name: "tool2", description: "Second tool"}
	
	registry.Register(tool1)
	registry.Register(tool2)
	
	// Test existing tool lookup
	foundTool, exists := registry.Lookup("tool1")
	if !exists || foundTool != tool1 {
		t.Error("Failed to lookup existing tool")
	}
	
	// Test non-existing tool lookup
	_, exists = registry.Lookup("nonexistent")
	if exists {
		t.Error("Found non-existent tool")
	}
}

func TestToolRegistry_List(t *testing.T) {
	registry := NewToolRegistry()
	tool1 := &mockTool{name: "tool1", description: "First tool"}
	tool2 := &mockTool{name: "tool2", description: "Second tool"}
	
	registry.Register(tool1)
	registry.Register(tool2)
	
	tools := registry.List()
	if len(tools) != 2 {
		t.Errorf("Expected 2 tools, got: %d", len(tools))
	}
	
	// Check if both tools are present
	found1, found2 := false, false
	for _, tool := range tools {
		if tool == tool1 {
			found1 = true
		}
		if tool == tool2 {
			found2 = true
		}
	}
	
	if !found1 || !found2 {
		t.Error("Not all registered tools found in list")
	}
}

func TestToolRegistry_Names(t *testing.T) {
	registry := NewToolRegistry()
	tool1 := &mockTool{name: "tool1", description: "First tool"}
	tool2 := &mockTool{name: "tool2", description: "Second tool"}
	
	registry.Register(tool1)
	registry.Register(tool2)
	
	names := registry.Names()
	if len(names) != 2 {
		t.Errorf("Expected 2 names, got: %d", len(names))
	}
	
	expectedNames := map[string]bool{"tool1": false, "tool2": false}
	for _, name := range names {
		if _, exists := expectedNames[name]; exists {
			expectedNames[name] = true
		}
	}
	
	for name, found := range expectedNames {
		if !found {
			t.Errorf("Name %s not found in names list", name)
		}
	}
}

func TestToolRegistry_Remove(t *testing.T) {
	registry := NewToolRegistry()
	tool := &mockTool{name: "test_tool", description: "Test tool"}
	
	registry.Register(tool)
	
	// Test successful removal
	removed := registry.Remove("test_tool")
	if !removed {
		t.Error("Failed to remove existing tool")
	}
	
	if count := registry.Count(); count != 0 {
		t.Errorf("Expected count 0 after removal, got: %d", count)
	}
	
	// Test removal of non-existing tool
	removed = registry.Remove("nonexistent")
	if removed {
		t.Error("Reported removal of non-existent tool")
	}
}

func TestToolRegistry_Clear(t *testing.T) {
	registry := NewToolRegistry()
	tool1 := &mockTool{name: "tool1", description: "First tool"}
	tool2 := &mockTool{name: "tool2", description: "Second tool"}
	
	registry.Register(tool1)
	registry.Register(tool2)
	
	if count := registry.Count(); count != 2 {
		t.Errorf("Expected 2 tools before clear, got: %d", count)
	}
	
	registry.Clear()
	
	if count := registry.Count(); count != 0 {
		t.Errorf("Expected 0 tools after clear, got: %d", count)
	}
	
	if tools := registry.List(); len(tools) != 0 {
		t.Errorf("Expected empty list after clear, got: %v", tools)
	}
}

func TestToolRegistry_Concurrency(t *testing.T) {
	registry := NewToolRegistry()
	
	// Test concurrent registration and lookup
	var wg sync.WaitGroup
	numGoroutines := 10
	
	// Concurrent registration
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			tool := &mockTool{
				name:        fmt.Sprintf("tool_%d", id),
				description: fmt.Sprintf("Tool %d", id),
			}
			registry.Register(tool)
		}(i)
	}
	
	// Concurrent lookup
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			time.Sleep(time.Millisecond) // Small delay to allow registration
			registry.Lookup(fmt.Sprintf("tool_%d", id))
		}(i)
	}
	
	wg.Wait()
	
	if count := registry.Count(); count != numGoroutines {
		t.Errorf("Expected %d tools after concurrent registration, got: %d", numGoroutines, count)
	}
}

func TestMethodTool_Creation(t *testing.T) {
	agent := &TestAgent{}
	agentType := reflect.TypeOf(agent)
	
	// Find SimpleMethod
	var simpleMethod reflect.Method
	found := false
	for i := 0; i < agentType.NumMethod(); i++ {
		method := agentType.Method(i)
		if method.Name == "SimpleMethod" {
			simpleMethod = method
			found = true
			break
		}
	}
	
	if !found {
		t.Fatal("SimpleMethod not found")
	}
	
	// Test successful creation
	tool, err := NewMethodTool("simple", "Simple test method", simpleMethod, agent)
	if err != nil {
		t.Fatalf("Failed to create method tool: %v", err)
	}
	
	if tool.Name() != "simple" {
		t.Errorf("Expected name 'simple', got: %s", tool.Name())
	}
	
	if tool.Description() != "Simple test method" {
		t.Errorf("Expected description 'Simple test method', got: %s", tool.Description())
	}
	
	if tool.Schema() == nil {
		t.Error("Expected schema to be generated")
	}
	
	// Test creation with nil receiver
	_, err = NewMethodTool("simple", "Simple test method", simpleMethod, nil)
	if err == nil {
		t.Error("Expected error when creating tool with nil receiver")
	}
}

func TestMethodTool_Call(t *testing.T) {
	agent := &TestAgent{}
	agentType := reflect.TypeOf(agent)
	
	// Find SimpleMethod
	var simpleMethod reflect.Method
	for i := 0; i < agentType.NumMethod(); i++ {
		method := agentType.Method(i)
		if method.Name == "SimpleMethod" {
			simpleMethod = method
			break
		}
	}
	
	tool, err := NewMethodTool("simple", "Simple test method", simpleMethod, agent)
	if err != nil {
		t.Fatalf("Failed to create method tool: %v", err)
	}
	
	// Test successful call
	params := SimpleParams{Name: "test", Value: 42}
	argsData, _ := json.Marshal(params)
	
	ctx := context.Background()
	result, err := tool.Call(ctx, argsData)
	if err != nil {
		t.Fatalf("Tool call failed: %v", err)
	}
	
	var resultStr string
	err = json.Unmarshal(result, &resultStr)
	if err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}
	
	expected := "result: test"
	if resultStr != expected {
		t.Errorf("Expected result '%s', got: '%s'", expected, resultStr)
	}
	
	// Verify method was called
	if agent.callCount != 1 {
		t.Errorf("Expected 1 method call, got: %d", agent.callCount)
	}
	
	if len(agent.callLog) != 1 || agent.callLog[0] != "SimpleMethod" {
		t.Errorf("Expected call log ['SimpleMethod'], got: %v", agent.callLog)
	}
}

func TestMethodTool_CallComplexParams(t *testing.T) {
	agent := &TestAgent{}
	agentType := reflect.TypeOf(agent)
	
	// Find ComplexMethod
	var complexMethod reflect.Method
	for i := 0; i < agentType.NumMethod(); i++ {
		method := agentType.Method(i)
		if method.Name == "ComplexMethod" {
			complexMethod = method
			break
		}
	}
	
	tool, err := NewMethodTool("complex", "Complex test method", complexMethod, agent)
	if err != nil {
		t.Fatalf("Failed to create method tool: %v", err)
	}
	
	// Test with complex parameters
	params := ComplexParams{
		Items:       []string{"item1", "item2"},
		Config:      &SimpleParams{Name: "config", Value: 100},
		IsActive:    true,
		Temperature: 25.5,
	}
	argsData, _ := json.Marshal(params)
	
	ctx := context.Background()
	result, err := tool.Call(ctx, argsData)
	if err != nil {
		t.Fatalf("Tool call failed: %v", err)
	}
	
	var resultParams ComplexParams
	err = json.Unmarshal(result, &resultParams)
	if err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}
	
	// Verify transformation
	if len(resultParams.Items) != 3 || resultParams.Items[2] != "processed" {
		t.Errorf("Expected items to be processed, got: %v", resultParams.Items)
	}
	
	if resultParams.IsActive != false {
		t.Error("Expected IsActive to be flipped")
	}
	
	if resultParams.Temperature != 51.0 {
		t.Errorf("Expected temperature 51.0, got: %f", resultParams.Temperature)
	}
}

func TestMethodTool_CallNoParams(t *testing.T) {
	agent := &TestAgent{}
	agentType := reflect.TypeOf(agent)
	
	// Find NoParamsMethod
	var noParamsMethod reflect.Method
	for i := 0; i < agentType.NumMethod(); i++ {
		method := agentType.Method(i)
		if method.Name == "NoParamsMethod" {
			noParamsMethod = method
			break
		}
	}
	
	tool, err := NewMethodTool("no_params", "No params test method", noParamsMethod, agent)
	if err != nil {
		t.Fatalf("Failed to create method tool: %v", err)
	}
	
	// Test call with no parameters
	ctx := context.Background()
	result, err := tool.Call(ctx, nil)
	if err != nil {
		t.Fatalf("Tool call failed: %v", err)
	}
	
	var resultStr string
	err = json.Unmarshal(result, &resultStr)
	if err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}
	
	expected := "no params result"
	if resultStr != expected {
		t.Errorf("Expected result '%s', got: '%s'", expected, resultStr)
	}
}

func TestDiscoverTools(t *testing.T) {
	agent := &TestAgent{}
	
	tools, err := DiscoverTools(agent)
	if err != nil {
		t.Fatalf("Failed to discover tools: %v", err)
	}
	
	// Verify discovered tools
	expectedTools := map[string]bool{
		"simple_method":     false,
		"complex_method":    false,
		"no_params_method":  false,
		"no_context_method": false,
		"no_return_method":  false,
	}
	
	for _, tool := range tools {
		name := tool.Name()
		if _, exists := expectedTools[name]; exists {
			expectedTools[name] = true
		}
	}
	
	// Verify all expected tools were found (except no_context_method which should be filtered out)
	for toolName, found := range expectedTools {
		if toolName == "no_context_method" {
			if found {
				t.Errorf("Tool %s should not be discovered (no context parameter)", toolName)
			}
		} else if !found {
			t.Errorf("Expected tool %s not discovered", toolName)
		}
	}
	
	// Verify lifecycle methods are excluded
	for _, tool := range tools {
		name := tool.Name()
		if name == "on_enter" || name == "start" {
			t.Errorf("Lifecycle method %s should not be discovered as tool", name)
		}
	}
	
	// Verify private methods are excluded
	for _, tool := range tools {
		name := tool.Name()
		if name == "private_method" {
			t.Error("Private method should not be discovered as tool")
		}
	}
}

func TestDiscoverTools_InvalidInputs(t *testing.T) {
	// Test nil agent
	_, err := DiscoverTools(nil)
	if err == nil {
		t.Error("Expected error when discovering tools from nil agent")
	}
	
	// Test non-pointer agent
	agent := TestAgent{}
	_, err = DiscoverTools(agent)
	if err == nil {
		t.Error("Expected error when discovering tools from non-pointer agent")
	}
}

func TestToSnakeCase(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"SimpleMethod", "simple_method"},
		{"ComplexMethodName", "complex_method_name"},
		{"GetWeather", "get_weather"},
		{"XMLHttpRequest", "x_m_l_http_request"},
		{"lowercase", "lowercase"},
		{"UPPERCASE", "u_p_p_e_r_c_a_s_e"},
		{"", ""},
	}
	
	for _, test := range tests {
		result := toSnakeCase(test.input)
		if result != test.expected {
			t.Errorf("toSnakeCase(%s) = %s, expected %s", test.input, result, test.expected)
		}
	}
}

func TestIsLifecycleMethod(t *testing.T) {
	lifecycleMethods := []string{
		"OnEnter", "OnExit", "OnUserTurnCompleted",
		"UpdateInstructions", "UpdateTools", "UpdateChatContext",
		"Start", "Stop", "GetInstructions", "GetTools",
	}
	
	for _, method := range lifecycleMethods {
		if !isLifecycleMethod(method) {
			t.Errorf("Method %s should be recognized as lifecycle method", method)
		}
	}
	
	regularMethods := []string{
		"SimpleMethod", "GetWeather", "ProcessData", "Calculate",
	}
	
	for _, method := range regularMethods {
		if isLifecycleMethod(method) {
			t.Errorf("Method %s should not be recognized as lifecycle method", method)
		}
	}
}

func TestMethodTool_IntegrationWithRegistry(t *testing.T) {
	agent := &TestAgent{}
	registry := NewToolRegistry()
	
	// Discover and register tools
	tools, err := DiscoverTools(agent)
	if err != nil {
		t.Fatalf("Failed to discover tools: %v", err)
	}
	
	for _, tool := range tools {
		err = registry.Register(tool)
		if err != nil {
			t.Fatalf("Failed to register tool %s: %v", tool.Name(), err)
		}
	}
	
	// Test lookup and execution
	tool, exists := registry.Lookup("simple_method")
	if !exists {
		t.Fatal("simple_method not found in registry")
	}
	
	params := SimpleParams{Name: "integration_test", Value: 99}
	argsData, _ := json.Marshal(params)
	
	ctx := context.Background()
	result, err := tool.Call(ctx, argsData)
	if err != nil {
		t.Fatalf("Tool call failed: %v", err)
	}
	
	var resultStr string
	err = json.Unmarshal(result, &resultStr)
	if err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}
	
	expected := "result: integration_test"
	if resultStr != expected {
		t.Errorf("Expected result '%s', got: '%s'", expected, resultStr)
	}
	
	// Verify the registry contains expected tools
	allTools := registry.List()
	if len(allTools) == 0 {
		t.Error("No tools found in registry after discovery and registration")
	}
	
	names := registry.Names()
	if len(names) == 0 {
		t.Error("No tool names found in registry")
	}
}

func BenchmarkToolRegistry_Register(b *testing.B) {
	registry := NewToolRegistry()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tool := &mockTool{
			name:        fmt.Sprintf("tool_%d", i),
			description: "Benchmark tool",
		}
		registry.Register(tool)
	}
}

func BenchmarkToolRegistry_Lookup(b *testing.B) {
	registry := NewToolRegistry()
	
	// Pre-populate registry
	for i := 0; i < 1000; i++ {
		tool := &mockTool{
			name:        fmt.Sprintf("tool_%d", i),
			description: "Benchmark tool",
		}
		registry.Register(tool)
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		registry.Lookup(fmt.Sprintf("tool_%d", i%1000))
	}
}

func BenchmarkMethodTool_Call(b *testing.B) {
	agent := &TestAgent{}
	agentType := reflect.TypeOf(agent)
	
	var simpleMethod reflect.Method
	for i := 0; i < agentType.NumMethod(); i++ {
		method := agentType.Method(i)
		if method.Name == "SimpleMethod" {
			simpleMethod = method
			break
		}
	}
	
	tool, _ := NewMethodTool("simple", "Simple test method", simpleMethod, agent)
	
	params := SimpleParams{Name: "benchmark", Value: 42}
	argsData, _ := json.Marshal(params)
	ctx := context.Background()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tool.Call(ctx, argsData)
	}
}

func BenchmarkDiscoverTools(b *testing.B) {
	agent := &TestAgent{}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		DiscoverTools(agent)
	}
}