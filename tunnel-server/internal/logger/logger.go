package logger

import (
	"os"
	"time"

	"github.com/rs/zerolog"
)

type Logger interface {
	ServerStarted(port int)
	TunnelConnected(subdomain, remoteAddr string)
	TunnelDisconnected(subdomain string, err error)
	TunnelRegistrationFailed(err error)
	HTTPRequestReceived(subdomain, method, path, remoteAddr string)
	TunnelNotFound(subdomain, host string)
	RequestForwarded(requestID, requestURI, subdomain string)
	RequestForwardFailed(requestID, subdomain string, err error)
	HTTPResponse(subdomain, method, path string, status int, duration time.Duration, requestID string)
	StreamingStarted(requestID string, status int, bodySize int)
	StreamingChunk(requestID string, chunkSize int, totalDuration time.Duration)
	StreamingCompleted(requestID string, totalDuration time.Duration)
	ChannelClosed(requestID string, duration time.Duration)
	RequestTimeout(requestID, subdomain, path string)
	ResponseWriteFailed(requestID string, bytesExpected, bytesWritten int, err error)
	WebSocketCloseFailed(subdomain string, err error)
	MaxTunnelsReached()
	MaxRequestsPerTunnelReached(subdomain string)
	RequestRegistrationFailed(requestId, subdomain string, err error)
	RequestBodyTooLarge(subdomain, path string)
	PanicRecovered(path string, panicValue interface{})
}

type ZerologLogger struct {
	log zerolog.Logger
}

func NewLogger(enabled bool) Logger {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	var logger zerolog.Logger
	if enabled {
		logger = zerolog.New(zerolog.ConsoleWriter{
			Out:        os.Stderr,
			TimeFormat: time.RFC3339,
		}).With().Timestamp().Logger().Level(zerolog.InfoLevel)
	} else {
		logger = zerolog.New(os.Stderr).Level(zerolog.Disabled)
	}

	return &ZerologLogger{log: logger}
}

func (l *ZerologLogger) ServerStarted(port int) {
	l.log.Info().
		Int("port", port).
		Msg("Tunnel server started")
}

func (l *ZerologLogger) TunnelConnected(subdomain, remoteAddr string) {
	l.log.Info().
		Str("subdomain", subdomain).
		Str("remote_addr", remoteAddr).
		Msg("Tunnel connected")
}

func (l *ZerologLogger) TunnelDisconnected(subdomain string, err error) {
	l.log.Info().
		Str("subdomain", subdomain).
		Err(err).
		Msg("Tunnel disconnected")
}

func (l *ZerologLogger) TunnelRegistrationFailed(err error) {
	l.log.Error().
		Err(err).
		Msg("Failed to register tunnel connection")
}

func (l *ZerologLogger) HTTPRequestReceived(subdomain, method, path, remoteAddr string) {
	l.log.Info().
		Str("subdomain", subdomain).
		Str("method", method).
		Str("path", path).
		Str("remote_addr", remoteAddr).
		Msg("HTTP request received")
}

func (l *ZerologLogger) TunnelNotFound(subdomain, host string) {
	l.log.Warn().
		Str("subdomain", subdomain).
		Str("host", host).
		Msg("Tunnel not found")
}

func (l *ZerologLogger) RequestForwarded(requestID, requestURI, subdomain string) {
	l.log.Info().
		Str("request_id", requestID).
		Str("request_uri", requestURI).
		Str("subdomain", subdomain).
		Msg("Request forwarded to tunnel")
}

func (l *ZerologLogger) RequestForwardFailed(requestID, subdomain string, err error) {
	l.log.Error().
		Err(err).
		Str("request_id", requestID).
		Str("subdomain", subdomain).
		Msg("Failed to forward request to tunnel")
}

func (l *ZerologLogger) HTTPResponse(subdomain, method, path string, status int, duration time.Duration, requestID string) {
	l.log.Info().
		Str("subdomain", subdomain).
		Str("method", method).
		Str("path", path).
		Int("status", status).
		Dur("duration_ms", duration).
		Str("request_id", requestID).
		Msg("HTTP response")
}

func (l *ZerologLogger) StreamingStarted(requestID string, status int, bodySize int) {
	l.log.Info().
		Str("request_id", requestID).
		Int("status", status).
		Int("initial_bytes", bodySize).
		Msg("Streaming response started")
}

func (l *ZerologLogger) StreamingChunk(requestID string, chunkSize int, totalDuration time.Duration) {
	l.log.Info().
		Str("request_id", requestID).
		Int("chunk_bytes", chunkSize).
		Dur("elapsed_ms", totalDuration).
		Msg("Streaming chunk sent")
}

func (l *ZerologLogger) StreamingCompleted(requestID string, totalDuration time.Duration) {
	l.log.Info().
		Str("request_id", requestID).
		Dur("total_duration_ms", totalDuration).
		Msg("Streaming response completed")
}

func (l *ZerologLogger) ChannelClosed(requestID string, duration time.Duration) {
	l.log.Warn().
		Str("request_id", requestID).
		Dur("duration", duration).
		Msg("Response channel closed")
}

func (l *ZerologLogger) RequestTimeout(requestID, subdomain, path string) {
	l.log.Warn().
		Str("request_id", requestID).
		Str("subdomain", subdomain).
		Str("path", path).
		Msg("Request timeout")
}

func (l *ZerologLogger) ResponseWriteFailed(requestID string, bytesExpected, bytesWritten int, err error) {
	l.log.Info().
		Str("request_id", requestID).
		Int("bytes_expected", bytesExpected).
		Int("bytes_written", bytesWritten).
		Err(err).
		Msg("Failed to write response to client")
}

func (l *ZerologLogger) WebSocketCloseFailed(subdomain string, err error) {
	l.log.Info().
		Str("subdomain", subdomain).
		Err(err).
		Msg("Failed to close WebSocket connection")
}

func (l *ZerologLogger) MaxTunnelsReached() {
	l.log.Error().
		Msg("Maximum number of tunnels reached")
}

func (l *ZerologLogger) MaxRequestsPerTunnelReached(subdomain string) {
	l.log.Error().
		Str("subdomain", subdomain).
		Msg("Maximum number of concurrent requests per tunnel reached")
}

func (l *ZerologLogger) RequestRegistrationFailed(requestId, subdomain string, err error) {
	l.log.Error().
		Str("request_id", requestId).
		Str("subdomain", subdomain).
		Err(err).
		Msg("Failed to register request")
}

func (l *ZerologLogger) RequestBodyTooLarge(subdomain, path string) {
	l.log.Error().
		Str("subdomain", subdomain).
		Str("path", path).
		Msg("Request body too large")
}

func (l *ZerologLogger) PanicRecovered(path string, panicValue interface{}) {
	l.log.Error().
		Str("path", path).
		Interface("panic_value", panicValue).
		Msg("Panic recovered in HTTP handler")
}
