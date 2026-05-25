package permission

import "testing"

func TestPolicyRequiresConfirmation(t *testing.T) {
	tests := []struct {
		name string
		p    Policy
		req  Request
		want bool
	}{
		{
			name: "risky tool requires confirmation",
			p:    NewPolicy(false, nil, nil),
			req:  Request{ToolName: ToolWrite},
			want: true,
		},
		{
			name: "read-only tool does not require confirmation",
			p:    NewPolicy(false, nil, nil),
			req:  Request{ToolName: "read"},
		},
		{
			name: "skip requests bypasses confirmation",
			p:    NewPolicy(true, nil, nil),
			req:  Request{ToolName: ToolCodeRun},
		},
		{
			name: "tool allowlist bypasses confirmation",
			p:    NewPolicy(false, []string{ToolPatch}, nil),
			req:  Request{ToolName: ToolPatch},
		},
		{
			name: "tool action allowlist bypasses confirmation",
			p:    NewPolicy(false, []string{ToolCodeRun + ":python"}, nil),
			req:  Request{ToolName: ToolCodeRun, Action: "python"},
		},
		{
			name: "custom risky tool requires confirmation",
			p:    NewPolicy(false, nil, []string{"deploy"}),
			req:  Request{ToolName: "deploy"},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.p.RequiresConfirmation(tt.req); got != tt.want {
				t.Fatalf("RequiresConfirmation() = %v, want %v", got, tt.want)
			}
		})
	}
}
