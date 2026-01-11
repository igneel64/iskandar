package websocket

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
)

func TestWriteSafeWSDialer_Dial_Success(t *testing.T) {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("Failed to upgrade connection: %v", err)
			return
		}
		//nolint:errcheck
		defer conn.Close()

		// Keep connection open for the test
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				break
			}
		}
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	dialer := NewWriteSafeWSDialer(wsURL, false)
	conn, err := dialer.Dial()

	assert.NoError(t, err, "Dial should succeed")
	assert.NotNil(t, conn, "Connection should not be nil")

	//nolint:errcheck
	conn.Close()
}
