package httpapi

import (
	"embed"
	"net/http"
)

// Keep documentation in the binary so agents can discover it in packaged builds.
//
//go:embed agent/SKILL.md agent/references/api.md
var agentDocuments embed.FS

func (s *Server) agentSkill(w http.ResponseWriter, r *http.Request) {
	serveAgentDocument(w, r, "agent/SKILL.md")
}

func (s *Server) apiDocs(w http.ResponseWriter, r *http.Request) {
	serveAgentDocument(w, r, "agent/references/api.md")
}

func serveAgentDocument(w http.ResponseWriter, r *http.Request, path string) {
	content, err := agentDocuments.ReadFile(path)
	if err != nil {
		http.Error(w, "documentation unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if r.Method != http.MethodHead {
		_, _ = w.Write(content)
	}
}
