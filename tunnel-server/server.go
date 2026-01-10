package main

import (
	"errors"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/igneel64/iskandar/server/internal/config"
	"github.com/igneel64/iskandar/server/internal/logger"
	"github.com/igneel64/iskandar/shared"
	"github.com/igneel64/iskandar/shared/protocol"
)

type IskndrServer struct {
	http.Handler
	publicURLBase  *url.URL
	connStore      ConnectionStore
	requestManager RequestManager
}

func NewIskndrServer(publicURLBase *url.URL, connectionStore ConnectionStore, requestManager RequestManager) *IskndrServer {
	i := &IskndrServer{
		publicURLBase:  publicURLBase,
		connStore:      connectionStore,
		requestManager: requestManager,
	}

	router := http.NewServeMux()
	router.HandleFunc("/health", i.handleHealth)
	router.HandleFunc("/tunnel/connect", i.handleTunnelConnect)
	router.HandleFunc("/", i.handleRequest)

	i.Handler = router

	return i
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func (i *IskndrServer) handleTunnelConnect(w http.ResponseWriter, r *http.Request) {
	con, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, "Failed to upgrade to websocket", http.StatusInternalServerError)
		return
	}

	var subdomainKey string

	defer func() {
		if err := con.Close(); err != nil {
			logger.WebSocketCloseFailed(subdomainKey, err)
		}
	}()

	subdomainKey, err = i.connStore.RegisterConnection(con)
	if err != nil {
		if errors.Is(err, ErrMaxTunnelsReached) {
			logger.MaxTunnelsReached()
			http.Error(w, "Server tunnel capacity reached", http.StatusServiceUnavailable)
			return
		}
		logger.TunnelRegistrationFailed(err)
		http.Error(w, "Failed to register connection", http.StatusInternalServerError)
		return
	}

	logger.TunnelConnected(subdomainKey, r.RemoteAddr)

	subdomainURL := config.ExtractSubdomainURL(i.publicURLBase, subdomainKey)

	err = con.WriteJSON(&protocol.RegisterTunnelMessage{Subdomain: subdomainURL})
	if err != nil {
		http.Error(w, "Failed to send register tunnel message", http.StatusInternalServerError)
		return
	}

	for {
		var msg protocol.Message
		if err = con.ReadJSON(&msg); err != nil {
			logger.TunnelDisconnected(subdomainKey, err)
			i.connStore.RemoveConnection(subdomainKey)
			return
		}

		if ch, ok := i.requestManager.GetRequestChannel(msg.Id); ok {
			ch <- msg
		}
	}
}

func (i *IskndrServer) handleRequest(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	subdomain := config.ExtractAssignedSubdomain(r.Host)

	logger.HTTPRequestReceived(subdomain, r.Method, r.RequestURI, r.RemoteAddr)

	conn, err := i.connStore.GetConnection(subdomain)
	if err != nil {
		logger.TunnelNotFound(subdomain, r.Host)
		http.Error(w, "No tunnel found for subdomain", http.StatusNotFound)
		return
	}

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusInternalServerError)
		return
	}

	//nolint:errcheck
	defer r.Body.Close()

	requestId := uuid.New().String()

	message := &protocol.Message{
		Type:    "request",
		Id:      requestId,
		Body:    bodyBytes,
		Method:  r.Method,
		Headers: shared.SerializeHeaders(r.Header),
		Path:    r.RequestURI,
	}

	if err = conn.WriteJSON(message); err != nil {
		logger.RequestForwardFailed(requestId, subdomain, err)
		http.Error(w, "Failed to forward request to tunnel", http.StatusInternalServerError)
		return
	}

	logger.RequestForwarded(requestId, subdomain)

	ch := i.requestManager.RegisterRequest(requestId)
	defer i.requestManager.RemoveRequest(requestId)

	select {
	case response, ok := <-ch:
		duration := time.Since(startTime)

		if !ok {
			logger.ChannelClosed(requestId, duration)
			http.Error(w, "Failed to get response from tunnel", http.StatusInternalServerError)
			return
		}

		logger.HTTPResponse(subdomain, r.Method, r.RequestURI, response.Status, duration, requestId)

		for k, v := range response.Headers {
			w.Header().Set(k, v)
		}
		w.WriteHeader(response.Status)
		n, err := w.Write(response.Body)
		if err != nil {
			logger.ResponseWriteFailed(requestId, len(response.Body), n, err)
			return
		}
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}

		if !response.Done {
			logger.StreamingStarted(requestId, response.Status, len(response.Body))
		}

		for !response.Done {
			select {
			case response, ok = <-ch:
				if !ok {
					logger.ChannelClosed(requestId, time.Since(startTime))
					return
				}

				n, err := w.Write(response.Body)
				if err != nil {
					logger.ResponseWriteFailed(requestId, len(response.Body), n, err)
					return
				}
				if flusher, ok := w.(http.Flusher); ok {
					flusher.Flush()
				}

				if response.Done {
					logger.StreamingCompleted(requestId, time.Since(startTime))
				} else {
					logger.StreamingChunk(requestId, len(response.Body), time.Since(startTime))
				}

			case <-time.After(30 * time.Second):
				logger.RequestTimeout(requestId, subdomain, r.RequestURI)
				return
			}
		}

	case <-time.After(30 * time.Second):
		logger.RequestTimeout(requestId, subdomain, r.RequestURI)
		http.Error(w, "Timeout waiting for response from tunnel", http.StatusGatewayTimeout)
		return
	}
}

func (i *IskndrServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}
