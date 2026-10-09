package fieldservice

import "testing"

func TestFollowUpAction(t *testing.T) {
	tests := []struct {
		name  string
		reply string
		want  string
	}{
		{name: "photo needs a return visit", reply: "FOLLOW_UP: meter seal is missing", want: "schedule_technician_follow_up"},
		{name: "photo clears dispatch", reply: "OPEN: panel label is readable", want: "keep_dispatch_open"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FollowUpAction(tt.reply); got != tt.want {
				t.Fatalf("FollowUpAction(%q) = %q, want %q", tt.reply, got, tt.want)
			}
		})
	}
}
