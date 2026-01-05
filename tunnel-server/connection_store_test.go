package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/igneel64/iskandar/shared"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createWSServerConnection(t *testing.T) *websocket.Conn {
	var upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		con, err := upgrader.Upgrade(w, r, nil)
		require.NoError(t, err)
		defer con.Close()
	}))

	t.Cleanup(s.Close)

	wsUrl := "ws" + s.URL[4:]
	c, _, err := websocket.DefaultDialer.Dial(wsUrl, nil)
	require.NoError(t, err)
	t.Cleanup(func() {
		c.Close()
	})

	return c
}

func TestInMemoryConnectionStore(t *testing.T) {
	t.Run("registers connection", func(t *testing.T) {
		t.Parallel()
		connectionStore := NewInMemoryConnectionStore()
		conn := createWSServerConnection(t)
		subdomain, err := connectionStore.RegisterConnection(conn)
		assert.NoError(t, err)
		assert.Len(t, connectionStore.connMap, 1)
		assert.NotNil(t, connectionStore.connMap[subdomain])
		assert.IsType(t, &shared.SafeWebSocketConn{}, connectionStore.connMap[subdomain])
	})

	t.Run("gets registered connection", func(t *testing.T) {
		t.Parallel()
		connectionStore := NewInMemoryConnectionStore()
		conn := createWSServerConnection(t)
		subdomain, err := connectionStore.RegisterConnection(conn)
		require.NoError(t, err)

		retrievedConn, err := connectionStore.GetConnection(subdomain)
		assert.NoError(t, err)
		assert.NotNil(t, retrievedConn)
		assert.IsType(t, &shared.SafeWebSocketConn{}, retrievedConn)
	})

	t.Run("returns error for non-existent connection", func(t *testing.T) {
		t.Parallel()
		connectionStore := NewInMemoryConnectionStore()
		_, err := connectionStore.GetConnection("nonexistent")
		assert.Error(t, err)
	})

	t.Run("removes registered connection", func(t *testing.T) {
		t.Parallel()
		connectionStore := NewInMemoryConnectionStore()
		conn := createWSServerConnection(t)
		subdomain, err := connectionStore.RegisterConnection(conn)
		require.NoError(t, err)

		connectionStore.RemoveConnection(subdomain)

		_, err = connectionStore.GetConnection(subdomain)
		assert.Error(t, err)
		assert.NotContains(t, connectionStore.connMap, subdomain)
	})
}
