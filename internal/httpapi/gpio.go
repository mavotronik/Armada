package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"

	"armada/internal/gpio"
)

func (s *Server) handleGPIOList(w http.ResponseWriter, r *http.Request) {
	if s.gpio == nil {
		http.Error(w, "gpio unavailable", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(s.gpio.Snapshot())
}

func (s *Server) handleGPIOPin(w http.ResponseWriter, r *http.Request) {
	if s.gpio == nil {
		http.Error(w, "gpio unavailable", http.StatusServiceUnavailable)
		return
	}
	id := r.PathValue("id")
	if _, ok := gpio.PinByID(id); !ok {
		http.Error(w, "unknown pin", http.StatusNotFound)
		return
	}

	switch r.Method {
	case http.MethodPut:
		var cfg gpio.PinConfig
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		if err := s.gpio.SetConfig(id, cfg); err != nil {
			writeGPIOError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleGPIODefault(w http.ResponseWriter, r *http.Request) {
	if s.gpio == nil {
		http.Error(w, "gpio unavailable", http.StatusServiceUnavailable)
		return
	}
	id := r.PathValue("id")
	if err := s.gpio.SaveDefault(id); err != nil {
		writeGPIOError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleGPIOReset(w http.ResponseWriter, r *http.Request) {
	if s.gpio == nil {
		http.Error(w, "gpio unavailable", http.StatusServiceUnavailable)
		return
	}
	id := r.PathValue("id")
	if err := s.gpio.ResetToDefault(id); err != nil {
		writeGPIOError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeGPIOError(w http.ResponseWriter, err error) {
	msg := err.Error()
	if strings.HasPrefix(msg, "unknown pin") {
		http.Error(w, msg, http.StatusNotFound)
		return
	}
	if msg == "no default saved" || msg == "pin not configured" ||
		strings.Contains(msg, "invalid") || strings.Contains(msg, "duty must") {
		http.Error(w, msg, http.StatusBadRequest)
		return
	}
	http.Error(w, msg, http.StatusInternalServerError)
}
