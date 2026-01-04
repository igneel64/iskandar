package logger

import (
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func Initialize(dev bool) {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	if dev {
		log.Logger = log.Output(zerolog.ConsoleWriter{
			Out:        os.Stderr,
			TimeFormat: time.RFC3339,
		})
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}
}

func ServerStarted(port int) {
	log.Info().
		Int("port", port).
		Msg("Tunnel server started")
}

func ServerShutdown() {
	log.Info().Msg("Server shutting down")
}

func TunnelConnected(subdomain, remoteAddr string) {
	log.Info().
		Str("subdomain", subdomain).
		Str("remote_addr", remoteAddr).
		Msg("Tunnel connected")
}

func TunnelDisconnected(subdomain string, err error) {
	log.Info().
		Str("subdomain", subdomain).
		Err(err).
		Msg("Tunnel disconnected")
}

func TunnelRegistrationFailed(err error) {
	log.Error().
		Err(err).
		Msg("Failed to register tunnel connection")
}

func HTTPRequestReceived(subdomain, method, path, remoteAddr string) {
	log.Debug().
		Str("subdomain", subdomain).
		Str("method", method).
		Str("path", path).
		Str("remote_addr", remoteAddr).
		Msg("HTTP request received")
}

func TunnelNotFound(subdomain, host string) {
	log.Warn().
		Str("subdomain", subdomain).
		Str("host", host).
		Msg("Tunnel not found")
}

func RequestForwarded(requestID, subdomain string) {
	log.Debug().
		Str("request_id", requestID).
		Str("subdomain", subdomain).
		Msg("Request forwarded to tunnel")
}

func RequestForwardFailed(requestID, subdomain string, err error) {
	log.Error().
		Err(err).
		Str("request_id", requestID).
		Str("subdomain", subdomain).
		Msg("Failed to forward request to tunnel")
}

func HTTPResponse(subdomain, method, path string, status int, duration time.Duration, requestID string) {
	log.Info().
		Str("subdomain", subdomain).
		Str("method", method).
		Str("path", path).
		Int("status", status).
		Dur("duration_ms", duration).
		Str("request_id", requestID).
		Msg("HTTP response")
}

func StreamingStarted(requestID string, status int, bodySize int) {
	log.Debug().
		Str("request_id", requestID).
		Int("status", status).
		Int("initial_bytes", bodySize).
		Msg("Streaming response started")
}

func StreamingChunk(requestID string, chunkSize int, totalDuration time.Duration) {
	log.Debug().
		Str("request_id", requestID).
		Int("chunk_bytes", chunkSize).
		Dur("elapsed_ms", totalDuration).
		Msg("Streaming chunk sent")
}

func StreamingCompleted(requestID string, totalDuration time.Duration) {
	log.Info().
		Str("request_id", requestID).
		Dur("total_duration_ms", totalDuration).
		Msg("Streaming response completed")
}

func ChannelClosed(requestID string, duration time.Duration) {
	log.Warn().
		Str("request_id", requestID).
		Dur("duration", duration).
		Msg("Response channel closed")
}

func RequestTimeout(requestID, subdomain, path string) {
	log.Warn().
		Str("request_id", requestID).
		Str("subdomain", subdomain).
		Str("path", path).
		Msg("Request timeout")
}

func Error(msg string, err error) {
	log.Error().
		Err(err).
		Msg(msg)
}
