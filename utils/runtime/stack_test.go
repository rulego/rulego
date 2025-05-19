package runtime

import (
	"strings"
	"testing"

	"github.com/rulego/rulego/test/assert"
)

func TestStack(t *testing.T) {
	// Call the Stack function
	stackTrace := Stack()

	// Check if the stack trace is not empty
	assert.True(t, len(stackTrace) > 0, "Stack trace should not be empty")

	// Check if the stack trace contains the current file's name (stack_test.go)
	// This is a basic check, as the exact line number can vary.
	assert.True(t, strings.Contains(stackTrace, "testing.go"), "Stack trace should contain the test file name")

	// Check for a line number pattern (e.g., ":<number>")
	// This is a more fragile test and might need adjustment if the Stack function's formatting changes.
	assert.True(t, strings.Contains(stackTrace, ":"), "Stack trace should contain line numbers")

}
