package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/chriscow/minds"
)

// MethodTool wraps a Go method as a FunctionTool using reflection
type MethodTool struct {
	name        string
	description string
	method      reflect.Method
	receiver    reflect.Value
	schema      *minds.Definition
}

// NewMethodTool creates a function tool from a method using reflection
func NewMethodTool(name, description string, method reflect.Method, receiver interface{}) (*MethodTool, error) {
	if receiver == nil {
		return nil, fmt.Errorf("receiver cannot be nil")
	}
	
	receiverValue := reflect.ValueOf(receiver)
	if !receiverValue.IsValid() {
		return nil, fmt.Errorf("invalid receiver")
	}
	
	// Validate method signature
	methodType := method.Type
	if methodType.NumIn() < 1 {
		return nil, fmt.Errorf("method must have at least one parameter (receiver)")
	}
	
	// Check if method has context.Context as first parameter (after receiver)
	if methodType.NumIn() > 1 {
		firstParam := methodType.In(1)
		if firstParam != reflect.TypeOf((*context.Context)(nil)).Elem() {
			return nil, fmt.Errorf("first parameter must be context.Context")
		}
	}
	
	// Generate schema for method parameters
	schema, err := generateMethodSchema(method)
	if err != nil {
		return nil, fmt.Errorf("failed to generate schema: %v", err)
	}
	
	return &MethodTool{
		name:        name,
		description: description,
		method:      method,
		receiver:    receiverValue,
		schema:      schema,
	}, nil
}

func (mt *MethodTool) Name() string {
	return mt.name
}

func (mt *MethodTool) Description() string {
	return mt.description
}

func (mt *MethodTool) Schema() *minds.Definition {
	return mt.schema
}

func (mt *MethodTool) Call(ctx context.Context, args []byte) ([]byte, error) {
	methodType := mt.method.Type
	
	// Prepare input values
	inputs := []reflect.Value{mt.receiver}
	
	// Add context if method expects it
	if methodType.NumIn() > 1 {
		inputs = append(inputs, reflect.ValueOf(ctx))
	}
	
	// If method expects parameters beyond context, unmarshal args
	if methodType.NumIn() > 2 {
		// Check if single struct parameter or multiple individual parameters
		if methodType.NumIn() == 3 && methodType.In(2).Kind() == reflect.Struct {
			// Single struct parameter - unmarshal directly
			paramType := methodType.In(2)
			paramValue := reflect.New(paramType).Interface()
			
			if len(args) > 0 {
				if err := json.Unmarshal(args, paramValue); err != nil {
					return nil, fmt.Errorf("failed to unmarshal arguments: %v", err)
				}
			}
			
			inputs = append(inputs, reflect.ValueOf(paramValue).Elem())
		} else {
			// Multiple individual parameters - unmarshal from synthetic object
			if err := mt.unmarshalMultipleParams(args, methodType, &inputs); err != nil {
				return nil, err
			}
		}
	}
	
	// Call the method
	results := mt.method.Func.Call(inputs)
	
	// Handle return values
	if len(results) == 0 {
		return []byte(`{}`), nil
	}
	
	// Check for error (assume last return value is error if present)
	if len(results) > 1 {
		if errValue := results[len(results)-1]; !errValue.IsNil() {
			if err, ok := errValue.Interface().(error); ok {
				return nil, err
			}
		}
	}
	
	// Marshal first return value as JSON
	if results[0].IsValid() && !results[0].IsZero() {
		result, err := json.Marshal(results[0].Interface())
		if err != nil {
			return nil, fmt.Errorf("failed to marshal result: %v", err)
		}
		return result, nil
	}
	
	return []byte(`{}`), nil
}

