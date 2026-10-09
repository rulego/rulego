package endpoint

import (
	"testing"
	"time"

	"github.com/rulego/rulego/server/app"
	"github.com/rulego/rulego/server/config"
	"github.com/rulego/rulego/server/model"
	"github.com/rulego/rulego/server/services"
)

// denied 限采样：同键窗口期内只放行第一条，过期恢复
func TestDeniedSampler(t *testing.T) {
	s := &deniedSampler{seen: make(map[string]time.Time), window: 50 * time.Millisecond}
	if !s.allow("k") {
		t.Fatal("first event should pass")
	}
	if s.allow("k") {
		t.Fatal("second event within window should be sampled out")
	}
	if !s.allow("other") {
		t.Fatal("different key should not be affected")
	}
	time.Sleep(60 * time.Millisecond)
	if !s.allow("k") {
		t.Fatal("key should pass again after window expiry")
	}
}

type stubAuditService struct {
	events []model.AuditEvent
}

func (s *stubAuditService) Record(event model.AuditEvent) { s.events = append(s.events, event) }
func (s *stubAuditService) List(filter model.AuditFilter) ([]model.AuditEvent, int64, error) {
	return nil, 0, nil
}

type stubViewerAuthenticator struct{}

func (stubViewerAuthenticator) Authenticate(authorization string) (*model.UserContext, error) {
	return &model.UserContext{Username: "viewer-u", Roles: []string{"viewer"}}, nil
}

// 403 必须落 denied 审计且限采样：探测刷屏时同键只记第一条
func TestAuthWithPermission_DeniedAuditedAndSampled(t *testing.T) {
	svc := &stubAuditService{}
	srv := &Server{
		container: app.NewContainer(),
		config: &config.Config{
			RequireAuth:     true,
			DefaultUsername: "admin",
			JwtSecretKey:    "unit-test-secret",
		},
	}
	srv.container.Register(services.KeyAuditService, svc)
	srv.container.Register(services.KeyAuthenticator, stubViewerAuthenticator{})

	for i := 0; i < 3; i++ {
		exchange := newTestExchange(t)
		exchange.In.Headers().Set("Authorization", "Bearer some-token")
		if srv.authWithPermission("rule", "write")(nil, exchange) {
			t.Fatal("viewer write should be denied")
		}
	}
	if len(svc.events) != 1 {
		t.Fatalf("denied events = %d, want 1 (sampled)", len(svc.events))
	}
	if svc.events[0].Result != model.AuditResultDenied || svc.events[0].Action != "rule:write" {
		t.Fatalf("unexpected event: %+v", svc.events[0])
	}
	if svc.events[0].Actor != "viewer-u" {
		t.Errorf("actor = %q, want viewer-u", svc.events[0].Actor)
	}
}
