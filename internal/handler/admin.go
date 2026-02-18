package handler

import (
	"fmt"
	"html/template"
	"net/http"
	"strconv"

	"ns116/internal/auth"
	"ns116/internal/database"
	"ns116/internal/model"
)

type AdminHandler struct {
	db         *database.DB
	sessionMgr *auth.SessionManager
	tmpl       *template.Template
}

func NewAdminHandler(db *database.DB, sm *auth.SessionManager, tmpl *template.Template) *AdminHandler {
	return &AdminHandler{db: db, sessionMgr: sm, tmpl: tmpl}
}

func (h *AdminHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	username, csrfToken, _ := h.sessionMgr.GetSessionInfo(r)
	user, _ := h.db.GetUserByUsername(username)

	users, err := h.db.ListUsers()
	if err != nil {
		h.tmpl.ExecuteTemplate(w, "layout", map[string]interface{}{
			"Title":     "Users",
			"Username":  username,
			"CSRFToken": csrfToken,
			"Role":      roleOf(user),
			"Error":     "Failed to load users: " + err.Error(),
		})
		return
	}

	h.tmpl.ExecuteTemplate(w, "layout", map[string]interface{}{
		"Title":     "Users",
		"Username":  username,
		"CSRFToken": csrfToken,
		"Role":      roleOf(user),
		"Users":     users,
		"Flash":     r.URL.Query().Get("msg"),
	})
}

func (h *AdminHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	username, _ := h.sessionMgr.GetUsername(r)
	newUsername := r.FormValue("username")
	password := r.FormValue("password")
	role := r.FormValue("role")

	if role != "admin" && role != "editor" {
		role = "editor"
	}

	msg := fmt.Sprintf("User '%s' created successfully", newUsername)
	if err := h.db.CreateUser(newUsername, password, role); err != nil {
		msg = "Error: " + err.Error()
	} else {
		_ = h.db.LogAudit(model.AuditEntry{
			Username:  username,
			Action:    "create_user",
			Detail:    fmt.Sprintf("created user=%s role=%s", newUsername, role),
			IPAddress: r.RemoteAddr,
		})
	}

	http.Redirect(w, r, "/admin/users?msg="+msg, http.StatusSeeOther)
}

func (h *AdminHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	username, _ := h.sessionMgr.GetUsername(r)
	targetUser := r.FormValue("username")

	if targetUser == username {
		http.Redirect(w, r, "/admin/users?msg=Cannot+delete+yourself", http.StatusSeeOther)
		return
	}

	msg := fmt.Sprintf("User '%s' deleted", targetUser)
	if err := h.db.DeleteUser(targetUser); err != nil {
		msg = "Error: " + err.Error()
	} else {
		_ = h.db.LogAudit(model.AuditEntry{
			Username:  username,
			Action:    "delete_user",
			Detail:    fmt.Sprintf("deleted user=%s", targetUser),
			IPAddress: r.RemoteAddr,
		})
	}

	http.Redirect(w, r, "/admin/users?msg="+msg, http.StatusSeeOther)
}

func (h *AdminHandler) AuditLog(w http.ResponseWriter, r *http.Request) {
	username, csrfToken, _ := h.sessionMgr.GetSessionInfo(r)
	user, _ := h.db.GetUserByUsername(username)

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	limit := 50
	offset := (page - 1) * limit

	entries, total, err := h.db.ListAuditLog(limit, offset)
	if err != nil {
		h.tmpl.ExecuteTemplate(w, "layout", map[string]interface{}{
			"Title":     "Audit Log",
			"Username":  username,
			"CSRFToken": csrfToken,
			"Role":      roleOf(user),
			"Error":     "Failed to load audit log: " + err.Error(),
		})
		return
	}

	totalPages := (total + limit - 1) / limit

	h.tmpl.ExecuteTemplate(w, "layout", map[string]interface{}{
		"Title":      "Audit Log",
		"Username":   username,
		"CSRFToken":  csrfToken,
		"Role":       roleOf(user),
		"Entries":    entries,
		"Page":       page,
		"TotalPages": totalPages,
		"Total":      total,
	})
}
