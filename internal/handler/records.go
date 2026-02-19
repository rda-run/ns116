package handler

import (
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"ns116/internal/auth"
	"ns116/internal/database"
	"ns116/internal/model"
	"ns116/internal/service"
	"ns116/internal/util"
)

type RecordHandler struct {
	r53        *service.DNSService
	sessionMgr *auth.SessionManager
	db         *database.DB
	tmpl       *template.Template
}

func NewRecordHandler(r53 *service.DNSService, sm *auth.SessionManager, db *database.DB, tmpl *template.Template) *RecordHandler {
	return &RecordHandler{r53: r53, sessionMgr: sm, db: db, tmpl: tmpl}
}

func (h *RecordHandler) List(w http.ResponseWriter, r *http.Request) {
	zoneID := r.PathValue("zoneID")
	username, csrfToken, _ := h.sessionMgr.GetSessionInfo(r)
	user, _ := h.db.GetUserByUsername(username)

	zone, err := h.r53.GetZone(r.Context(), zoneID)
	if err != nil {
		h.tmpl.ExecuteTemplate(w, "layout", map[string]interface{}{
			"Title":     "Records",
			"Username":  username,
			"CSRFToken": csrfToken,
			"Role":      roleOf(user),
			"ZoneID":    zoneID,
			"ZoneName":  zoneID,
			"Error":     "Failed to load zone: " + err.Error(),
		})
		return
	}

	records, err := h.r53.ListRecords(r.Context(), zoneID)
	if err != nil {
		h.tmpl.ExecuteTemplate(w, "layout", map[string]interface{}{
			"Title":      zone.Name,
			"Username":   username,
			"CSRFToken":  csrfToken,
			"Role":       roleOf(user),
			"ZoneID":     zoneID,
			"ZoneName":   zone.Name,
			"ZoneDomain": zone.Name,
			"Error":      "Failed to load records: " + err.Error(),
		})
		return
	}

	zoneName := zone.Name
	if zone.Label != "" {
		zoneName = zone.Label
	}

	h.tmpl.ExecuteTemplate(w, "layout", map[string]interface{}{
		"Title":      zoneName,
		"Username":   username,
		"CSRFToken":  csrfToken,
		"Role":       roleOf(user),
		"ZoneID":     zoneID,
		"ZoneName":   zoneName,
		"ZoneDomain": zone.Name,
		"Records":    records,
		"Flash":      r.URL.Query().Get("msg"),
	})
}

func qualifyName(name, zoneDomain string) string {
	name = strings.TrimSpace(name)
	if name == "" || name == "@" {
		return zoneDomain
	}
	if strings.HasSuffix(name, ".") {
		return name
	}
	if strings.HasSuffix(name, strings.TrimSuffix(zoneDomain, ".")) {
		return name + "."
	}
	return name + "." + zoneDomain
}

func (h *RecordHandler) Create(w http.ResponseWriter, r *http.Request) {
	zoneID := r.PathValue("zoneID")
	username, _ := h.sessionMgr.GetUsername(r)
	_ = r.ParseForm()

	zone, err := h.r53.GetZone(r.Context(), zoneID)
	zoneDomain := ""
	if err == nil {
		zoneDomain = zone.Name
	}

	req := model.RecordChangeRequest{
		Action: "CREATE",
		Name:   qualifyName(r.FormValue("name"), zoneDomain),
		Type:   r.FormValue("type"),
		TTL:    parseTTL(r.FormValue("ttl")),
		Values: r.Form["value"],
	}

	msg := "Record created successfully"
	if err := h.r53.ChangeRecord(r.Context(), zoneID, req); err != nil {
		msg = "Error: " + err.Error()
	}

	_ = h.db.LogAudit(model.AuditEntry{
		Username:   username,
		Action:     "create_record",
		ZoneID:     zoneID,
		RecordName: req.Name,
		RecordType: req.Type,
		Detail:     fmt.Sprintf("values=%v ttl=%d", req.Values, req.TTL),
		IPAddress:  util.GetClientIP(r),
	})

	http.Redirect(w, r, fmt.Sprintf("/zones/%s/records?msg=%s", zoneID, url.QueryEscape(msg)), http.StatusSeeOther)
}

