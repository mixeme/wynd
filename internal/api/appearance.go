package api

import "net/http"

// Внешний вид, общий для всех устройств человека (план 46, A16): форма
// аватаров.

type appearanceBody struct {
	AvatarShape string `json:"avatar_shape"`
}

func (s *Server) handleGetAppearance(w http.ResponseWriter, r *http.Request) {
	sess, ok := requireSession(w, r)
	if !ok {
		return
	}
	shape, err := s.Auth.AvatarShape(r.Context(), sess.AccountID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, appearanceBody{AvatarShape: shape})
}

func (s *Server) handleSetAppearance(w http.ResponseWriter, r *http.Request) {
	sess, ok := requireSession(w, r)
	if !ok {
		return
	}
	body, ok := bindJSON[appearanceBody](w, r)
	if !ok {
		return
	}
	if err := s.Auth.SetAvatarShape(r.Context(), sess.AccountID, body.AvatarShape); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, body)
}
