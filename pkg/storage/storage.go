package storage

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/King-kin5/analysis/pkg/types"
)

// Storage defines the interface for storing profiling data
type Storage interface {
	SaveSession(session *types.ProfileSession) error
	GetSession(sessionID string) (*types.ProfileSession, error)
	ListSessions(applicationID string) ([]*types.ProfileSession, error)
	DeleteSession(sessionID string) error

	SaveProfileData(data *types.ProfileData) error
	GetProfileData(sessionID string) ([]*types.ProfileData, error)

	SaveMetrics(sessionID string, metrics *types.MetricsSnapshot) error
	GetMetrics(sessionID string) ([]*types.MetricsSnapshot, error)
}

// FileStorage implements Storage using the filesystem
type FileStorage struct {
	basePath string
	mu       sync.RWMutex
}

// NewFileStorage creates a new file based storage
func NewFileStorage(basePath string) (*FileStorage, error) {
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create base directory: %w", err)
	}

	return &FileStorage{
		basePath: basePath,
	}, nil
}

func (fs *FileStorage) SaveSession(session *types.ProfileSession) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	sessionsDir := filepath.Join(fs.basePath, "sessions")
	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		return fmt.Errorf("failed to create sessions directory: %w", err)
	}

	path := filepath.Join(sessionsDir, session.ID+".json")
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write session file: %w", err)
	}

	return nil
}

func (fs *FileStorage) GetSession(sessionID string) (*types.ProfileSession, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	path := filepath.Join(fs.basePath, "sessions", sessionID+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("session not found: %s", sessionID)
		}
		return nil, fmt.Errorf("failed to read session file: %w", err)
	}

	var session types.ProfileSession
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session: %w", err)
	}

	return &session, nil
}

func (fs *FileStorage) ListSessions(applicationID string) ([]*types.ProfileSession, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	sessionsDir := filepath.Join(fs.basePath, "sessions")
	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*types.ProfileSession{}, nil
		}
		return nil, fmt.Errorf("failed to read sessions directory: %w", err)
	}

	var sessions []*types.ProfileSession
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		data, err := os.ReadFile(filepath.Join(sessionsDir, entry.Name()))
		if err != nil {
			// Log warning but continue processing other files
			continue
		}

		var session types.ProfileSession
		if err := json.Unmarshal(data, &session); err != nil {
			// Log warning but continue processing other files
			continue
		}

		if applicationID == "" || session.ApplicationID == applicationID {
			sessions = append(sessions, &session)
		}
	}

	return sessions, nil
}

func (fs *FileStorage) DeleteSession(sessionID string) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	// Delete session file
	sessionPath := filepath.Join(fs.basePath, "sessions", sessionID+".json")
	if err := os.Remove(sessionPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete session file: %w", err)
	}

	// Delete profile data directory
	profileDir := filepath.Join(fs.basePath, "profiles", sessionID)
	if err := os.RemoveAll(profileDir); err != nil {
		return fmt.Errorf("failed to delete profile directory: %w", err)
	}

	// Delete metrics directory
	metricsDir := filepath.Join(fs.basePath, "metrics", sessionID)
	if err := os.RemoveAll(metricsDir); err != nil {
		return fmt.Errorf("failed to delete metrics directory: %w", err)
	}

	return nil
}

func (fs *FileStorage) SaveProfileData(data *types.ProfileData) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	profileDir := filepath.Join(fs.basePath, "profiles", data.SessionID)
	if err := os.MkdirAll(profileDir, 0755); err != nil {
		return fmt.Errorf("failed to create profile directory: %w", err)
	}

	// Save the profile data
	timestamp := data.Timestamp.Format("20060102_150405")
	filename := fmt.Sprintf("%s_%s.pprof", data.Type, timestamp)
	profilePath := filepath.Join(profileDir, filename)

	if err := os.WriteFile(profilePath, data.Data, 0644); err != nil {
		return fmt.Errorf("failed to write profile data: %w", err)
	}

	// Save metadata
	metaFilename := fmt.Sprintf("%s_%s.meta.json", data.Type, timestamp)
	metaPath := filepath.Join(profileDir, metaFilename)

	metaData := map[string]interface{}{
		"session_id":   data.SessionID,
		"type":         data.Type,
		"timestamp":    data.Timestamp,
		"sample_rate":  data.SampleRate,
		"sample_count": data.SampleCount,
		"metadata":     data.Metadata,
		"file":         filename,
	}

	metaBytes, err := json.MarshalIndent(metaData, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	if err := os.WriteFile(metaPath, metaBytes, 0644); err != nil {
		return fmt.Errorf("failed to write metadata: %w", err)
	}

	return nil
}

