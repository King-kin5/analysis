# Universal Profiler

A lightweight performance analysis platform that helps you find bottlenecks in your applications. Works with any programming language that can generate pprof format data.

## What Does It Do?

Universal Profiler helps you answer questions like:
- Why is my application slow?
- Which functions are using the most CPU?
- Where are my memory leaks?
- How has performance changed over time?

## Key Features

- **Profile Any Language** - Uses standard pprof format (Go, Python, Node.js, Rust, C++, Java)
- **Visual Analysis** - Interactive flame graphs and call graphs
- **Real-time Metrics** - Monitor CPU, memory, and I/O as your app runs
- **Compare Sessions** - Spot performance regressions between versions
- **Easy Integration** - Embed in your app or run as a separate service
- **No Database Required** - File-based storage, single binary deployment

## Quick Start

### 1. Install

```bash
go install github.com/King-kin5/analysis/cmd/server@latest
```

### 2. Start the Server

```bash
universal-profiler server --port 8080
```

### 3. Profile Your Application

**Option A: Embedded Mode (Go applications)**
```go
import "github.com/King-kin5/analysis/pkg/agent"

func main() {
    agent := agent.New(agent.Config{
        ServerURL: "http://localhost:8080",
        AppName:   "my-app",
    })
    agent.Start()
    defer agent.Stop()
    
    // Your application code
}
```

**Option B: Upload Existing pprof Files**
```bash
curl -X POST http://localhost:8080/api/v1/profiles \
  -F "file=@cpu.pprof" \
  -F "session_id=my-session"
```

### 4. View Results

Open `http://localhost:8080` in your browser to see flame graphs and metrics.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                        Web Dashboard                         │
│              (HTMX + Tailwind CSS + D3.js)                  │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐     │
│  │ Flame Graphs │  │ Call Graphs  │  │   Metrics    │     │
│  └──────────────┘  └──────────────┘  └──────────────┘     │
└────────────────────────────┬────────────────────────────────┘
                             │
                    ┌────────▼────────┐
                    │   HTTP Server   │
                    │  (Gorilla Mux)  │
                    └────────┬────────┘
                             │
        ┌────────────────────┼────────────────────┐
        │                    │                    │
   ┌────▼────┐         ┌─────▼─────┐      ┌──────▼──────┐
   │Collector│         │ Analyzer  │      │   Storage   │
   │         │────────▶│           │◀─────│             │
   │ Receive │         │Parse pprof│      │File-based DB│
   │ Profiles│         │Generate   │      │             │
   └────▲────┘         │Flame Graph│      └─────────────┘
        │              └───────────┘
        │
┌───────┴────────────────────────────────────────────────────┐
│                    Data Collection                          │
├─────────────────────────────┬───────────────────────────────┤
│                             │                               │
│  ┌────────────────┐    ┌────▼─────────┐    ┌────────────┐ │
│  │ Embedded Agent │    │Sidecar Agent │    │Manual Upload│ │
│  │  (In Process)  │    │  (Separate)  │    │  (CLI/API)  │ │
│  └────────────────┘    └──────────────┘    └─────────────┘ │
└────────────────────────────────────────────────────────────┘
```
## Data Flow

```
Application → Agent → Collector → Storage
                                      ↓
                            Analyzer ← Storage
                                      ↓
                            Web UI ← Analyzer
```

**Step by Step:**
1. Your application runs with profiling enabled
2. Agent collects CPU/memory/IO data
3. Agent sends data to Collector via HTTP
4. Collector saves raw data to Storage
5. User opens Web Dashboard
6. Analyzer reads and processes the profile
7. Web UI displays flame graphs and metrics

## Profile Types Supported

| Type | Description | Use Case |
|------|-------------|----------|
| **CPU** | Function execution time | Find slow functions |
| **Memory** | Heap allocations | Detect memory leaks |
| **Heap** | Memory usage snapshot | Analyze memory layout |
| **Block** | Goroutine blocking | Find contention |
| **Mutex** | Lock contention | Optimize synchronization |
| **IO** | File/network operations | Identify IO bottlenecks |

## API Reference

### Create Session
```http
POST /api/v1/sessions
Content-Type: application/json

{
  "id": "session-123",
  "application_id": "my-app",
  "name": "Production Load Test",
  "language": "go",
  "profile_type": "cpu",
  "duration": 30000000000
}
```

### Upload Profile Data
```http
POST /api/v1/profiles
Content-Type: application/json

{
  "session_id": "session-123",
  "type": "cpu",
  "data": "<base64-encoded-pprof>",
  "sample_rate": 100
}
```

### Get Session
```http
GET /api/v1/sessions/{id}
```

### List Sessions
```http
GET /api/v1/sessions?application_id=my-app
```

## Configuration

### Server
```bash
universal-profiler server \
  --port 8080 \
  --data-dir ./profiler-data \
  --log-level info
```

### Agent
```go
config := agent.Config{
    ServerURL:       "http://localhost:8080",
    ApplicationID:   "my-app-id",
    ApplicationName: "My Application",
    Language:        "go",
    Mode:            types.ProfileModeEmbedded,
    AutoProfile:     true,
    ProfileInterval: 5 * time.Minute,
}
```

## Project Structure

```
universal-profiler/
├── cmd/
│   ├── server/           # Main server entry point
│   └── agent/            # Standalone agent binary
├── pkg/
│   ├── types/            # Core data types
│   ├── storage/          # File-based storage
│   ├── collector/        # HTTP API server
│   ├── analyzer/         # Profile analysis [TODO]
│   ├── metrics/          # System metrics [TODO]
│   ├── agent/            # Integration SDK [TODO]
│   └── web/              # Web dashboard [TODO]
├── web/
│   ├── templates/        # HTML templates
│   ├── static/           # CSS, JS, images
│   └── components/       # UI components
├── internal/
│   └── config/           # Configuration management
├── examples/             # Integration examples
└── docs/                 # Documentation

```

## Development Roadmap

- [x] Core type system
- [x] File-based storage
- [x] HTTP collector API
- [ ] pprof parser and analyzer
- [ ] Flame graph generator
- [ ] Web dashboard UI
- [ ] Agent SDK
- [ ] Metrics collector
- [ ] Session comparison
- [ ] Docker support
- [ ] Kubernetes deployment

## Technology Stack

- **Backend**: Go 1.21+
- **Web Framework**: Gorilla Mux
- **Frontend**: HTMX, Tailwind CSS
- **Visualization**: D3.js, Chart.js
- **Storage**: File-based (JSON + Binary)
- **Logging**: Uber Zap
- **Profile Format**: Google pprof

## Language Support

Universal Profiler works with any language that can generate pprof format:

- **Go**: Native support via `runtime/pprof`
- **Python**: Use `py-spy` or `pypprof`
- **Node.js**: Use `pprof` npm package
- **Rust**: Use `pprof-rs`
- **C/C++**: Use `gperftools`
- **Java**: Use `async-profiler` with pprof output

## Contributing

Contributions welcome! Areas that need help:
- Profile analyzer implementation
- Web dashboard development
- Language-specific agent examples
- Documentation improvements

---
**Status**: 🚧 Active Development | **Version**: 0.1.0-alpha