package constants

import "errors"

var (
	ErrNotFound        = errors.New("not found")
	ErrUsernameEmpty   = errors.New("username cannot empty")
	ErrWorkflowIdEmpty = errors.New("workflowId cannot empty")
)
