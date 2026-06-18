package cerber

import "testing"

func TestIsWriteTool(t *testing.T) {
	tests := []struct {
		method string
		want   bool
	}{
		{"citadel.write", true},
		{"identity.deploy", true},
		{"identity.rollback", true},
		{"superwing.create", true},
		{"superwing.delete", true},
		{"wing.create", true},
		{"wing.switch", true},
		{"wing.deploy", true},
		{"gc.run", true},
	}

	for _, tt := range tests {
		got := IsWriteTool(tt.method)
		if got != tt.want {
			t.Errorf("IsWriteTool(%q) = %v, want %v", tt.method, got, tt.want)
		}
	}
}

func TestIsReadTool(t *testing.T) {
	tests := []struct {
		method string
		want   bool
	}{
		{"citadel.read", true},
		{"citadel.search", true},
		{"citadel.count", true},
		{"citadel.read_room", true},
		{"session.status", true},
		{"session.save", true},
		{"lazarus.health", true},
		{"lazarus.version", true},
		{"identity.validate", true},
	}

	for _, tt := range tests {
		got := IsReadTool(tt.method)
		if got != tt.want {
			t.Errorf("IsReadTool(%q) = %v, want %v", tt.method, got, tt.want)
		}
	}
}

func TestWriteToolNeverInReadTool(t *testing.T) {
	// Invariant: write tools must NEVER be in read tools
	for method := range WriteTool {
		if ReadTool[method] {
			t.Errorf("WriteTool %q appears in ReadTool — invariant broken", method)
		}
	}
	for method := range ReadTool {
		if WriteTool[method] {
			t.Errorf("ReadTool %q appears in WriteTool — invariant broken", method)
		}
	}
}

func TestIsWriteTool_UnknownMethod(t *testing.T) {
	// Unknown/unlisted methods are write-safe by default (not in WriteTool)
	if IsWriteTool("nonexistent.method") {
		t.Errorf("unknown method should not be write tool")
	}
	if IsReadTool("nonexistent.method") {
		t.Errorf("unknown method should not be read tool")
	}
}

func TestGateResult(t *testing.T) {
	gr := GateResult{
		Passed: false,
		Reason: "write_not_approved",
		Head:   "write",
		Agent:  "echo",
		Method: "citadel.write",
	}
	if gr.Passed {
		t.Errorf("GateResult should be failed")
	}
	if gr.Head != "write" {
		t.Errorf("expected head 'write', got %q", gr.Head)
	}
	if gr.Agent != "echo" {
		t.Errorf("expected agent 'echo', got %q", gr.Agent)
	}
}
