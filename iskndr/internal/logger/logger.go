package logger

import (
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func Initialize(logToStdout bool) {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	if !logToStdout {
		zerolog.SetGlobalLevel(zerolog.Disabled)
		return
	}

	log.Logger = log.Output(zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: time.RFC3339,
	})
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
}

func TunnelStarting(destination string, serverURL string) {
	log.Info().
		Str("local_destination", destination).
		Str("server", serverURL).
		Msg("Starting tunnel")
}

func TunnelConnected(subdomain string) {
	log.Info().
		Str("public_url", subdomain).
		Msg("Tunnel connected")
}

func TunnelDisconnected(err error) {
	log.Info().
		Err(err).
		Msg("Tunnel disconnected")
}

func RequestReceived(requestID, method, path string) {
	log.Debug().
		Str("request_id", requestID).
		Str("method", method).
		Str("path", path).
		Msg("Request received from tunnel")
}

func ForwardingToLocal(requestID, method, localURL string) {
	log.Debug().
		Str("request_id", requestID).
		Str("method", method).
		Str("local_url", localURL).
		Msg("Forwarding to local app")
}

func LocalRequestFailed(requestID string, err error) {
	log.Error().
		Err(err).
		Str("request_id", requestID).
		Msg("Failed to reach local app")
}

func LocalResponseReceived(requestID string, status int, bodySize int) {
	log.Debug().
		Str("request_id", requestID).
		Int("status", status).
		Int("body_bytes", bodySize).
		Msg("Response from local app")
}

func StreamingResponse(requestID string, chunkSize int, done bool) {
	log.Debug().
		Str("request_id", requestID).
		Int("chunk_bytes", chunkSize).
		Bool("done", done).
		Msg("Streaming response chunk")
}

func ResponseSent(requestID string, status int) {
	log.Debug().
		Str("request_id", requestID).
		Int("status", status).
		Msg("Response sent to tunnel")
}

func ResponseSendFailed(requestID string, err error) {
	log.Error().
		Err(err).
		Str("request_id", requestID).
		Msg("Failed to send response to tunnel")
}

func Error(msg string, err error) {
	log.Error().
		Err(err).
		Msg(msg)
}
