package server

import (
	"fmt"
	"log"
	"net/http"
	
	"github.com/Cobliteam/workflow-toolkit/pkg/types"
)

// Server servidor HTTP
type Server struct {
	port      int
	backlog   *types.Backlog
	deepDives []types.DeepDive
	tasks     []types.TaskBreakdown
}

// NewServer cria servidor
func NewServer(port int, backlog *types.Backlog, deepDives []types.DeepDive, tasks []types.TaskBreakdown) *Server {
	return &Server{
		port:      port,
		backlog:   backlog,
		deepDives: deepDives,
		tasks:     tasks,
	}
}

// Start inicia servidor
func (s *Server) Start() error {
	http.HandleFunc("/", serveIndex)
	
	addr := fmt.Sprintf(":%d", s.port)
	log.Printf("Servidor rodando em http://localhost%s\n", addr)
	return http.ListenAndServe(addr, nil)
}

func serveIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte("<html><body><h1>Workflow Platform Server</h1></body></html>"))
}
