package httpapi

import "net/http"

func (s *Server) retention(w http.ResponseWriter, r *http.Request) {
	request := struct {
		Days        int  `json:"days"`
		KeepPerTask int  `json:"keepPerTask"`
		DryRun      bool `json:"dryRun"`
	}{Days: 30, KeepPerTask: 100, DryRun: true}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, err)
		return
	}
	result, err := s.engine.Retain(r.Context(), request.Days, request.KeepPerTask, request.DryRun)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
