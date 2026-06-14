package ollama

import "time"

type Clock func() time.Time

type Source string

const SourceHTTPAPI Source = "ollama-http-api"

type SnapshotStatus string

const (
	StatusConfirmed   SnapshotStatus = "confirmed"
	StatusUnreachable SnapshotStatus = "unreachable"
)

type SnapshotMeta struct {
	Source          Source
	Endpoint        string
	ObservedAt      time.Time
	LastConfirmedAt time.Time
	Status          SnapshotStatus
	Reachable       bool
	Cached          bool
	Error           string
}

type ModelDetails struct {
	ParentModel       string   `json:"parent_model"`
	Format            string   `json:"format"`
	Family            string   `json:"family"`
	Families          []string `json:"families"`
	ParameterSize     string   `json:"parameter_size"`
	QuantizationLevel string   `json:"quantization_level"`
}

type VersionSnapshot struct {
	Meta    SnapshotMeta
	Version string `json:"version"`
}

type RunningModel struct {
	Name          string       `json:"name"`
	Model         string       `json:"model"`
	Size          int64        `json:"size"`
	Digest        string       `json:"digest"`
	Details       ModelDetails `json:"details"`
	ExpiresAt     time.Time
	SizeVRAM      int64 `json:"size_vram"`
	ContextLength int   `json:"context_length"`
}

type RunningModelsSnapshot struct {
	Meta   SnapshotMeta
	Models []RunningModel
}

type CatalogModel struct {
	Name        string `json:"name"`
	Model       string `json:"model"`
	RemoteModel string `json:"remote_model"`
	RemoteHost  string `json:"remote_host"`
	ModifiedAt  time.Time
	Size        int64        `json:"size"`
	Digest      string       `json:"digest"`
	Details     ModelDetails `json:"details"`
}

type CatalogSnapshot struct {
	Meta   SnapshotMeta
	Models []CatalogModel
}

type ShowSnapshot struct {
	Meta         SnapshotMeta
	Model        string
	Parameters   string   `json:"parameters"`
	License      string   `json:"license"`
	Template     string   `json:"template"`
	Capabilities []string `json:"capabilities"`
	ModifiedAt   time.Time
	Details      ModelDetails   `json:"details"`
	ModelInfo    map[string]any `json:"model_info"`
}

type PollRequest struct {
	ShowModels []string
}

type PollSnapshot struct {
	Meta    SnapshotMeta
	Version VersionSnapshot
	Running RunningModelsSnapshot
	Catalog CatalogSnapshot
	Shows   map[string]ShowSnapshot
}
