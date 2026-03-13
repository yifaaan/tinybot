package app

import "testing"

func TestBuildCoreToolRegistry_DoesNotRegisterMessageTool(t *testing.T) {
	cfg := DefaultConfig()
	reg := buildCoreToolRegistry(t.TempDir(), cfg)

	if reg.Has("message") {
		t.Fatal("buildCoreToolRegistry() should not register message tool")
	}
}

func TestBuildCoreToolRegistry_DefinitionsDoNotExposeMessageTool(t *testing.T) {
	cfg := DefaultConfig()
	reg := buildCoreToolRegistry(t.TempDir(), cfg)

	definitions := reg.GetDefinitions()
	for _, definition := range definitions {
		function, ok := definition["function"].(map[string]any)
		if !ok {
			continue
		}
		name, _ := function["name"].(string)
		if name == "message" {
			t.Fatal("buildCoreToolRegistry() should not expose message tool definitions")
		}
	}
}
