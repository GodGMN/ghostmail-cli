// Package output provides JSON output formatting utilities.
package output

import (
	"encoding/json"
	"fmt"
	"os"
)

// JSONOutput handles JSON output formatting.
type JSONOutput struct {
	indent bool
}

// NewJSONOutput creates a new JSON output handler.
func NewJSONOutput(indent bool) *JSONOutput {
	return &JSONOutput{indent: indent}
}

// Print prints data as JSON.
func (j *JSONOutput) Print(data interface{}) error {
	var output []byte
	var err error

	if j.indent {
		output, err = json.MarshalIndent(data, "", "  ")
	} else {
		output, err = json.Marshal(data)
	}

	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	fmt.Println(string(output))
	return nil
}

// PrintError prints an error as JSON.
func (j *JSONOutput) PrintError(err error) error {
	data := map[string]interface{}{
		"success": false,
		"error":   err.Error(),
	}
	return j.Print(data)
}

// PrintErrorMsg prints an error message as JSON.
func PrintErrorMsg(msg string) {
	data := map[string]interface{}{
		"success": false,
		"error":   msg,
	}
	output, _ := json.Marshal(data)
	fmt.Fprintln(os.Stderr, string(output))
}