// unmarshalMultipleParams unmarshals JSON args to individual method parameters
func (mt *MethodTool) unmarshalMultipleParams(args []byte, methodType reflect.Type, inputs *[]reflect.Value) error {
	if len(args) == 0 {
		// No arguments provided - add zero values for all parameters
		for i := 2; i < methodType.NumIn(); i++ {
			paramType := methodType.In(i)
			*inputs = append(*inputs, reflect.Zero(paramType))
		}
		return nil
	}
	
	// Unmarshal to map to get individual parameter values
	var argsMap map[string]interface{}
	if err := json.Unmarshal(args, &argsMap); err != nil {
		return fmt.Errorf("failed to unmarshal args to map: %v", err)
	}
	
	// Convert map values to method parameters
	for i := 2; i < methodType.NumIn(); i++ {
		paramType := methodType.In(i)
		paramName := fmt.Sprintf("param%d", i-1)
		
		rawValue, exists := argsMap[paramName]
		if !exists {
			// Parameter not provided - use zero value
			*inputs = append(*inputs, reflect.Zero(paramType))
			continue
		}
		
		// Convert interface{} value to proper type
		paramValue, err := convertToType(rawValue, paramType)
		if err != nil {
			return fmt.Errorf("failed to convert param %s: %v", paramName, err)
		}
		
		*inputs = append(*inputs, paramValue)
	}
	
	return nil
}

// convertToType converts an interface{} value to the target reflect.Type
func convertToType(value interface{}, targetType reflect.Type) (reflect.Value, error) {
	if value == nil {
		return reflect.Zero(targetType), nil
	}
	
	sourceValue := reflect.ValueOf(value)
	
	// If types match directly, return as-is
	if sourceValue.Type() == targetType {
		return sourceValue, nil
	}
	
	// Handle string conversions
	if targetType.Kind() == reflect.String {
		if str, ok := value.(string); ok {
			return reflect.ValueOf(str), nil
		}
		// Convert other types to string
		return reflect.ValueOf(fmt.Sprintf("%v", value)), nil
	}
	
	// Handle numeric conversions
	switch targetType.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if num, ok := value.(float64); ok { // JSON unmarshals numbers as float64
			return reflect.ValueOf(int64(num)).Convert(targetType), nil
		}
	case reflect.Float32, reflect.Float64:
		if num, ok := value.(float64); ok {
			return reflect.ValueOf(num).Convert(targetType), nil
		}
	case reflect.Bool:
		if b, ok := value.(bool); ok {
			return reflect.ValueOf(b), nil
		}
	}
	
	// Try to convert directly
	if sourceValue.Type().ConvertibleTo(targetType) {
		return sourceValue.Convert(targetType), nil
	}
	
	return reflect.Zero(targetType), fmt.Errorf("cannot convert %T to %s", value, targetType)
}

// DiscoverTools finds all methods on the given interface that can be used as tools
func DiscoverTools(agent interface{}) ([]*MethodTool, error) {
	if agent == nil {
		return nil, fmt.Errorf("agent cannot be nil")
	}
	
	agentType := reflect.TypeOf(agent)
	if agentType.Kind() != reflect.Ptr {
		return nil, fmt.Errorf("agent must be a pointer to a struct")
	}
	
	var tools []*MethodTool
	
	// Iterate through all methods
	for i := 0; i < agentType.NumMethod(); i++ {
		method := agentType.Method(i)
		
		// Check if method is exported and follows tool naming convention
		if !method.IsExported() {
			continue
		}
		
		// Look for methods that could be tools (skip lifecycle methods)
		if isLifecycleMethod(method.Name) {
			continue
		}
		
		// Extract description from method (this would ideally come from docstrings/tags)
		description := fmt.Sprintf("Tool function: %s", method.Name)
		
		// Create tool name (convert CamelCase to snake_case)
		toolName := toSnakeCase(method.Name)
		
		tool, err := NewMethodTool(toolName, description, method, agent)
		if err != nil {
			// Skip methods that can't be converted to tools
			continue
		}
		
		tools = append(tools, tool)
	}
	
	return tools, nil
}

