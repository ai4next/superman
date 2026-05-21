package tools

import (
	"testing"
)

func TestIsPathAllowed(t *testing.T) {
	allowed := []string{"/home/user/project", "/tmp"}
	tests := []struct {
		path    string
		allowed bool
	}{
		{"/home/user/project/main.go", true},
		{"/home/user/project/subdir/file.go", true},
		{"/tmp/foo.txt", true},
		{"/etc/passwd", false},
		{"/var/log/syslog", false},
	}
	for _, tt := range tests {
		got := isPathAllowed(tt.path, allowed)
		if got != tt.allowed {
			t.Errorf("isPathAllowed(%q) = %v, want %v", tt.path, got, tt.allowed)
		}
	}
}

func TestValidatePath(t *testing.T) {
	allowed := []string{"/tmp"}
	_, err := validatePath("/tmp/test.txt", allowed)
	if err != nil {
		t.Errorf("validatePath(/tmp/test.txt) = %v, want nil", err)
	}
	_, err = validatePath("/etc/passwd", allowed)
	if err == nil {
		t.Error("validatePath(/etc/passwd) = nil, want error")
	}
}

func TestGetCheckpoints(t *testing.T) {
	// Reset checkpoint store
	checkpointMu.Lock()
	checkpointStore = make(map[string]string)
	checkpointMu.Unlock()

	cps := GetCheckpoints()
	if len(cps) != 0 {
		t.Errorf("expected 0 checkpoints initially, got %d", len(cps))
	}
}