package config

import (
	"testing"
)

func TestParseDestination(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:    "port only - valid",
			input:   "8080",
			want:    "http://localhost:8080",
			wantErr: false,
		},
		{
			name:    "port only - low port",
			input:   "80",
			want:    "http://localhost:80",
			wantErr: false,
		},
		{
			name:    "port only - high port",
			input:   "65535",
			want:    "http://localhost:65535",
			wantErr: false,
		},
		{
			name:    "host:port - valid",
			input:   "foo.dev:80",
			want:    "http://foo.dev:80",
			wantErr: false,
		},
		{
			name:    "host:port - localhost",
			input:   "localhost:3000",
			want:    "http://localhost:3000",
			wantErr: false,
		},
		{
			name:    "host:port - IP address",
			input:   "192.168.1.100:8080",
			want:    "http://192.168.1.100:8080",
			wantErr: false,
		},
		{
			name:    "port only - below range",
			input:   "0",
			want:    "",
			wantErr: true,
		},
		{
			name:    "port only - above range",
			input:   "65536",
			want:    "",
			wantErr: true,
		},
		{
			name:    "invalid - not a number",
			input:   "abc",
			want:    "",
			wantErr: true,
		},
		{
			name:    "host:port - missing port",
			input:   "foo.dev",
			want:    "",
			wantErr: true,
		},
		{
			name:    "host:port - invalid port",
			input:   "foo.dev:99999",
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseDestination(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseDestination() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseDestination() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseServerURL(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:    "plain host:port",
			input:   "localhost:8080",
			want:    "ws://localhost:8080/tunnel/connect",
			wantErr: false,
		},
		{
			name:    "http scheme",
			input:   "http://localhost:8080",
			want:    "ws://localhost:8080/tunnel/connect",
			wantErr: false,
		},
		{
			name:    "https scheme",
			input:   "https://tunnel.example.com",
			want:    "wss://tunnel.example.com/tunnel/connect",
			wantErr: false,
		},
		{
			name:    "https with port",
			input:   "https://tunnel.example.com:443",
			want:    "wss://tunnel.example.com:443/tunnel/connect",
			wantErr: false,
		},
		{
			name:    "domain without port",
			input:   "tunnel.example.com",
			want:    "ws://tunnel.example.com/tunnel/connect",
			wantErr: false,
		},
		{
			name:    "invalid scheme - ftp",
			input:   "ftp://example.com",
			want:    "",
			wantErr: true,
		},
		{
			name:    "invalid scheme - ws",
			input:   "ws://example.com",
			want:    "",
			wantErr: true,
		},
		{
			name:    "invalid scheme - wss",
			input:   "wss://example.com",
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseServerURL(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseServerURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseServerURL() = %v, want %v", got, tt.want)
			}
		})
	}
}
