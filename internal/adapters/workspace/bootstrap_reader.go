package workspace

import (
	"os"
	"path/filepath"
	"strings"
)

// BootstrapReader loads optional workspace bootstrap documents such as AGENTS.md.
//
// It is intentionally thin:
// the service layer decides which files matter and in what order,
// while this adapter only translates "workspace + filename" into file reads.
type BootstrapReader struct {
	workspace string
}

// NewBootstrapReader creates a file-backed bootstrap reader for one workspace.
func NewBootstrapReader(workspace string) *BootstrapReader {
	return &BootstrapReader{
		workspace: strings.TrimSpace(workspace),
	}
}

// Load reads the requested bootstrap files and returns only the files that exist.
func (r *BootstrapReader) Load(names []string) (map[string]string, error) {
	if r == nil || r.workspace == "" || len(names) == 0 {
		return map[string]string{}, nil
	}

	loaded := make(map[string]string, len(names))
	for _, name := range names {
		if strings.TrimSpace(name) == "" {
			continue
		}

		path := filepath.Join(r.workspace, name)
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return loaded, err
		}

		loaded[name] = string(data)
	}
	return loaded, nil
}
