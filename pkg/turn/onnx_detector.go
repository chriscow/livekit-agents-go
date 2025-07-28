package turn

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/sugarme/tokenizer"
	"github.com/sugarme/tokenizer/pretrained"
	ort "github.com/yalue/onnxruntime_go"

	"github.com/chriscow/livekit-agents-go/pkg/ai/llm"
	"github.com/chriscow/livekit-agents-go/pkg/turn/internal"
)

const (
	// modelFileRel is the relative path to the ONNX model file within the model directory
	modelFileRel = "onnx/model_q8.onnx"
)

// ONNXDetector implements turn detection using ONNX models.
type ONNXDetector struct {
	modelInfo   internal.ModelInfo
	modelPath   string
	sessionOnce sync.Once
	session     *ort.Session[float32]
	sessionErr  error

	// Tokenizer
	tokenizer     *tokenizer.Tokenizer
	tokenizerOnce sync.Once
	tokenizerErr  error

	// Languages thresholds loaded from languages.json
	languages     map[string]float64
	languagesOnce sync.Once
	languagesErr  error
}

// NewONNXDetector creates a new ONNX-based turn detector.
func NewONNXDetector(modelName, modelPath string) (*ONNXDetector, error) {
	var modelInfo internal.ModelInfo
	found := false

	for _, model := range internal.AllModels {
		if model.Name == modelName {
			modelInfo = model
			found = true
			break
		}
	}

	if !found {
		return nil, fmt.Errorf("unknown model: %s", modelName)
	}

	if modelPath == "" {
		modelPath = getDefaultModelPath()
	}

	return &ONNXDetector{
		modelInfo: modelInfo,
		modelPath: modelPath,
	}, nil
}

// UnlikelyThreshold returns the language-specific threshold for EOU detection.
func (d *ONNXDetector) UnlikelyThreshold(language string) (float64, error) {
	if err := d.loadLanguages(); err != nil {
		return 0, err
	}
	threshold, exists := d.languages[language]
	if !exists {
		return 0, fmt.Errorf("unsupported language: %s", language)
	}
	return threshold, nil
}

// SupportsLanguage returns true if the detector has a tuned threshold for this language.
func (d *ONNXDetector) SupportsLanguage(language string) bool {
	if err := d.loadLanguages(); err != nil {
		return false
	}
	_, exists := d.languages[language]
	return exists
}

// PredictEndOfTurn returns probability (0–1) that the user has finished speaking.
func (d *ONNXDetector) PredictEndOfTurn(ctx context.Context, chatCtx ChatContext) (float64, error) {
	startTime := time.Now()

	// Load ONNX session lazily
	if err := d.loadSession(); err != nil {
		return 0, fmt.Errorf("failed to load ONNX session: %w", err)
	}

	// Load tokenizer lazily
	if err := d.loadTokenizer(); err != nil {
		return 0, fmt.Errorf("failed to load tokenizer: %w", err)
	}

	// Convert chat context to tokens with chat template
	tokens, err := d.tokenizeChat(chatCtx)
	if err != nil {
		return 0, fmt.Errorf("tokenization failed: %w", err)
	}

	// Run ONNX inference
	probability, err := d.runInference(ctx, tokens)
	if err != nil {
		return 0, fmt.Errorf("inference failed: %w", err)
	}

	// Log inference latency
	latency := time.Since(startTime)
	if latency > 25*time.Millisecond {
		fmt.Printf("Turn detection inference took %v (target: <25ms)\n", latency)
	}

	return probability, nil
}

