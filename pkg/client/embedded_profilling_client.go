package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"runtime/pprof"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/process"

	"github.com/King-kin5/analysis/pkg/types"
	"go.uber.org/zap"
)

// Client is the embedded profiling client
type Client struct {
	config    types.AgentConfig
	logger    *zap.Logger
	httpClient *http.Client
	sessions  map[string]*profilingSession
	mu        sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc
}

type profilingSession struct {
	session    types.ProfileSession
	cpuFile    io.WriteCloser
	cancel     context.CancelFunc
	collecting bool
}

// NewClient creates a new embedded profiling client
func NewClient(config types.AgentConfig) (*Client, error) {
	logger, _ := zap.NewProduction()
	
	ctx, cancel := context.WithCancel(context.Background())
	
	client := &Client{
		config: config,
		logger: logger,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		sessions: make(map[string]*profilingSession),
		ctx:      ctx,
		cancel:   cancel,
	}

	if config.AutoProfile {
		go client.autoProfile()
	}

	return client, nil
}

// StartProfiling starts a new profiling session
func (c *Client) StartProfiling(ctx context.Context, config types.ProfilingConfig) (string, error) {
	sessionID := fmt.Sprintf("%s_%d", c.config.ApplicationID, time.Now().UnixNano())
	
	session := types.ProfileSession{
		ID:            sessionID,
		ApplicationID: c.config.ApplicationID,
		Name:          c.config.ApplicationName,
		Language:      c.config.Language,
		StartTime:     time.Now(),
		ProfileType:   types.ProfileTypeCPU, // Default
		Mode:          c.config.Mode,
		Metadata: map[string]interface{}{
			"go_version": runtime.Version(),
			"os":         runtime.GOOS,
			"arch":       runtime.GOARCH,
		},
	}

	ps := &profilingSession{
		session:    session,
		collecting: true,
	}

	c.mu.Lock()
	c.sessions[sessionID] = ps
	c.mu.Unlock()

	// Start profiling based on types requested
	for _, profileType := range config.ProfileTypes {
		switch profileType {
		case types.ProfileTypeCPU:
			if err := c.startCPUProfile(ctx, ps); err != nil {
				c.logger.Error("Failed to start CPU profiling", zap.Error(err))
			}
		case types.ProfileTypeMemory, types.ProfileTypeHeap:
			go c.collectMemoryProfile(ctx, ps, config)
		case types.ProfileTypeIO:
			go c.collectIOProfile(ctx, ps, config)
		}
	}

	// Collect metrics if enabled
	if config.CollectMetrics {
		go c.collectMetrics(ctx, sessionID, config.MetricsInterval)
	}

	// Stop profiling after duration
	if config.Duration > 0 {
		time.AfterFunc(config.Duration, func() {
			c.StopProfiling(sessionID)
		})
	}

	return sessionID, nil
}

// StopProfiling stops a profiling session
func (c *Client) StopProfiling(sessionID string) error {
	c.mu.Lock()
	ps, exists := c.sessions[sessionID]
	if !exists {
		c.mu.Unlock()
		return fmt.Errorf("session not found: %s", sessionID)
	}
	delete(c.sessions, sessionID)
	c.mu.Unlock()

	ps.collecting = false
	if ps.cancel != nil {
		ps.cancel()
	}

	if ps.cpuFile != nil {
		pprof.StopCPUProfile()
		ps.cpuFile.Close()
	}

	ps.session.EndTime = time.Now()
	ps.session.Duration = ps.session.EndTime.Sub(ps.session.StartTime)

	// Send session data to server
	return c.sendSession(ps.session)
}

func (c *Client) startCPUProfile(ctx context.Context, ps *profilingSession) error {
	var buf bytes.Buffer
	ps.cpuFile = nopCloser{&buf}
	
	if err := pprof.StartCPUProfile(ps.cpuFile); err != nil {
		return err
	}

	sessionCtx, cancel := context.WithCancel(ctx)
	ps.cancel = cancel

	go func() {
		<-sessionCtx.Done()
		pprof.StopCPUProfile()
		
		// Send CPU profile data
		profileData := types.ProfileData{
			SessionID:   ps.session.ID,
			Type:        types.ProfileTypeCPU,
			Timestamp:   time.Now(),
			Data:        buf.Bytes(),
			SampleCount: int64(buf.Len()),
		}
		c.sendProfileData(profileData)
	}()

	return nil
}

func (c *Client) collectMemoryProfile(ctx context.Context, ps *profilingSession, config types.ProfilingConfig) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !ps.collecting {
				return
			}

			var buf bytes.Buffer
			if err := pprof.WriteHeapProfile(&buf); err != nil {
				c.logger.Error("Failed to collect heap profile", zap.Error(err))
				continue
			}

			profileData := types.ProfileData{
				SessionID:   ps.session.ID,
				Type:        types.ProfileTypeHeap,
				Timestamp:   time.Now(),
				Data:        buf.Bytes(),
				SampleCount: int64(buf.Len()),
			}
			c.sendProfileData(profileData)
		}
	}
}