func (fs *FileStorage) GetProfileData(sessionID string) ([]*types.ProfileData, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	profileDir := filepath.Join(fs.basePath, "profiles", sessionID)
	entries, err := os.ReadDir(profileDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*types.ProfileData{}, nil
		}
		return nil, fmt.Errorf("failed to read profile directory: %w", err)
	}

	var profiles []*types.ProfileData
	for _, entry := range entries {
		// Only process metadata files
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".meta.json") {
			continue
		}

		metaPath := filepath.Join(profileDir, entry.Name())
		metaBytes, err := os.ReadFile(metaPath)
		if err != nil {
			continue
		}

		var meta map[string]interface{}
		if err := json.Unmarshal(metaBytes, &meta); err != nil {
			continue
		}

		// Read the actual profile data
		profileFile, ok := meta["file"].(string)
		if !ok {
			continue
		}

		profilePath := filepath.Join(profileDir, profileFile)
		profileBytes, err := os.ReadFile(profilePath)
		if err != nil {
			continue
		}

		timestamp, _ := time.Parse(time.RFC3339, meta["timestamp"].(string))
		profileData := &types.ProfileData{
			SessionID:   sessionID,
			Type:        types.ProfileType(meta["type"].(string)),
			Timestamp:   timestamp,
			Data:        profileBytes,
			SampleCount: int64(meta["sample_count"].(float64)),
		}

		if sampleRate, ok := meta["sample_rate"].(float64); ok {
			profileData.SampleRate = int(sampleRate)
		}

		if metadata, ok := meta["metadata"].(map[string]interface{}); ok {
			profileData.Metadata = metadata
		}

		profiles = append(profiles, profileData)
	}

	return profiles, nil
}

func (fs *FileStorage) SaveMetrics(sessionID string, metrics *types.MetricsSnapshot) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	metricsDir := filepath.Join(fs.basePath, "metrics", sessionID)
	if err := os.MkdirAll(metricsDir, 0755); err != nil {
		return fmt.Errorf("failed to create metrics directory: %w", err)
	}

	// Append to metrics file (JSONL format - one JSON object per line)
	metricsFile := filepath.Join(metricsDir, "metrics.jsonl")
	f, err := os.OpenFile(metricsFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open metrics file: %w", err)
	}
	defer f.Close()

	data, err := json.Marshal(metrics)
	if err != nil {
		return fmt.Errorf("failed to marshal metrics: %w", err)
	}

	if _, err := f.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("failed to write metrics: %w", err)
	}

	return nil
}

// GetMetrics reads metrics from JSONL file - FIXED VERSION
func (fs *FileStorage) GetMetrics(sessionID string) ([]*types.MetricsSnapshot, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	metricsFile := filepath.Join(fs.basePath, "metrics", sessionID, "metrics.jsonl")
	f, err := os.Open(metricsFile)
	if err != nil {
		if os.IsNotExist(err) {
			return []*types.MetricsSnapshot{}, nil
		}
		return nil, fmt.Errorf("failed to open metrics file: %w", err)
	}
	defer f.Close()

	var metrics []*types.MetricsSnapshot
	scanner := bufio.NewScanner(f)
	
	// Set a larger buffer size for scanner if needed (default is 64KB)
	const maxScanTokenSize = 1024 * 1024 // 1MB
	buf := make([]byte, maxScanTokenSize)
	scanner.Buffer(buf, maxScanTokenSize)

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		
		// Skip empty lines
		if line == "" {
			continue
		}

		var m types.MetricsSnapshot
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			// Log the error but continue processing other lines
			// In production, you'd want proper logging here
			fmt.Fprintf(os.Stderr, "Warning: failed to parse metrics line %d: %v\n", lineNum, err)
			continue
		}
		
		metrics = append(metrics, &m)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading metrics file: %w", err)
	}

	return metrics, nil
}