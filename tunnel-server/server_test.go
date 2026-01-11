package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	cerrors "github.com/igneel64/iskandar/server/internal/errors"
	"github.com/igneel64/iskandar/server/internal/logger"
	"github.com/igneel64/iskandar/shared"
	"github.com/igneel64/iskandar/shared/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type MockConnectionStore struct {
	mock.Mock
}

func (m *MockConnectionStore) RegisterConnection(conn *websocket.Conn) (string, error) {
	args := m.Called(conn)
	return args.String(0), args.Error(1)
}

func (m *MockConnectionStore) GetConnection(subdomain string) (*shared.SafeWebSocketConn, error) {
	args := m.Called(subdomain)
	return args.Get(0).(*shared.SafeWebSocketConn), args.Error(1)
}

func (m *MockConnectionStore) RemoveConnection(subdomain string) {
	m.Called(subdomain)
}

type MockRequestManager struct {
	mock.Mock
}

func (m *MockRequestManager) GetRequestChannel(requestId string) (MessageChannel, bool) {
	args := m.Called(requestId)
	return args.Get(0).(MessageChannel), args.Bool(1)
}

func (m *MockRequestManager) RegisterRequest(requestId string, subdomain string) (MessageChannel, error) {
	args := m.Called(requestId, subdomain)
	return args.Get(0).(MessageChannel), args.Error(1)
}

func (m *MockRequestManager) RemoveRequest(requestId string, subdomain string) {
	m.Called(requestId, subdomain)
}

func TestServer(t *testing.T) {
	publicURLBase, err := url.Parse("http://localhost.direct:8080")
	require.NoError(t, err)

	t.Run("accepts websocket connection at /tunnel/connect", func(t *testing.T) {
		connectionStore := NewInMemoryConnectionStore(10)
		requestManager := NewInMemoryRequestManager(10)
		appLogger := logger.NewLogger(false)
		server := NewIskndrServer(publicURLBase, connectionStore, requestManager, appLogger)

		ts := httptest.NewServer(server)
		defer ts.Close()

		wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/tunnel/connect"

		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		require.NoError(t, err, "should connect to websocket")
		//nolint:errcheck
		defer conn.Close()

		var regMsg protocol.RegisterTunnelMessage
		err = conn.ReadJSON(&regMsg)
		require.NoError(t, err)

		assert.NotEmpty(t, regMsg.Subdomain)
		assert.Contains(t, regMsg.Subdomain, "http://")
	})
}

func TestHandleRequest(t *testing.T) {
	t.Run("error on request without subdomain", func(t *testing.T) {
		publicURLBase, err := url.Parse("http://localhost.direct:8080")
		require.NoError(t, err)
		appLogger := logger.NewLogger(false)

		mockConnectionStore := new(MockConnectionStore)
		mockRequestManager := new(MockRequestManager)

		server := NewIskndrServer(publicURLBase, mockConnectionStore, mockRequestManager, appLogger)

		ts := httptest.NewServer(server)
		defer ts.Close()

		req, err := http.NewRequest("GET", ts.URL+"/test-path", nil)
		require.NoError(t, err)
		req.Host = "localhost"

		response := httptest.NewRecorder()

		server.handleRequest(response, req)

		result := response.Result()
		//nolint:errcheck
		defer result.Body.Close()

		assert.Equal(t, http.StatusBadRequest, result.StatusCode)
		assert.Equal(t, "Invalid subdomain\n", response.Body.String())
	})

	t.Run("error on request to unassigned subdomain", func(t *testing.T) {
		publicURLBase, err := url.Parse("http://localhost.direct:8080")
		require.NoError(t, err)
		appLogger := logger.NewLogger(false)

		mockConnectionStore := new(MockConnectionStore)
		mockRequestManager := new(MockRequestManager)
		mockConnectionStore.On("GetConnection", "test").Return((*shared.SafeWebSocketConn)(nil), errors.New("not found"))

		server := NewIskndrServer(publicURLBase, mockConnectionStore, mockRequestManager, appLogger)

		ts := httptest.NewServer(server)
		defer ts.Close()

		req, err := http.NewRequest("GET", ts.URL+"/test-path", nil)
		require.NoError(t, err)
		req.Host = "test.localhost"

		response := httptest.NewRecorder()

		server.handleRequest(response, req)

		result := response.Result()
		//nolint:errcheck
		defer result.Body.Close()

		assert.Equal(t, http.StatusNotFound, result.StatusCode)
		assert.Equal(t, "No tunnel found for subdomain\n", response.Body.String())
	})
}

type trackingResponseWriter struct {
	*httptest.ResponseRecorder
	writeTimestamps []time.Time
	flushCount      int
}

