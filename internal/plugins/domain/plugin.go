package domain

const APIVersion = "harnessforge.plugin/v1"

type Manifest struct {
	APIVersion   string   `json:"api_version"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Command      []string `json:"command"`
	Capabilities []string `json:"capabilities"`
}

type Request struct {
	APIVersion string         `json:"api_version"`
	Operation  string         `json:"operation"`
	Capability string         `json:"capability"`
	Input      map[string]any `json:"input"`
}

type Response struct {
	APIVersion string         `json:"api_version"`
	Output     map[string]any `json:"output"`
	Error      string         `json:"error,omitempty"`
}
