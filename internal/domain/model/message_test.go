package model

import "testing"

func TestInboundMessage_SessionKey(t *testing.T) {
	tests := []struct {
		name string
		msg  InboundMessage
		want string
	}{
		{
			name: "default key from channel and chatID",
			msg:  InboundMessage{Channel: ChannelTelegram, ChatID: "12345"},
			want: "telegram:12345",
		},
		{
			name: "override takes precedence",
			msg: InboundMessage{
				Channel:            ChannelTelegram,
				ChatID:             "12345",
				SessionKeyOverride: strPtr("custom-session"),
			},
			want: "custom-session",
		},
		{
			name: "cli channel",
			msg:  InboundMessage{Channel: ChannelCLI, ChatID: "local"},
			want: "cli:local",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.msg.SessionKey(); got != tt.want {
				t.Errorf("SessionKey() = %q, want %q", got, tt.want)
			}
		})
	}
}

func strPtr(s string) *string { return &s }