// generateMethodSchema generates a JSON schema for a method's parameters
func generateMethodSchema(method reflect.Method) (*minds.Definition, error) {
	methodType := method.Type
	
	// If method only has receiver (no context, no parameters), create minimal schema
	if methodType.NumIn() == 1 {
		return &minds.Definition{
			Type: "object",
			Properties: map[string]minds.Definition{
				"_dummy": {
					Type: "null",
					Description: "No parameters required",
				},
			},
		}, nil
	}
	
	// If method only has receiver and context, no other parameters
	if methodType.NumIn() == 2 {
		return &minds.Definition{
			Type: "object", 
			Properties: map[string]minds.Definition{
				"_dummy": {
					Type: "null",
					Description: "No parameters required",
				},
			},
		}, nil
	}
	
	// Check if method has a single struct parameter (preferred pattern)
	if methodType.NumIn() == 3 {
		paramType := methodType.In(2)
		if paramType.Kind() == reflect.Struct {
			// Use the struct directly for schema generation
			paramValue := reflect.New(paramType).Interface()
			return minds.GenerateSchema(paramValue)
		}
	}
	
	// Handle multiple individual parameters by creating a synthetic struct
	return generateSyntheticStructSchema(method)
}

// generateSyntheticStructSchema creates an object schema from multiple method parameters
func generateSyntheticStructSchema(method reflect.Method) (*minds.Definition, error) {
	methodType := method.Type
	
	properties := make(map[string]minds.Definition)
	required := make([]string, 0)
	
	// Start from parameter 2 (skip receiver and context)
	for i := 2; i < methodType.NumIn(); i++ {
		paramType := methodType.In(i)
		paramName := fmt.Sprintf("param%d", i-1) // param1, param2, etc.
		
		// Generate schema for individual parameter
		var paramSchema minds.Definition
		switch paramType.Kind() {
		case reflect.String:
			paramSchema = minds.Definition{
				Type: "string",
				Description: fmt.Sprintf("Parameter %d for %s", i-1, method.Name),
			}
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			paramSchema = minds.Definition{
				Type: "integer",
				Description: fmt.Sprintf("Parameter %d for %s", i-1, method.Name),
			}
		case reflect.Float32, reflect.Float64:
			paramSchema = minds.Definition{
				Type: "number",
				Description: fmt.Sprintf("Parameter %d for %s", i-1, method.Name),
			}
		case reflect.Bool:
			paramSchema = minds.Definition{
				Type: "boolean", 
				Description: fmt.Sprintf("Parameter %d for %s", i-1, method.Name),
			}
		default:
			// For complex types, try to generate schema
			paramValue := reflect.New(paramType).Interface()
			schema, err := minds.GenerateSchema(paramValue)
			if err != nil {
				return nil, fmt.Errorf("failed to generate schema for parameter %d: %w", i-1, err)
			}
			paramSchema = *schema
		}
		
		properties[paramName] = paramSchema
		required = append(required, paramName)
	}
	
	return &minds.Definition{
		Type:       "object",
		Properties: properties,
		Required:   required,
	}, nil
}

// isLifecycleMethod checks if a method is a lifecycle method that should not be exposed as a tool
func isLifecycleMethod(name string) bool {
	lifecycleMethods := []string{
		"OnEnter", "OnExit", "OnUserTurnCompleted",
		"OnAudioFrame", "OnSpeechDetected", "OnSpeechEnded",
		"UpdateInstructions", "UpdateTools", "UpdateChatContext", 
		"Start", "Stop", "GetInstructions", "GetTools",
		"HandleEvent", "Name", "SetMetadata", "GetMetadata",
	}
	
	for _, lifecycle := range lifecycleMethods {
		if name == lifecycle {
			return true
		}
	}
	return false
}

// toSnakeCase converts CamelCase to snake_case
func toSnakeCase(s string) string {
	var result strings.Builder
	
	for i, r := range s {
		if i > 0 && 'A' <= r && r <= 'Z' {
			result.WriteRune('_')
		}
		result.WriteRune(r)
	}
	
	return strings.ToLower(result.String())
}