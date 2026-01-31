package output

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
)

func TestNewJSONOutput(t *testing.T) {
	// Test with indentation
	out := NewJSONOutput(true)
	if out == nil {
		t.Error("NewJSONOutput(true) returned nil")
	}
	if !out.indent {
		t.Error("NewJSONOutput(true).indent should be true")
	}

	// Test without indentation
	out = NewJSONOutput(false)
	if out == nil {
		t.Error("NewJSONOutput(false) returned nil")
	}
	if out.indent {
		t.Error("NewJSONOutput(false).indent should be false")
	}
}

func TestJSONOutput_Print(t *testing.T) {
	tests := []struct {
		name     string
		indent   bool
		data     interface{}
		wantErr  bool
		validate func(string) bool
	}{
		{
			name:    "simple map without indent",
			indent:  false,
			data:    map[string]string{"key": "value"},
			wantErr: false,
			validate: func(s string) bool {
				return !strings.Contains(s, "\n") && strings.Contains(s, `"key":"value"`)
			},
		},
		{
			name:    "simple map with indent",
			indent:  true,
			data:    map[string]string{"key": "value"},
			wantErr: false,
			validate: func(s string) bool {
				return strings.Contains(s, "\n") && strings.Contains(s, `  "key"`)
			},
		},
		{
			name:   "nested structure",
			indent: true,
			data: map[string]interface{}{
				"success": true,
				"data": map[string]string{
					"message": "hello",
				},
			},
			wantErr: false,
			validate: func(s string) bool {
				var result map[string]interface{}
				return json.Unmarshal([]byte(s), &result) == nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture stdout
			old := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			out := NewJSONOutput(tt.indent)
			err := out.Print(tt.data)

			w.Close()
			os.Stdout = old

			var buf bytes.Buffer
			io.Copy(&buf, r)
			result := strings.TrimSpace(buf.String())

			if (err != nil) != tt.wantErr {
				t.Errorf("Print() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.validate != nil && !tt.validate(result) {
				t.Errorf("Print() output validation failed: %s", result)
			}
		})
	}
}

func TestJSONOutput_PrintError(t *testing.T) {
	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	out := NewJSONOutput(false)
	testErr := errors.New("test error message")
	err := out.PrintError(testErr)

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	result := strings.TrimSpace(buf.String())

	if err != nil {
		t.Errorf("PrintError() unexpected error = %v", err)
	}

	var output map[string]interface{}
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Errorf("PrintError() output is not valid JSON: %v", err)
	}

	if success, ok := output["success"].(bool); !ok || success {
		t.Error("PrintError() success field should be false")
	}

	if errMsg, ok := output["error"].(string); !ok || errMsg != "test error message" {
		t.Errorf("PrintError() error message = %v, want %v", errMsg, "test error message")
	}
}

func TestPrintErrorMsg(t *testing.T) {
	// Capture stderr
	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	PrintErrorMsg("error message test")

	w.Close()
	os.Stderr = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	result := strings.TrimSpace(buf.String())

	var output map[string]interface{}
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Errorf("PrintErrorMsg() output is not valid JSON: %v", err)
	}

	if success, ok := output["success"].(bool); !ok || success {
		t.Error("PrintErrorMsg() success field should be false")
	}

	if errMsg, ok := output["error"].(string); !ok || errMsg != "error message test" {
		t.Errorf("PrintErrorMsg() error message = %v, want %v", errMsg, "error message test")
	}
}

func TestPrintErrorMsg_InvalidJSON(t *testing.T) {
	// Test with a message that might break JSON encoding
	// Capture stderr
	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	// Test with special characters
	PrintErrorMsg("error with \"quotes\" and \n newline")

	w.Close()
	os.Stderr = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	result := strings.TrimSpace(buf.String())

	var output map[string]interface{}
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Errorf("PrintErrorMsg() output with special characters is not valid JSON: %v", err)
	}
}

func TestJSONOutput_Print_StructuredData(t *testing.T) {
	type TestMessage struct {
		UID     int      `json:"uid"`
		Subject string   `json:"subject"`
		From    string   `json:"from"`
		To      []string `json:"to"`
		Flags   []string `json:"flags"`
	}

	msg := TestMessage{
		UID:     123,
		Subject: "Test Subject",
		From:    "sender@example.com",
		To:      []string{"recipient@example.com"},
		Flags:   []string{"\\Seen"},
	}

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	out := NewJSONOutput(true)
	err := out.Print(msg)

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	result := strings.TrimSpace(buf.String())

	if err != nil {
		t.Errorf("Print() unexpected error = %v", err)
	}

	var output TestMessage
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Errorf("Print() output is not valid JSON: %v", err)
	}

	if output.UID != msg.UID {
		t.Errorf("Print() UID = %v, want %v", output.UID, msg.UID)
	}
	if output.Subject != msg.Subject {
		t.Errorf("Print() Subject = %v, want %v", output.Subject, msg.Subject)
	}
}

func TestJSONOutput_Print_InvalidData(t *testing.T) {
	// Test with data that cannot be marshaled to JSON
	invalidData := make(chan int) // channels cannot be marshaled to JSON

	out := NewJSONOutput(false)
	err := out.Print(invalidData)

	if err == nil {
		t.Error("Print() expected error for invalid data, got nil")
	}
}

// BenchmarkPrint benchmarks the Print function
func BenchmarkPrint(b *testing.B) {
	data := map[string]interface{}{
		"success": true,
		"message": "test message",
		"data": map[string]int{
			"count": 42,
		},
	}

	out := NewJSONOutput(false)

	// Discard output during benchmark
	old := os.Stdout
	os.Stdout, _ = os.Open(os.DevNull)
	defer func() { os.Stdout = old }()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		out.Print(data)
	}
}

// BenchmarkPrintIndent benchmarks the Print function with indentation
func BenchmarkPrintIndent(b *testing.B) {
	data := map[string]interface{}{
		"success": true,
		"message": "test message",
		"data": map[string]int{
			"count": 42,
		},
	}

	out := NewJSONOutput(true)

	// Discard output during benchmark
	old := os.Stdout
	os.Stdout, _ = os.Open(os.DevNull)
	defer func() { os.Stdout = old }()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		out.Print(data)
	}
}
