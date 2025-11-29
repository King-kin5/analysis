package collector
import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/King-kin5/analysis/pkg/storage"
	"github.com/King-kin5/analysis/pkg/types"
	"go.uber.org/zap"
)
// Collector receives and processes profiling data
type Collector struct {
	storage storage.Storage
	logger  *zap.Logger
	server  *http.Server
	router  *mux.Router
	
	sessions map[string]*types.ProfileSession
	mu       sync.RWMutex
}
// NewCollector creates a new profiling data collector
func NewCollector(store storage.Storage, logger *zap.Logger) *Collector {
	if logger == nil {
		logger, _ = zap.NewProduction()
	}

	c := &Collector{
		storage:  store,
		logger:   logger,
		sessions: make(map[string]*types.ProfileSession),
	}

	c.setupRouter()
	return c
}
func (c *Collector) setupRouter() {
	c.router = mux.NewRouter()

	// API routes
	api := c.router.PathPrefix("/api/v1").Subrouter()
	api.HandleFunc("/sessions", c.handleCreateSession).Methods("POST")
	api.HandleFunc("/sessions", c.handleListSessions).Methods("GET")
	api.HandleFunc("/sessions/{id}", c.handleGetSession).Methods("GET")
	api.HandleFunc("/sessions/{id}", c.handleDeleteSession).Methods("DELETE")
	
	api.HandleFunc("/profiles", c.handleProfileData).Methods("POST")
	api.HandleFunc("/profiles/{session_id}", c.handleGetProfiles).Methods("GET")
	
	api.HandleFunc("/metrics", c.handleMetrics).Methods("POST")
	api.HandleFunc("/metrics/{session_id}", c.handleGetMetrics).Methods("GET")

	// Health check
	c.router.HandleFunc("/health", c.handleHealth).Methods("GET")
}
func (c *Collector) respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (c *Collector) respondError(w http.ResponseWriter, status int, message string) {
	c.respondJSON(w, status, map[string]string{"error": message})
}
// Start starts the collector HTTP server
func (c *Collector) Start(ctx context.Context, addr string) error {
	c.server = &http.Server{
		Addr:    addr,
		Handler: c.router,
	}

	c.logger.Info("Starting collector server", zap.String("addr", addr))

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		c.server.Shutdown(shutdownCtx)
	}()

	if err := c.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}

	return nil
}
func (c *Collector) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	var session types.ProfileSession
	if err := json.NewDecoder(r.Body).Decode(&session); err != nil {
		c.respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	c.mu.Lock()
	c.sessions[session.ID] = &session
	c.mu.Unlock()

	if err := c.storage.SaveSession(&session); err != nil {
		c.logger.Error("Failed to save session", zap.Error(err))
		c.respondError(w, http.StatusInternalServerError, "Failed to save session")
		return
	}

	c.logger.Info("Session created", 
		zap.String("session_id", session.ID),
		zap.String("app_id", session.ApplicationID))

	c.respondJSON(w, http.StatusCreated, session)
}
func (c *Collector) handleListSessions(w http.ResponseWriter, r *http.Request) {
	appID := r.URL.Query().Get("application_id")
	
	sessions, err := c.storage.ListSessions(appID)
	if err != nil {
		c.logger.Error("Failed to list sessions", zap.Error(err))
		c.respondError(w, http.StatusInternalServerError, "Failed to list sessions")
		return
	}

	c.respondJSON(w, http.StatusOK, sessions)
}
func (c *Collector) handleGetSession(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	sessionID := vars["id"]

	session, err := c.storage.GetSession(sessionID)
	if err != nil {
		c.respondError(w, http.StatusNotFound, "Session not found")
		return
	}

	c.respondJSON(w, http.StatusOK, session)
}
func (c *Collector) handleDeleteSession(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	sessionID := vars["id"]

	if err := c.storage.DeleteSession(sessionID); err != nil {
		c.logger.Error("Failed to delete session", zap.Error(err))
		c.respondError(w, http.StatusInternalServerError, "Failed to delete session")
		return
	}

	c.mu.Lock()
	delete(c.sessions, sessionID)
	c.mu.Unlock()

	w.WriteHeader(http.StatusNoContent)
}
func (c *Collector) handleProfileData(w http.ResponseWriter, r *http.Request) {
	var profileData types.ProfileData
	if err := json.NewDecoder(r.Body).Decode(&profileData); err != nil {
		c.respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := c.storage.SaveProfileData(&profileData); err != nil {
		c.logger.Error("Failed to save profile data", zap.Error(err))
		c.respondError(w, http.StatusInternalServerError, "Failed to save profile data")
		return
	}

	c.logger.Debug("Profile data received",
		zap.String("session_id", profileData.SessionID),
		zap.String("type", string(profileData.Type)),
		zap.Int("size", len(profileData.Data)))

	c.respondJSON(w, http.StatusCreated, map[string]string{"status": "ok"})
}
func (c *Collector) handleGetProfiles(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	sessionID := vars["session_id"]

	profiles, err := c.storage.GetProfileData(sessionID)
	if err != nil {
		c.logger.Error("Failed to get profile data", zap.Error(err))
		c.respondError(w, http.StatusInternalServerError, "Failed to get profile data")
		return
	}

	c.respondJSON(w, http.StatusOK, profiles)
}
func (c *Collector) handleMetrics(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		SessionID string                 `json:"session_id"`
		Metrics   types.MetricsSnapshot  `json:"metrics"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		c.respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := c.storage.SaveMetrics(payload.SessionID, &payload.Metrics); err != nil {
		c.logger.Error("Failed to save metrics", zap.Error(err))
		c.respondError(w, http.StatusInternalServerError, "Failed to save metrics")
		return
	}

	c.respondJSON(w, http.StatusCreated, map[string]string{"status": "ok"})
}
func (c *Collector) handleGetMetrics(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	sessionID := vars["session_id"]

	metrics, err := c.storage.GetMetrics(sessionID)
	if err != nil {
		c.logger.Error("Failed to get metrics", zap.Error(err))
		c.respondError(w, http.StatusInternalServerError, "Failed to get metrics")
		return
	}

	c.respondJSON(w, http.StatusOK, metrics)
}

func (c *Collector) handleHealth(w http.ResponseWriter, r *http.Request) {
	c.respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":        "healthy",
		"timestamp":     time.Now(),
		"active_sessions": len(c.sessions),
	})
}
// GetRouter returns the HTTP router for the collector
func (c *Collector) GetRouter() *mux.Router {
	return c.router
}
