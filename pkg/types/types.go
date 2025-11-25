package types

import (
	"time"
)

// ProfileType represents the type of profiling data
type ProfileType string

const (
	ProfileTypeCPU    ProfileType = "cpu"
	ProfileTypeMemory ProfileType = "memory"
	ProfileTypeIO     ProfileType = "io"
	ProfileTypeBlock  ProfileType = "block"
	ProfileTypeMutex  ProfileType = "mutex"
	ProfileTypeHeap   ProfileType = "heap"
)

// ProfileMode represents how the profiling was initiated
type ProfileMode string

const (
	ProfileModeEmbedded ProfileMode = "embedded"
	ProfileModeSidecar  ProfileMode = "sidecar"
)

// ProfileSession represents a profiling session
type ProfileSession struct {
	ID            string                 `json:"id"`
	ApplicationID string                 `json:"application_id"`
	Name          string                 `json:"name"`
	Language      string                 `json:"language"`
	StartTime     time.Time              `json:"start_time"`
	EndTime       time.Time              `json:"end_time"`
	Duration      time.Duration          `json:"duration"`
	ProfileType   ProfileType            `json:"profile_type"`
	Mode          ProfileMode            `json:"mode"`
	Metadata      map[string]interface{} `json:"metadata"`
	DataPath      string                 `json:"data_path"`
}

// ProfileData represents collected profiling data
type ProfileData struct {
	SessionID   string                 `json:"session_id"`
	Type        ProfileType            `json:"type"`
	Timestamp   time.Time              `json:"timestamp"`
	Data        []byte                 `json:"data"` // pprof format
	Metadata    map[string]interface{} `json:"metadata"`
	SampleRate  int                    `json:"sample_rate"`
	SampleCount int64                  `json:"sample_count"`
}

// CallGraphNode represents a node in the call graph
type CallGraphNode struct {
	ID           string                 `json:"id"`
	FunctionName string                 `json:"function_name"`
	FileName     string                 `json:"file_name"`
	LineNumber   int                    `json:"line_number"`
	SelfTime     float64                `json:"self_time"`
	TotalTime    float64                `json:"total_time"`
	Calls        int64                  `json:"calls"`
	Children     []*CallGraphNode       `json:"children,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// FlameGraphFrame represents a frame in the flame graph
type FlameGraphFrame struct {
	Name     string             `json:"name"`
	Value    float64            `json:"value"`
	Children []*FlameGraphFrame `json:"children,omitempty"`
}

// MetricsSnapshot represents system metrics at a point in time
type MetricsSnapshot struct {
	Timestamp      time.Time `json:"timestamp"`
	CPUPercent     float64   `json:"cpu_percent"`
	MemoryUsed     uint64    `json:"memory_used"`
	MemoryTotal    uint64    `json:"memory_total"`
	MemoryPercent  float64   `json:"memory_percent"`
	IOReadBytes    uint64    `json:"io_read_bytes"`
	IOWriteBytes   uint64    `json:"io_write_bytes"`
	IOReadOps      uint64    `json:"io_read_ops"`
	IOWriteOps     uint64    `json:"io_write_ops"`
	GoroutineCount int       `json:"goroutine_count,omitempty"`
	HeapAlloc      uint64    `json:"heap_alloc,omitempty"`
	HeapSys        uint64    `json:"heap_sys,omitempty"`
	GCPauseTotal   uint64    `json:"gc_pause_total,omitempty"`
}

// ProfilingConfig represents configuration for a profiling session
type ProfilingConfig struct {
	ProfileTypes    []ProfileType `json:"profile_types"`
	Duration        time.Duration `json:"duration"`
	SampleRate      int           `json:"sample_rate"`
	CollectMetrics  bool          `json:"collect_metrics"`
	MetricsInterval time.Duration `json:"metrics_interval"`
}

// AgentConfig represents configuration for the profiling agent
type AgentConfig struct {
	ServerURL       string        `json:"server_url"`
	ApplicationID   string        `json:"application_id"`
	ApplicationName string        `json:"application_name"`
	Language        string        `json:"language"`
	Mode            ProfileMode   `json:"mode"`
	AutoProfile     bool          `json:"auto_profile"`
	ProfileInterval time.Duration `json:"profile_interval"`
}