func (h *RecordHandler) Edit(w http.ResponseWriter, r *http.Request) {
	zoneID := r.PathValue("zoneID")
	username, _ := h.sessionMgr.GetUsername(r)
	_ = r.ParseForm()

	zone, err := h.r53.GetZone(r.Context(), zoneID)
	zoneDomain := ""
	if err == nil {
		zoneDomain = zone.Name
	}

	originalName := r.FormValue("original_name")
	originalType := r.FormValue("original_type")
	newName := qualifyName(r.FormValue("name"), zoneDomain)
	newType := r.FormValue("type")

	// If Name and Type are unchanged, use UPSERT (atomic update)
	if originalName == newName && originalType == newType {
		upsertReq := model.RecordChangeRequest{
			Action: "UPSERT",
			Name:   newName,
			Type:   newType,
			TTL:    parseTTL(r.FormValue("ttl")),
			Values: r.Form["value"],
		}

		msg := "Record updated successfully"
		if err := h.r53.ChangeRecord(r.Context(), zoneID, upsertReq); err != nil {
			msg = "Error updating record: " + err.Error()
		}

		_ = h.db.LogAudit(model.AuditEntry{
			Username:   username,
			Action:     "edit_record",
			ZoneID:     zoneID,
			RecordName: upsertReq.Name,
			RecordType: upsertReq.Type,
			Detail:     fmt.Sprintf("upsert ttl=%d values=%v", upsertReq.TTL, upsertReq.Values),
			IPAddress:  util.GetClientIP(r),
		})

		http.Redirect(w, r, fmt.Sprintf("/zones/%s/records?msg=%s", zoneID, url.QueryEscape(msg)), http.StatusSeeOther)
		return
	}

	// If Name or Type changed, we must DELETE old and CREATE new (non-atomic 2-step process)
	deleteReq := model.RecordChangeRequest{
		Action: "DELETE",
		Name:   originalName,
		Type:   originalType,
		TTL:    parseTTL(r.FormValue("original_ttl")),
		Values: r.Form["original_value"],
	}

	if err := h.r53.ChangeRecord(r.Context(), zoneID, deleteReq); err != nil {
		msg := "Error deleting old record: " + err.Error()
		http.Redirect(w, r, fmt.Sprintf("/zones/%s/records?msg=%s", zoneID, url.QueryEscape(msg)), http.StatusSeeOther)
		return
	}

	createReq := model.RecordChangeRequest{
		Action: "CREATE",
		Name:   newName,
		Type:   newType,
		TTL:    parseTTL(r.FormValue("ttl")),
		Values: r.Form["value"],
	}

	msg := "Record updated successfully"
	if err := h.r53.ChangeRecord(r.Context(), zoneID, createReq); err != nil {
		msg = "Error creating new record: " + err.Error()
	}

	_ = h.db.LogAudit(model.AuditEntry{
		Username:   username,
		Action:     "edit_record",
		ZoneID:     zoneID,
		RecordName: createReq.Name,
		RecordType: createReq.Type,
		Detail:     fmt.Sprintf("rename from %s used 2-step update", deleteReq.Name),
		IPAddress:  util.GetClientIP(r),
	})

	http.Redirect(w, r, fmt.Sprintf("/zones/%s/records?msg=%s", zoneID, url.QueryEscape(msg)), http.StatusSeeOther)
}

func (h *RecordHandler) Delete(w http.ResponseWriter, r *http.Request) {
	zoneID := r.PathValue("zoneID")
	username, _ := h.sessionMgr.GetUsername(r)
	_ = r.ParseForm()

	req := model.RecordChangeRequest{
		Action: "DELETE",
		Name:   r.FormValue("name"),
		Type:   r.FormValue("type"),
		TTL:    parseTTL(r.FormValue("ttl")),
		Values: r.Form["value"],
	}

	msg := "Record deleted successfully"
	if err := h.r53.ChangeRecord(r.Context(), zoneID, req); err != nil {
		msg = "Error: " + err.Error()
	}

	_ = h.db.LogAudit(model.AuditEntry{
		Username:   username,
		Action:     "delete_record",
		ZoneID:     zoneID,
		RecordName: req.Name,
		RecordType: req.Type,
		IPAddress:  util.GetClientIP(r),
	})

	http.Redirect(w, r, fmt.Sprintf("/zones/%s/records?msg=%s", zoneID, url.QueryEscape(msg)), http.StatusSeeOther)
}

func parseTTL(s string) int64 {
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 300
	}
	return v
}

func (h *RecordHandler) RefreshRecords(w http.ResponseWriter, r *http.Request) {
	zoneID := r.PathValue("zoneID")
	h.db.InvalidateRecordCache(zoneID)
	http.Redirect(w, r, fmt.Sprintf("/zones/%s/records", zoneID), http.StatusSeeOther)
}
