package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"

	"armada/internal/settings"
)

func (s *Server) handleSettingsGet(w http.ResponseWriter, r *http.Request) {
	if s.settings == nil {
		http.Error(w, "settings unavailable", http.StatusServiceUnavailable)
		return
	}
	v, err := s.settings.Get()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(v)
}

func (s *Server) handleSettingsPut(w http.ResponseWriter, r *http.Request) {
	if s.settings == nil {
		http.Error(w, "settings unavailable", http.StatusServiceUnavailable)
		return
	}
	var body settings.Values
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if err := s.settings.Put(body); err != nil {
		msg := err.Error()
		if strings.Contains(msg, "required") || strings.Contains(msg, "too long") {
			http.Error(w, msg, http.StatusBadRequest)
			return
		}
		http.Error(w, msg, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