func (c *Client) collectIOProfile(ctx context.Context, ps *profilingSession, config types.ProfilingConfig) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	// Get current process
	pid := int32(runtime.GOMAXPROCS(0)) // This is a placeholder
	proc, err := process.NewProcess(pid)
	if err != nil {
		// If we can't get process, try to get current process ID
		proc, err = process.NewProcess(int32(os.Getpid()))
		if err != nil {
			c.logger.Error("Failed to get process for I/O profiling", zap.Error(err))
			return
		}
	}

	// Track previous I/O stats for delta calculation
	var prevReadBytes, prevWriteBytes uint64

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !ps.collecting {
				return
			}

			// Collect I/O statistics
			ioCounters, err := proc.IOCounters()
			if err != nil {
				c.logger.Error("Failed to collect I/O stats", zap.Error(err))
				continue
			}

			// Calculate deltas
			readDelta := ioCounters.ReadBytes - prevReadBytes
			writeDelta := ioCounters.WriteBytes - prevWriteBytes

			prevReadBytes = ioCounters.ReadBytes
			prevWriteBytes = ioCounters.WriteBytes

			// Create I/O profile data
			ioData := map[string]interface{}{
				"timestamp":       time.Now(),
				"read_bytes":      ioCounters.ReadBytes,
				"write_bytes":     ioCounters.WriteBytes,
				"read_count":      ioCounters.ReadCount,
				"write_count":     ioCounters.WriteCount,
				"read_delta":      readDelta,
				"write_delta":     writeDelta,
			}

			jsonData, err := json.Marshal(ioData)
			if err != nil {
				c.logger.Error("Failed to marshal I/O data", zap.Error(err))
				continue
			}

			profileData := types.ProfileData{
				SessionID:   ps.session.ID,
				Type:        types.ProfileTypeIO,
				Timestamp:   time.Now(),
				Data:        jsonData,
				Metadata:    ioData,
				SampleCount: 1,
			}

			c.sendProfileData(profileData)
		}
	}
}

func (c *Client) collectMetrics(ctx context.Context, sessionID string, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			var m runtime.MemStats
			runtime.ReadMemStats(&m)

			metrics := types.MetricsSnapshot{
				Timestamp:      time.Now(),
				GoroutineCount: runtime.NumGoroutine(),
				HeapAlloc:      m.HeapAlloc,
				HeapSys:        m.HeapSys,
				GCPauseTotal:   m.PauseTotalNs,
			}

			c.sendMetrics(sessionID, metrics)
		}
	}
}

func (c *Client) sendSession(session types.ProfileSession) error {
	data, err := json.Marshal(session)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/api/v1/sessions", c.config.ServerURL)
	resp, err := c.httpClient.Post(url, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("failed to send session: %d", resp.StatusCode)
	}

	return nil
}

func (c *Client) sendProfileData(data types.ProfileData) error {
	payload, err := json.Marshal(data)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/api/v1/profiles", c.config.ServerURL)
	resp, err := c.httpClient.Post(url, "application/json", bytes.NewBuffer(payload))
	if err != nil {
		c.logger.Error("Failed to send profile data", zap.Error(err))
		return err
	}
	defer resp.Body.Close()

	return nil
}

func (c *Client) sendMetrics(sessionID string, metrics types.MetricsSnapshot) error {
	payload := map[string]interface{}{
		"session_id": sessionID,
		"metrics":    metrics,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/api/v1/metrics", c.config.ServerURL)
	resp, err := c.httpClient.Post(url, "application/json", bytes.NewBuffer(data))
	if err != nil {
		c.logger.Error("Failed to send metrics", zap.Error(err))
		return err
	}
	defer resp.Body.Close()

	return nil
}

func (c *Client) autoProfile() {
	ticker := time.NewTicker(c.config.ProfileInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			config := types.ProfilingConfig{
				ProfileTypes:    []types.ProfileType{types.ProfileTypeCPU, types.ProfileTypeMemory},
				Duration:        30 * time.Second,
				CollectMetrics:  true,
				MetricsInterval: 5 * time.Second,
			}
			
			sessionID, err := c.StartProfiling(c.ctx, config)
			if err != nil {
				c.logger.Error("Auto-profiling failed", zap.Error(err))
			} else {
				c.logger.Info("Auto-profiling started", zap.String("session_id", sessionID))
			}
		}
	}
}

// Close stops the client and cleans up resources
func (c *Client) Close() error {
	c.cancel()
	
	c.mu.Lock()
	for sessionID := range c.sessions {
		c.StopProfiling(sessionID)
	}
	c.mu.Unlock()

	return c.logger.Sync()
}

type nopCloser struct {
	io.Writer
}

func (nopCloser) Close() error { return nil }