// loadSession initializes the ONNX session with proper settings.
func (d *ONNXDetector) loadSession() error {
	d.sessionOnce.Do(func() {
		// Get model file path
		modelFile := internal.GetModelFilePath(d.modelPath, d.modelInfo.Revision, modelFileRel)

		// Check if model file exists
		if _, err := os.Stat(modelFile); os.IsNotExist(err) {
			d.sessionErr = fmt.Errorf("model file not found: %s (run 'lk-go turn download-models' first)", modelFile)
			return
		}

		// Initialize ONNX runtime environment (singleton)
		if err := ensureOrtEnv(); err != nil {
			d.sessionErr = fmt.Errorf("failed to initialize ONNX runtime: %w", err)
			return
		}

		// Create session options with CPU provider settings
		options, err := ort.NewSessionOptions()
		if err != nil {
			d.sessionErr = fmt.Errorf("failed to create session options: %w", err)
			return
		}
		defer options.Destroy()

		// Configure CPU execution provider with thread settings
		intraOpThreads := max(1, runtime.NumCPU()/2)
		interOpThreads := 1

		err = options.SetIntraOpNumThreads(intraOpThreads)
		if err != nil {
			d.sessionErr = fmt.Errorf("failed to set intra-op threads: %w", err)
			return
		}

		err = options.SetInterOpNumThreads(interOpThreads)
		if err != nil {
			d.sessionErr = fmt.Errorf("failed to set inter-op threads: %w", err)
			return
		}
		// session.dynamic_block_base = 4
		if err := options.AddSessionConfigEntry("session.dynamic_block_base", "4"); err != nil {
			d.sessionErr = fmt.Errorf("failed to set session.dynamic_block_base: %w", err)
			return
		}

		// Create the session using the simpler Session API
		inputNames := []string{"input_ids"}
		outputNames := []string{"logits"}

		// We'll create a dummy input tensor for session creation
		dummyShape := ort.NewShape(1, 1)
		dummyData := []int64{0}
		dummyInput, err := ort.NewTensor(dummyShape, dummyData)
		if err != nil {
			d.sessionErr = fmt.Errorf("failed to create dummy input tensor: %w", err)
			return
		}
		defer dummyInput.Destroy()

		// Create dummy output tensor
		dummyOutputShape := ort.NewShape(1, 1)
		dummyOutput, err := ort.NewEmptyTensor[float32](dummyOutputShape)
		if err != nil {
			d.sessionErr = fmt.Errorf("failed to create dummy output tensor: %w", err)
			return
		}
		defer dummyOutput.Destroy()

		// For Session creation, both input and output tensors must be the same type
		// Let's create float32 tensors for both input and output
		dummyInputFloat32 := []float32{0}
		dummyInputTensorFloat32, err := ort.NewTensor(dummyShape, dummyInputFloat32)
		if err != nil {
			d.sessionErr = fmt.Errorf("failed to create dummy float32 input tensor: %w", err)
			return
		}
		defer dummyInputTensorFloat32.Destroy()

		// Create the session with float32 tensors
		d.session, err = ort.NewSession[float32](
			modelFile,
			inputNames,
			outputNames,
			[]*ort.Tensor[float32]{dummyInputTensorFloat32},
			[]*ort.Tensor[float32]{dummyOutput},
		)
		if err != nil {
			d.sessionErr = fmt.Errorf("failed to create ONNX session: %w", err)
			return
		}
	})

	return d.sessionErr
}

// loadTokenizer loads the HuggingFace tokenizer from tokenizer.json.
func (d *ONNXDetector) loadTokenizer() error {
	d.tokenizerOnce.Do(func() {
		tokenizerFile := internal.GetModelFilePath(d.modelPath, d.modelInfo.Revision, "tokenizer.json")

		// Check if tokenizer file exists
		if _, err := os.Stat(tokenizerFile); os.IsNotExist(err) {
			d.tokenizerErr = fmt.Errorf("tokenizer file not found: %s (run 'lk-go turn download-models' first)", tokenizerFile)
			return
		}

		// Load the tokenizer from the JSON file
		tk, err := pretrained.FromFile(tokenizerFile)
		if err != nil {
			d.tokenizerErr = fmt.Errorf("failed to load tokenizer: %w", err)
			return
		}

		d.tokenizer = tk
	})

	return d.tokenizerErr
}

// loadLanguages parses languages.json once and caches the thresholds.
func (d *ONNXDetector) loadLanguages() error {
	d.languagesOnce.Do(func() {
		langFile := internal.GetModelFilePath(d.modelPath, d.modelInfo.Revision, "languages.json")
		file, err := os.Open(langFile)
		if err != nil {
			d.languagesErr = fmt.Errorf("failed to open languages.json: %w", err)
			return
		}
		defer file.Close()

		var cfg map[string]float64
		if err := json.NewDecoder(file).Decode(&cfg); err != nil {
			d.languagesErr = fmt.Errorf("failed to decode languages.json: %w", err)
			return
		}
		d.languages = cfg
	})
	return d.languagesErr
}

