// Package ingestion provides a Unix socket HTTP server for receiving tool events.
package ingestion

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/2lines/trajectory-memory/internal/store"
	"github.com/2lines/trajectory-memory/internal/types"
)

// HookPayload represents the Claude Code PostToolUse hook payload.
type HookPayload struct {
	SessionID  string          `json:"session_id"`
	ToolName   string          `json:"tool_name"`
	ToolInput  json.RawMessage `json:"tool_input"`
	ToolOutput json.RawMessage `json:"tool_output"`
	DurationMs int64           `json:"duration_ms"`
}

// Server handles incoming step events from hook scripts.
type Server struct {
	store      store.Store
	socketPath string
	listener   net.Listener
	server     *http.Server
	mu         sync.RWMutex
	running    bool
}

// NewServer creates a new ingestion server.
func NewServer(s store.Store, socketPath string) *Server {
	return &Server{
		store:      s,
		socketPath: socketPath,
	}
}

// Start begins listening on the Unix socket.
func (s *Server) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("server already running")
	}

	// Remove existing socket file if present
	if err := os.Remove(s.socketPath); err != nil && !os.IsNotExist(err) {
		s.mu.Unlock()
		return fmt.Errorf("failed to remove existing socket: %w", err)
	}

	// Create socket directory if needed
	socketDir := filepath.Dir(s.socketPath)
	if err := os.MkdirAll(socketDir, 0755); err != nil {
		s.mu.Unlock()
		return fmt.Errorf("failed to create socket directory: %w", err)
	}

	listener, err := net.Listen("unix", s.socketPath)
	if err != nil {
		s.mu.Unlock()
		return fmt.Errorf("failed to listen on socket: %w", err)
	}

	// Set socket permissions
	if err := os.Chmod(s.socketPath, 0666); err != nil {
		listener.Close()
		s.mu.Unlock()
		return fmt.Errorf("failed to set socket permissions: %w", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/step", s.handleStep)
	mux.HandleFunc("/health", s.handleHealth)

	s.listener = listener
	s.server = &http.Server{Handler: mux}
	s.running = true
	s.mu.Unlock()

	// Start server in background
	go func() {
		if err := s.server.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Printf("ingestion server error: %v", err)
		}
	}()

	// Handle graceful shutdown
	go func() {
		<-ctx.Done()
		s.Stop()
	}()

	return nil
}

// Stop gracefully shuts down the server.
func (s *Server) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	s.running = false

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("server shutdown error: %w", err)
	}

	// Clean up socket file
	os.Remove(s.socketPath)

	return nil
}

// IsRunning returns whether the server is currently running.
func (s *Server) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// handleHealth responds to health check requests.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

// handleStep processes incoming tool invocation events.
func (s *Server) handleStep(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse payload
	var payload HookPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		log.Printf("failed to decode payload: %v", err)
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if payload.ToolName == "" {
		http.Error(w, "tool_name required", http.StatusBadRequest)
		return
	}

	// Get active session
	session, err := s.store.GetActiveSession()
	if err != nil {
		if err == store.ErrNoActiveSession {
			http.Error(w, "no active session", http.StatusNotFound)
			return
		}
		log.Printf("failed to get active session: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Create trajectory step
	step := types.TrajectoryStep{
		Timestamp:     time.Now(),
		ToolName:      payload.ToolName,
		InputSummary:  summarizeJSON(payload.ToolInput),
		OutputSummary: summarizeJSON(payload.ToolOutput),
		DurationMs:    payload.DurationMs,
	}

	// Append step to session
	if err := s.store.AppendStep(session.ID, step); err != nil {
		log.Printf("failed to append step: %v", err)
		http.Error(w, "failed to append step", http.StatusInternalServerError)
		return
	}

	// If Read tool targets .md file, update LoadedContext
	if payload.ToolName == "Read" {
		filePath := extractFilePath(payload.ToolInput)
		if strings.HasSuffix(strings.ToLower(filePath), ".md") {
			if err := s.appendLoadedContext(session.ID, filePath); err != nil {
				log.Printf("failed to update loaded context: %v", err)
				// Don't fail the request for this
			}
		}
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

// appendLoadedContext adds a file path to the session's LoadedContext.
func (s *Server) appendLoadedContext(sessionID string, filePath string) error {
	session, err := s.store.GetSession(sessionID)
	if err != nil {
		return err
	}

	// Check if already in context
	for _, ctx := range session.LoadedContext {
		if ctx == filePath {
			return nil // Already present
		}
	}

	session.LoadedContext = append(session.LoadedContext, filePath)
	return s.store.UpdateSession(session)
}

// summarizeJSON converts JSON to a string summary, truncated to max length.
func summarizeJSON(data json.RawMessage) string {
	if len(data) == 0 {
		return ""
	}

	// Try to unmarshal as a map to extract key fields
	var obj map[string]interface{}
	if err := json.Unmarshal(data, &obj); err == nil {
		// For Read/Write tools, extract file_path
		if filePath, ok := obj["file_path"].(string); ok {
			return types.TruncateString(filePath, types.MaxInputSummaryLen)
		}
		// For Bash, extract command
		if cmd, ok := obj["command"].(string); ok {
			return types.TruncateString(cmd, types.MaxInputSummaryLen)
		}
	}

	// Fall back to raw string, truncated
	s := string(data)
	// Remove surrounding quotes if present
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
	}
	return types.TruncateString(s, types.MaxInputSummaryLen)
}

// extractFilePath extracts the file_path from a tool input JSON.
func extractFilePath(data json.RawMessage) string {
	var obj map[string]interface{}
	if err := json.Unmarshal(data, &obj); err != nil {
		return ""
	}
	if filePath, ok := obj["file_path"].(string); ok {
		return filePath
	}
	return ""
}
