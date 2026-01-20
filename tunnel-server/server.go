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
	cerrors "github.com/igneel64/iskandar/server/internal/errors"
	"github.com/igneel64/iskandar/server/internal/logger"
	"github.com/igneel64/iskandar/server/internal/middleware"
	"github.com/igneel64/iskandar/shared"
	"github.com/igneel64/iskandar/shared/protocol"
)

type IskndrServer struct {
	http.Handler
	publicURLBase  *url.URL
	connStore      ConnectionStore
	requestManager RequestManager
	logger         logger.Logger
}

const MaxBodySize = 4 * 1024 * 1024 // 4 MB

func NewIskndrServer(publicURLBase *url.URL, connectionStore ConnectionStore, requestManager RequestManager, logger logger.Logger) *IskndrServer {
	i := &IskndrServer{
		publicURLBase:  publicURLBase,
		connStore:      connectionStore,
		requestManager: requestManager,
		logger:         logger,
	}

	router := http.NewServeMux()
	router.HandleFunc("/health", i.handleHealth)
	router.HandleFunc("/tunnel/connect", i.handleTunnelConnect)
	router.HandleFunc("/", i.handleRequest)

	i.Handler = middleware.PanicRecoveryMiddleware(router, logger)

	return i
}

var upgrader = websocket.Upgrader{
	CheckOrigin:     func(r *http.Request) bool { return true },
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
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
			i.logger.WebSocketCloseFailed(subdomainKey, err)
		}
	}()

	subdomainKey, err = i.connStore.RegisterConnection(con)
	if err != nil {
		if errors.Is(err, ErrMaxTunnelsReached) {
			i.logger.MaxTunnelsReached()
			http.Error(w, "Server tunnel capacity reached", http.StatusServiceUnavailable)
			return
		}
		i.logger.TunnelRegistrationFailed(err)
		http.Error(w, "Failed to register connection", http.StatusInternalServerError)
		return
	}

	i.logger.TunnelConnected(subdomainKey, r.RemoteAddr)

	subdomainURL := config.ExtractSubdomainURL(i.publicURLBase, subdomainKey)

	err = con.WriteJSON(&protocol.RegisterTunnelMessage{Subdomain: subdomainURL})
	if err != nil {
		http.Error(w, "Failed to send register tunnel message", http.StatusInternalServerError)
		return
	}

	for {
		var msg protocol.Message
		if err = con.ReadJSON(&msg); err != nil {
			i.logger.TunnelDisconnected(subdomainKey, err)
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
	subdomain, err := config.ExtractAssignedSubdomain(r.Host)
	if err != nil {
		http.Error(w, "Invalid subdomain", http.StatusBadRequest)
		return
	}

	i.logger.HTTPRequestReceived(subdomain, r.Method, r.RequestURI, r.RemoteAddr)

	conn, err := i.connStore.GetConnection(subdomain)
	if err != nil {
		i.logger.TunnelNotFound(subdomain, r.Host)
		http.Error(w, "No tunnel found for subdomain", http.StatusNotFound)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, MaxBodySize)
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		if errors.As(err, new(*http.MaxBytesError)) {
			i.logger.RequestBodyTooLarge(subdomain, r.RequestURI)
			http.Error(w, "Request body too large", http.StatusRequestEntityTooLarge)
			return
		}
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
		i.logger.RequestForwardFailed(requestId, subdomain, err)
		http.Error(w, "Failed to forward request to tunnel", http.StatusInternalServerError)
		return
	}

	i.logger.RequestForwarded(requestId, r.RequestURI, subdomain)

	ch, err := i.requestManager.RegisterRequest(requestId, subdomain)
	if err != nil {
		if errors.Is(err, ErrMaxRequestsPerTunnel) {
			i.logger.MaxRequestsPerTunnelReached(subdomain)
			http.Error(w, "Tunnel request capacity reached", http.StatusServiceUnavailable)
			return
		}
		i.logger.RequestRegistrationFailed(requestId, subdomain, err)
		http.Error(w, "Failed to register request", http.StatusInternalServerError)
		return
	}
	defer i.requestManager.RemoveRequest(requestId, subdomain)

	if err := i.writeProxiedResponse(w, ch, requestId, subdomain, r.RequestURI, r.Method, startTime); err != nil {
		if httpErr, ok := err.(cerrors.SendableHTTPError); ok {
			http.Error(w, httpErr.Error(), httpErr.StatusCode())
		}
		return
	}
}

func (i *IskndrServer) writeProxiedResponse(w http.ResponseWriter, ch MessageChannel, requestId, subdomain, requestURI, requestMethod string, startTime time.Time) error {
	select {
	case response, ok := <-ch:
		duration := time.Since(startTime)

		if !ok {
			i.logger.ChannelClosed(requestId, duration)
			return &cerrors.TunnelNotRespondingError{}
		}

		i.logger.HTTPResponse(subdomain, requestMethod, requestURI, response.Status, duration, requestId)

		for k, v := range response.Headers {
			w.Header().Set(k, v)
		}
		w.WriteHeader(response.Status)
		n, err := w.Write(response.Body)
		if err != nil {
			i.logger.ResponseWriteFailed(requestId, len(response.Body), n, err)
			return nil
		}
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}

		if !response.Done {
			i.logger.StreamingStarted(requestId, response.Status, len(response.Body))
		}

		for !response.Done {
			select {
			case response, ok = <-ch:
				if !ok {
					i.logger.ChannelClosed(requestId, time.Since(startTime))
					return nil
				}

				n, err := w.Write(response.Body)
				if err != nil {
					i.logger.ResponseWriteFailed(requestId, len(response.Body), n, err)
					return nil
				}
				if flusher, ok := w.(http.Flusher); ok {
					flusher.Flush()
				}

				if response.Done {
					i.logger.StreamingCompleted(requestId, time.Since(startTime))
				} else {
					i.logger.StreamingChunk(requestId, len(response.Body), time.Since(startTime))
				}

			case <-time.After(30 * time.Second):
				i.logger.RequestTimeout(requestId, subdomain, requestURI)
				return nil
			}
		}

	case <-time.After(30 * time.Second):
		i.logger.RequestTimeout(requestId, subdomain, requestURI)
		return &cerrors.TimeoutError{Message: "timeout waiting for tunnel response"}
	}

	return nil
}

func (i *IskndrServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK"))
}
