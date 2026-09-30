package httpapi

import (
	"encoding/json"
	"log"
	"net/http"
	"os"

	"armada/internal/cluster"
	"armada/internal/host"
	"armada/internal/term"
	"armada/internal/uart"
	webroot "armada/web"

	"github.com/coder/websocket"
)

type Server struct {
	collector *host.Collector
	board     *cluster.Board
	password  string
	mux       *http.ServeMux
}

func New(collector *host.Collector, board *cluster.Board) *Server {
	s := &Server{
		collector: collector,
		board:     board,
		password:  os.Getenv("ARMADA_PASSWORD"),
		mux:       http.NewServeMux(),
	}
	s.routes()
	return s
}

func (s *Server) Handler() http.Handler {
	if s.password == "" {
		return s.mux
	}
	return s.basicAuth(s.mux)
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/v1/host", s.handleHost)
	s.mux.HandleFunc("GET /api/v1/nodes", s.handleNodes)
	s.mux.HandleFunc("POST /api/v1/nodes/{id}/reboot", s.handleNodeReboot)
	s.mux.HandleFunc("POST /api/v1/nodes/{id}/shutdown", s.handleNodeShutdown)
	s.mux.HandleFunc("POST /api/v1/nodes/{id}/reset", s.handleNodeReset)
	s.mux.HandleFunc("GET /ws/console", s.handleConsoleWS)
	s.mux.Handle("/", serveStatic(webroot.Files))
}

func (s *Server) handleHost(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(s.collector.Snapshot())
}

func (s *Server) handleNodes(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	nodes := []cluster.Node{}
	if s.board != nil {
		nodes = s.board.Nodes()
	}
	_ = json.NewEncoder(w).Encode(struct {
		Nodes []cluster.Node `json:"nodes"`
	}{Nodes: nodes})
}

func (s *Server) handleNodeReboot(w http.ResponseWriter, r *http.Request) {
	s.nodeAction(w, r, func(id int) error { return s.board.Reboot(id) })
}

func (s *Server) handleNodeShutdown(w http.ResponseWriter, r *http.Request) {
	s.nodeAction(w, r, func(id int) error { return s.board.Shutdown(id) })
}

func (s *Server) handleNodeReset(w http.ResponseWriter, r *http.Request) {
	s.nodeAction(w, r, func(id int) error { return s.board.Reset(id) })
}

func (s *Server) nodeAction(w http.ResponseWriter, r *http.Request, fn func(int) error) {
	if s.board == nil {
		http.Error(w, "uart hub is not configured", http.StatusNotFound)
		return
	}
	id, err := uart.ParseNodeID(r.PathValue("id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := fn(id); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(struct {
		OK bool `json:"ok"`
	}{OK: true})
}

func (s *Server) handleConsoleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	sess, err := term.NewShellSession(80, 24)
	if err != nil {
		log.Printf("console: pty: %v", err)
		conn.Close(websocket.StatusInternalError, "pty failed")
		return
	}
	defer sess.Close()

	ctx := r.Context()

	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := sess.Read(buf)
			if n > 0 {
				if werr := conn.Write(ctx, websocket.MessageBinary, buf[:n]); werr != nil {
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()

	for {
		typ, data, err := conn.Read(ctx)
		if err != nil {
			return
		}
		switch typ {
		case websocket.MessageBinary:
			if _, err := sess.Write(data); err != nil {
				return
			}
		case websocket.MessageText:
			var msg struct {
				Type string `json:"type"`
				Cols int    `json:"cols"`
				Rows int    `json:"rows"`
			}
			if json.Unmarshal(data, &msg) == nil && msg.Type == "resize" {
				if msg.Cols > 0 && msg.Rows > 0 {
					_ = sess.Resize(uint16(msg.Cols), uint16(msg.Rows))
				}
			}
		}
	}
}

func (s *Server) basicAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "admin" || pass != s.password {
			w.Header().Set("WWW-Authenticate", `Basic realm="Armada"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