func newTrackingResponseWriter() *trackingResponseWriter {
	return &trackingResponseWriter{
		ResponseRecorder: httptest.NewRecorder(),
		writeTimestamps:  []time.Time{},
		flushCount:       0,
	}
}

func (trw *trackingResponseWriter) Write(b []byte) (int, error) {
	trw.writeTimestamps = append(trw.writeTimestamps, time.Now())
	return trw.ResponseRecorder.Write(b)
}

func (trw *trackingResponseWriter) Flush() {
	trw.flushCount++
	trw.ResponseRecorder.Flush()
}

func TestWriteProxiedResponse(t *testing.T) {
	publicURLBase, err := url.Parse("http://localhost.direct:8080")
	require.NoError(t, err)

	appLogger := logger.NewLogger(false)
	server := NewIskndrServer(publicURLBase, new(MockConnectionStore), new(MockRequestManager), appLogger)

	t.Run("send back response from channel", func(t *testing.T) {
		ch := make(chan protocol.Message, 1)
		defer close(ch)

		responseMessage := protocol.Message{
			Type:   "response",
			Id:     "req-123",
			Status: 200,
			Body:   []byte("Hello, World!"),
			Headers: map[string]string{
				"Content-Type": "text/plain",
			},
			Done: true,
		}
		ch <- responseMessage

		response := httptest.NewRecorder()

		err := server.writeProxiedResponse(response, ch, "req-123", "subdomain", "/test", "GET", time.Now())
		require.NoError(t, err)

		result := response.Result()
		//nolint:errcheck
		defer result.Body.Close()

		assert.Equal(t, 200, result.StatusCode)
		assert.Equal(t, "text/plain", result.Header.Get("Content-Type"))
		assert.Equal(t, "Hello, World!", response.Body.String())
	})

	t.Run("send back stream response from channel", func(t *testing.T) {
		ch := make(chan protocol.Message)
		defer close(ch)

		responseFirstMessage := protocol.Message{
			Type:   "response",
			Id:     "req-123",
			Status: 200,
			Body:   []byte("Hello"),
			Headers: map[string]string{
				"Content-Type": "text/plain",
			},
			Done: false,
		}

		responseFinalMessage := protocol.Message{
			Type: "response",
			Id:   "req-123",
			Body: []byte(", World!"),
			Done: true,
		}

		go func() {
			ch <- responseFirstMessage
			time.Sleep(10 * time.Millisecond)
			ch <- responseFinalMessage
		}()

		response := httptest.NewRecorder()

		err := server.writeProxiedResponse(response, ch, "req-123", "subdomain", "/test", "GET", time.Now())
		require.NoError(t, err)

		result := response.Result()
		//nolint:errcheck
		defer result.Body.Close()

		assert.Equal(t, 200, result.StatusCode)
		assert.Equal(t, "text/plain", result.Header.Get("Content-Type"))
		assert.Equal(t, "Hello, World!", response.Body.String())
	})

	t.Run("sends channel not responding error", func(t *testing.T) {
		ch := make(chan protocol.Message)
		close(ch)
		response := httptest.NewRecorder()

		err := server.writeProxiedResponse(response, ch, "req-123", "subdomain", "/test", "GET", time.Now())
		require.Error(t, err)
		assert.IsType(t, &cerrors.TunnelNotRespondingError{}, err)
	})

	t.Run("streams chunks immediately before completion", func(t *testing.T) {
		ch := make(chan protocol.Message)
		defer close(ch)

		responseFirstMessage := protocol.Message{
			Type:   "response",
			Id:     "req-123",
			Status: 200,
			Body:   []byte("Hello"),
			Headers: map[string]string{
				"Content-Type": "text/plain",
			},
			Done: false,
		}

		responseFinalMessage := protocol.Message{
			Type: "response",
			Id:   "req-123",
			Body: []byte(", World!"),
			Done: true,
		}

		response := newTrackingResponseWriter()
		delay := 50 * time.Millisecond
		go func() {
			ch <- responseFirstMessage
			time.Sleep(delay)
			ch <- responseFinalMessage
		}()

		err := server.writeProxiedResponse(response, ch, "req-123", "subdomain", "/test", "GET", time.Now())
		require.NoError(t, err)

		result := response.Result()
		//nolint:errcheck
		defer result.Body.Close()

		assert.Equal(t, 200, result.StatusCode)
		assert.Equal(t, "text/plain", result.Header.Get("Content-Type"))
		assert.Equal(t, "Hello, World!", response.Body.String())
		assert.Equal(t, response.flushCount, 2, "should have flushed at least twice")

		assert.Len(t, response.writeTimestamps, 2)
		timeBetweenWrites := response.writeTimestamps[1].Sub(response.writeTimestamps[0])
		assert.GreaterOrEqual(t, timeBetweenWrites, delay,
			"second write should be delayed (proves first write happened immediately)")
	})
}
