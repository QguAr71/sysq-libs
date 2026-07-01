package portfixer

import (
	"context"
	"encoding/json"
	"os"
	"testing"
)

type mockBus struct {
	messages []struct {
		topic   string
		payload map[string]interface{}
	}
}

func (m *mockBus) Publish(ctx context.Context, topic string, msg Message) error {
	var payload map[string]interface{}
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return err
	}
	m.messages = append(m.messages, struct {
		topic   string
		payload map[string]interface{}
	}{topic, payload})
	return nil
}

// mockCheck returns true for ports >= 9600 (simulate: 9596-9599 busy).
func mockCheck(port int) bool {
	return port >= 9600
}

func TestResolveEnvOverride(t *testing.T) {
	os.Setenv("PORTFIXER_TEST_PORT", "8888")
	defer os.Unsetenv("PORTFIXER_TEST_PORT")

	pf := New(Config{
		DefaultPort: 9595,
		EnvVar:      "PORTFIXER_TEST_PORT",
	})
	port := pf.Resolve()
	if port != 8888 {
		t.Errorf("expected 8888, got %d", port)
	}
}

func TestResolveDefault(t *testing.T) {
	pf := New(Config{
		DefaultPort: 9595,
	})
	// Default port will be checked via real net.Dial.
	// 9595 is typically free in tests, but we can't guarantee.
	// Just verify it returns a positive port.
	port := pf.Resolve()
	if port <= 0 {
		t.Errorf("expected positive port, got %d", port)
	}
}

func TestFindAvailable(t *testing.T) {
	// Simulate: 9595 busy, 9596 free
	busy := map[int]bool{9595: true, 9597: true, 9598: true, 9599: true, 9600: true}
	check := func(p int) bool { return busy[p] }

	got := findAvailable(9595, 9600, check)
	if got != 9596 {
		t.Errorf("expected 9596, got %d", got)
	}
}

func TestFindAvailableNone(t *testing.T) {
	busy := map[int]bool{9595: true, 9596: true, 9597: true}
	check := func(p int) bool { return busy[p] }

	got := findAvailable(9595, 9597, check)
	if got != -1 {
		t.Errorf("expected -1, got %d", got)
	}
}

func TestPortCaching(t *testing.T) {
	pf := New(Config{DefaultPort: 9595})

	// First call checks real port
	p1 := pf.isInUse(9999)
	// Second call must hit cache
	p2 := pf.isInUse(9999)
	if p1 != p2 {
		t.Errorf("cache inconsistency: %v vs %v", p1, p2)
	}
}

func TestClearCache(t *testing.T) {
	pf := New(Config{DefaultPort: 9595})

	_ = pf.isInUse(9998)
	pf.ClearCache()

	pf.cacheMu.RLock()
	_, exists := pf.cache[9998]
	pf.cacheMu.RUnlock()
	if exists {
		t.Error("cache not cleared")
	}
}

func TestIdempotentResolve(t *testing.T) {
	pf := New(Config{DefaultPort: 9595})
	p1 := pf.Resolve()
	p2 := pf.Resolve()
	if p1 != p2 {
		t.Errorf("idempotent fail: %d vs %d", p1, p2)
	}
}

func TestResolvePublishesAssigned(t *testing.T) {
	bus := &mockBus{}
	pf := New(Config{
		DefaultPort: 9595,
		Bus:         bus,
	})
	_ = pf.Resolve()

	found := false
	for _, m := range bus.messages {
		if m.topic == "port.assigned" {
			found = true
			if src, ok := m.payload["source"]; !ok || src == "" {
				t.Error("port.assigned missing source")
			}
		}
	}
	if !found {
		t.Error("port.assigned not published")
	}
}

func TestResolveNilSafeBus(t *testing.T) {
	// Nil bus — must not panic
	pf := New(Config{
		DefaultPort: 9595,
		Bus:         nil,
	})
	port := pf.Resolve()
	if port <= 0 {
		t.Errorf("expected positive port, got %d", port)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.DefaultPort != 9595 {
		t.Errorf("default port: %d", cfg.DefaultPort)
	}
	if cfg.EnvVar != "LAZARUSD_MCP_PORT" {
		t.Errorf("env var: %s", cfg.EnvVar)
	}
}

func TestWritePortFile(t *testing.T) {
	pf := New(Config{DefaultPort: 9595})
	pf.WritePortFile()

	home, _ := os.UserHomeDir()
	path := home + "/.cache/lazarus/mcp_port"

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("port file not written: %v", err)
	}

	// Verify content is a positive integer ending with newline
	content := string(data)
	if len(content) == 0 {
		t.Error("port file empty")
	}
	port := 0
	for _, c := range content {
		if c == '\n' {
			break
		}
		if c >= '0' && c <= '9' {
			port = port*10 + int(c-'0')
		}
	}
	if port <= 0 {
		t.Errorf("invalid port in file: %s", content)
	}
}

func TestWritePortFileCreatesDir(t *testing.T) {
	home, _ := os.UserHomeDir()
	path := home + "/.cache/lazarus/mcp_port"

	// Clean up first
	os.Remove(path)

	pf := New(Config{DefaultPort: 9595})
	pf.WritePortFile()

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("port file not created")
	}
}

func TestWritePortFileIdempotent(t *testing.T) {
	pf := New(Config{DefaultPort: 9595})

	// First write
	pf.WritePortFile()
	home, _ := os.UserHomeDir()
	path := home + "/.cache/lazarus/mcp_port"
	data1, _ := os.ReadFile(path)

	// Second write — port cached, same result
	pf.WritePortFile()
	data2, _ := os.ReadFile(path)

	if string(data1) != string(data2) {
		t.Error("port file changed between writes")
	}
}
