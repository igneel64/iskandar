package commands

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gorilla/websocket"
	"github.com/igneel64/iskandar/shared"
	"github.com/igneel64/iskandar/shared/protocol"
	"github.com/igneel64/iskndr/cli/internal/config"
	"github.com/igneel64/iskndr/cli/internal/logger"
	"github.com/igneel64/iskndr/cli/internal/ui"
	"github.com/spf13/cobra"
)

func newTunnelCommand() *cobra.Command {
	var enableLogging bool
	var serverUrl string

	tunnelCmd := &cobra.Command{
		Use:   "tunnel <destination>",
		Short: "Expose a local application to the internet",
		Long: `This command creates a tunnel to your local application, making it accessible from the internet.

The destination can be specified as:
  - port number only (e.g., '8080') - defaults to localhost:8080
  - host:port (e.g., 'foo.bar:80') - connects to the specified host and port`,
		Args:                  cobra.ExactArgs(1),
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			logger.Initialize(enableLogging)

			destinationAddress, err := config.ParseDestination(args[0])
			if err != nil {
				return err
			}

			serverWSUrl, err := config.ParseServerURL(serverUrl)
			if err != nil {
				return err
			}

			logger.TunnelStarting(destinationAddress, serverWSUrl)

			c, _, err := websocket.DefaultDialer.Dial(serverWSUrl, nil)
			if err != nil {
				logger.TunnelDisconnected(err)
				return fmt.Errorf("failed to connect to websocket: %w", err)
			}
			defer c.Close()
			safeWriteConn := shared.NewSafeWebSocketConn(c)

			var regMsg protocol.RegisterTunnelMessage
			if err = c.ReadJSON(&regMsg); err != nil {
				logger.TunnelDisconnected(err)
				return fmt.Errorf("failed to read register tunnel message: %w", err)
			}

			logger.TunnelConnected(regMsg.Subdomain)

			// Setup signal handler for Ctrl+C
			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, os.Interrupt)

			var program *tea.Program
			if !enableLogging {
				program = ui.InitUi(destinationAddress, serverUrl, regMsg.Subdomain)
			}

			// Handle Ctrl+C gracefully
			go func() {
				<-sigChan
				fmt.Println("\nShutting down tunnel...")
				if program != nil {
					program.Quit()
				}
				c.Close()
			}()

			for {
				var requestMsg protocol.Message
				if err := c.ReadJSON(&requestMsg); err != nil {
					// Check if error is a normal WebSocket close or local connection closure
					if websocket.IsCloseError(err, websocket.CloseNormalClosure) ||
						errors.Is(err, net.ErrClosed) {
						return nil
					}
					logger.TunnelDisconnected(err)
					return fmt.Errorf("failed to read request message: %w", err)
				}
				logger.RequestReceived(requestMsg.Id, requestMsg.Method, requestMsg.Path)

				go sendResponse(safeWriteConn, &requestMsg, destinationAddress)
			}
		},
	}

	tunnelCmd.Flags().StringVar(&serverUrl, "server", "", "Tunnel server URL (e.g., localhost:8080, https://tunnel.example.com).")
	tunnelCmd.Flags().BoolVar(&enableLogging, "logging", false, "Enable structured logging to stdout")
	if err := tunnelCmd.MarkFlagRequired("server"); err != nil {
		panic(err)
	}

	return tunnelCmd
}

func sendResponse(c *shared.SafeWebSocketConn, requestMsg *protocol.Message, destinationAddress string) {
	logger.ForwardingToLocal(requestMsg.Id, requestMsg.Method, destinationAddress+requestMsg.Path)

	req, err := http.NewRequest(requestMsg.Method, destinationAddress+requestMsg.Path, bytes.NewReader(requestMsg.Body))

	if err != nil {
		logger.ResponseSendFailed(requestMsg.Id, err)
		c.WriteJSON(&protocol.Message{
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
		c.WriteJSON(&protocol.Message{
			Type:   "response",
			Id:     requestMsg.Id,
			Status: http.StatusBadGateway,
			Body:   []byte(fmt.Sprintf("Failed to reach local app: %v", err)),
		})
		return
	}

	/* Used for not re-sending extra data, mostly headers, which can be pretty big if response is not done. */
	firstChunk := true

	byteBuffer := make([]byte, 32*1024)
	for {
		byteCount, err := res.Body.Read(byteBuffer)

		if err != nil && err != io.EOF {
			if firstChunk {
				logger.ResponseSendFailed(requestMsg.Id, err)
				c.WriteJSON(&protocol.Message{
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

			if err = c.WriteJSON(&responseMsg); err != nil {
				logger.ResponseSendFailed(requestMsg.Id, err)
			} else if responseMsg.Done {
				logger.ResponseSent(requestMsg.Id, responseMsg.Status)
			}
		}

		if err == io.EOF {
			break
		}
	}

	res.Body.Close()
}
