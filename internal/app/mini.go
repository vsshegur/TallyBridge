package app

import (
	"errors"
	"net/http"
)

func (s *Server) handleAdminRestart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonErr(w, 405, errors.New("POST required"))
		return
	}
	if err := restartDesktopProcess(); err != nil {
		jsonErr(w, 500, err)
		return
	}
	jsonOut(w, map[string]any{"ok": true, "restarting": true})
}
