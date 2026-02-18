package handler

import (
	"html/template"
	"net/http"

	"ns116/internal/auth"
	"ns116/internal/database"
	"ns116/internal/model"
	"ns116/internal/service"
)

type ZoneHandler struct {
	r53        *service.DNSService
	sessionMgr *auth.SessionManager
	db         *database.DB
	tmpl       *template.Template
}

func NewZoneHandler(r53 *service.DNSService, sm *auth.SessionManager, db *database.DB, tmpl *template.Template) *ZoneHandler {
	return &ZoneHandler{r53: r53, sessionMgr: sm, db: db, tmpl: tmpl}
}

func (h *ZoneHandler) List(w http.ResponseWriter, r *http.Request) {
	username, csrfToken, _ := h.sessionMgr.GetSessionInfo(r)
	user, _ := h.db.GetUserByUsername(username)

	zones, err := h.r53.ListZones(r.Context())
	if err != nil {
		h.tmpl.ExecuteTemplate(w, "layout", map[string]interface{}{
			"Title":     "Hosted Zones",
			"Username":  username,
			"CSRFToken": csrfToken,
			"Role":      roleOf(user),
			"Error":     "Failed to load zones: " + err.Error(),
		})
		return
	}

	h.tmpl.ExecuteTemplate(w, "layout", map[string]interface{}{
		"Title":     "Hosted Zones",
		"Username":  username,
		"CSRFToken": csrfToken,
		"Role":      roleOf(user),
		"Zones":     zones,
	})
}

func roleOf(u *model.User) string {
	if u != nil {
		return u.Role
	}
	return ""
}

func (h *ZoneHandler) RefreshZones(w http.ResponseWriter, r *http.Request) {
	h.db.InvalidateAllCache()
	http.Redirect(w, r, "/zones", http.StatusSeeOther)
}
