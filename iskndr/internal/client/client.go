package client

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"

	ws "github.com/gorilla/websocket"
	"github.com/igneel64/iskandar/iskndr/internal/logger"
	"github.com/igneel64/iskandar/shared"
	"github.com/igneel64/iskandar/shared/protocol"
)

type Client interface {
	Register() (*protocol.RegisterTunnelMessage, error)
	AcceptRequests() error
}

type IskndrClient struct {
	wsConnection *shared.SafeWebSocketConn
}

func NewIskndrClient(wsConnection *shared.SafeWebSocketConn) *IskndrClient {
	return &IskndrClient{
		wsConnection: wsConnection,
	}
}

func (i *IskndrClient) Register() (*protocol.RegisterTunnelMessage, error) {
	var regMsg protocol.RegisterTunnelMessage
	if err := i.wsConnection.ReadRegistrationMsg(&regMsg); err != nil {
		return nil, err
	}
	return &regMsg, nil
}

func (i *IskndrClient) AcceptRequests(destinationAddress string) error {
	for {
		var requestMsg protocol.Message
		if err := i.wsConnection.ReadJSON(&requestMsg); err != nil {
			if ws.IsCloseError(err, ws.CloseNormalClosure) ||
				errors.Is(err, net.ErrClosed) {
				return nil
			}
			logger.TunnelDisconnected(err)
			return fmt.Errorf("failed to read request message: %w", err)
		}
		logger.RequestReceived(requestMsg.Id, requestMsg.Method, requestMsg.Path)

		go i.sendResponse(&requestMsg, destinationAddress)
	}
}

func (i *IskndrClient) sendResponse(requestMsg *protocol.Message, destinationAddress string) {
	logger.ForwardingToLocal(requestMsg.Id, requestMsg.Method, destinationAddress+requestMsg.Path)

	req, err := http.NewRequest(requestMsg.Method, destinationAddress+requestMsg.Path, bytes.NewReader(requestMsg.Body))

	if err != nil {
		logger.ResponseSendFailed(requestMsg.Id, err)
		_ = i.wsConnection.WriteJSON(&protocol.Message{
			Type:   "response",
			Id:     requestMsg.Id,
			Status: http.StatusInternalServerError,
			Body:   []byte("Failed to create request"),
		})
		return
	}

	for k, v := range requestMsg.Headers {
		req.Header.Set(k, v)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		logger.LocalRequestFailed(requestMsg.Id, err)
		_ = i.wsConnection.WriteJSON(&protocol.Message{
			Type:   "response",
			Id:     requestMsg.Id,
			Status: http.StatusBadGateway,
			Body:   []byte(fmt.Sprintf("Failed to reach local app: %v", err)),
		})
		return
	}

	//nolint:errcheck
	defer res.Body.Close()

	/* Used for not re-sending extra data, mostly headers, which can be pretty big if response is not done. */
	firstChunk := true

	byteBuffer := make([]byte, 32*1024)
	for {
		byteCount, err := res.Body.Read(byteBuffer)

		if err != nil && err != io.EOF {
			if firstChunk {
				logger.ResponseSendFailed(requestMsg.Id, err)
				_ = i.wsConnection.WriteJSON(&protocol.Message{
					Type:   "response",
					Id:     requestMsg.Id,
					Status: http.StatusBadGateway,
					Body:   []byte(fmt.Sprintf("Failed to read response body: %v", err)),
					Done:   true,
				})
			} else {
				// Already sent status - just log and abort
				logger.Error("Error reading response body mid-stream", err)
			}
			break
		}

		if firstChunk || byteCount > 0 {
			responseMsg := protocol.Message{
				Type: "response",
				Id:   requestMsg.Id,
				Body: byteBuffer[:byteCount],
				Done: err == io.EOF,
			}

			if firstChunk {
				responseMsg.Status = res.StatusCode
				responseMsg.Headers = shared.SerializeHeaders(res.Header)
				logger.LocalResponseReceived(requestMsg.Id, res.StatusCode, byteCount)
				firstChunk = false
			} else {
				logger.StreamingResponse(requestMsg.Id, byteCount, err == io.EOF)
			}

			if err = i.wsConnection.WriteJSON(&responseMsg); err != nil {
				logger.ResponseSendFailed(requestMsg.Id, err)
				break
			} else if responseMsg.Done {
				logger.ResponseSent(requestMsg.Id, responseMsg.Status)
				break
			}
		}

		if err == io.EOF {
			break
		}
	}

}
