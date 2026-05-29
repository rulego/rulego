package endpoint

import (
	"testing"

	"github.com/rulego/rulego/server/app"
	"github.com/rulego/rulego/server/config"
)

// TestNewStandardRestEndpoint_WithSkillRoutes ensures static skill routes do
// not conflict with the wildcard skill detail route during router creation.
func TestNewStandardRestEndpoint_WithSkillRoutes(t *testing.T) {
	server := NewServer(&app.Container{}, &config.Config{
		Server: "127.0.0.1:0",
	}, nil)

	if _, err := server.NewStandardRestEndpoint(); err != nil {
		t.Fatalf("NewStandardRestEndpoint() error = %v", err)
	}
}