// tokenizeChat converts chat context to tokens using the model's tokenizer.
func (d *ONNXDetector) tokenizeChat(chatCtx ChatContext) ([]int32, error) {
	// Apply chat template with <|im_start|> ... <|im_end|> markers
	chatText := d.formatChatForTokenization(chatCtx.Messages)

	// Tokenize the formatted text
	encoding, err := d.tokenizer.EncodeSingle(chatText, false)
	if err != nil {
		return nil, fmt.Errorf("tokenization failed: %w", err)
	}

	// Get token IDs and limit to 128 tokens with left truncation
	tokenIds := encoding.GetIds()

	// Left-truncate to 128 tokens if needed
	if len(tokenIds) > 128 {
		tokenIds = tokenIds[len(tokenIds)-128:]
	}

	// Convert to int32 for ONNX
	result := make([]int32, len(tokenIds))
	for i, id := range tokenIds {
		result[i] = int32(id)
	}

	return result, nil
}

// formatChatForTokenization formats messages for the tokenizer with proper chat template.
// Uses the exact template from the LiveKit model: <|im_start|><|role|>content<|im_end|>
func (d *ONNXDetector) formatChatForTokenization(messages []llm.Message) string {
	// Keep only the most recent 6 turns for efficiency
	recentMessages := messages
	if len(messages) > 6 {
		recentMessages = messages[len(messages)-6:]
	}

	var formatted string
	for _, msg := range recentMessages {
		// Use the exact template format from the model config:
		// {% for message in messages %}{{'<|im_start|>' + '<|' + message['role'] + '|>' + message['content'] + '<|im_end|>'}}{% endfor %}
		formatted += fmt.Sprintf("<|im_start|><|%s|>%s<|im_end|>", string(msg.Role), msg.Content)
	}

	return formatted
}

// runInference executes the ONNX model and returns the EOU probability.
func (d *ONNXDetector) runInference(ctx context.Context, tokens []int32) (float64, error) {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	default:
	}

	// Create input tensor for input_ids
	seqLen := len(tokens)
	if seqLen == 0 {
		return 0.5, nil // Neutral probability for empty input
	}

	// The Session approach requires pre-allocated tensors, but we need to handle dynamic input sizes
	// For this use case, let's switch to a simpler approach by recreating the session for each call
	// or use AdvancedSession which is more flexible

	// For now, let's use a different approach: create new tensors for each inference
	// Convert tokens to float32 for consistency with session type
	inputData := make([]float32, seqLen)
	for i, token := range tokens {
		inputData[i] = float32(token)
	}

	// Create input tensor with shape [1, seqLen]
	inputShape := ort.NewShape(1, int64(seqLen))
	inputTensor, err := ort.NewTensor(inputShape, inputData)
	if err != nil {
		return 0, fmt.Errorf("failed to create input tensor: %w", err)
	}
	defer inputTensor.Destroy()

	// Create output tensor for results - assuming single probability output
	outputShape := ort.NewShape(1, 1)
	outputTensor, err := ort.NewEmptyTensor[float32](outputShape)
	if err != nil {
		return 0, fmt.Errorf("failed to create output tensor: %w", err)
	}
	defer outputTensor.Destroy()

	// Create a new session for this inference with the correct tensor sizes
	// This is not efficient but works around the fixed-size limitation
	inputNames := []string{"input_ids"}
	outputNames := []string{"logits"}

	sessionOptions, err := ort.NewSessionOptions()
	if err != nil {
		return 0, fmt.Errorf("failed to create session options: %w", err)
	}
	defer sessionOptions.Destroy()

	modelFile := internal.GetModelFilePath(d.modelPath, d.modelInfo.Revision, modelFileRel)
	tempSession, err := ort.NewSession[float32](
		modelFile,
		inputNames,
		outputNames,
		[]*ort.Tensor[float32]{inputTensor},
		[]*ort.Tensor[float32]{outputTensor},
	)
	if err != nil {
		return 0, fmt.Errorf("failed to create temp session: %w", err)
	}
	defer tempSession.Destroy()

	// Run inference
	err = tempSession.Run()
	if err != nil {
		return 0, fmt.Errorf("ONNX inference failed: %w", err)
	}

	// Extract the probability from the output tensor
	outputData := outputTensor.GetData()

	// The output data is []float32 from GetData()
	if len(outputData) == 0 {
		return 0, fmt.Errorf("empty output tensor")
	}

	// Clamp probability to [0, 1] range
	prob := float64(outputData[0])
	if prob < 0 {
		prob = 0
	} else if prob > 1 {
		prob = 1
	}

	return prob, nil
}

// getDefaultModelPath returns the default path for storing models.
func getDefaultModelPath() string {
	if path := os.Getenv("LK_MODEL_PATH"); path != "" {
		return path
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "/tmp/livekit-models" // Fallback
	}

	return filepath.Join(homeDir, ".livekit", "models")
}

// Helper functions
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
