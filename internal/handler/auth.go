package handler

import (
	"fmt"
	"html/template"
	"net/http"

	"ns116/internal/auth"
	"ns116/internal/database"
	"ns116/internal/model"
	"ns116/internal/util"
)

type AuthHandler struct {
	db         *database.DB
	sessionMgr *auth.SessionManager
	ldap       *auth.LDAPClient
	tmpl       *template.Template
}

func NewAuthHandler(db *database.DB, sm *auth.SessionManager, ldap *auth.LDAPClient, tmpl *template.Template) *AuthHandler {
	return &AuthHandler{db: db, sessionMgr: sm, ldap: ldap, tmpl: tmpl}
}

func (h *AuthHandler) LoginPage(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.sessionMgr.GetUsername(r); ok {
		http.Redirect(w, r, "/zones", http.StatusSeeOther)
		return
	}
	h.tmpl.ExecuteTemplate(w, "login.html", map[string]interface{}{
		"LDAPEnabled": h.ldap != nil,
	})
}

func (h *AuthHandler) LoginSubmit(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	username := r.FormValue("username")
	password := r.FormValue("password")

	var user *model.User
	var authMethod string

	// Try LDAP first (if enabled)
	if h.ldap != nil {
		result, err := h.ldap.Authenticate(username, password)
		if err == nil && result != nil {
			// LDAP auth succeeded — now check group membership
			role, allowed := h.ldap.ResolveRole(result.Groups)
			if !allowed {
				// User authenticated but is not in any mapped group
				h.tmpl.ExecuteTemplate(w, "login.html", map[string]interface{}{
					"Error":       "Access denied: you are not in an authorized group",
					"LDAPEnabled": true,
				})
				return
			}

			// Auto-provision or update user
			_ = h.db.CreateLDAPUser(result.Username, role)
			user, _ = h.db.GetUserByUsername(result.Username)
			authMethod = "ldap"
		}
	}

	// Local fallback — only for admin when LDAP is enabled
	if user == nil {
		u, err := h.db.AuthenticateUser(username, password)
		if err == nil && u != nil {
			if h.ldap != nil && u.Role != "admin" {
				// LDAP is enabled: block non-admin local users
				h.tmpl.ExecuteTemplate(w, "login.html", map[string]interface{}{
					"Error":       "Local login is disabled. Use LDAP credentials.",
					"LDAPEnabled": true,
				})
				return
			}
			user = u
			authMethod = "local"
		}
	}

	// Both failed
	if user == nil {
		h.tmpl.ExecuteTemplate(w, "login.html", map[string]interface{}{
			"Error":       "Invalid credentials",
			"LDAPEnabled": h.ldap != nil,
		})
		return
	}

	h.sessionMgr.CreateSession(w, user.Username)

	_ = h.db.LogAudit(model.AuditEntry{
		Username:  user.Username,
		Action:    "login",
		Detail:    fmt.Sprintf("auth=%s", authMethod),
		IPAddress: util.GetClientIP(r),
	})

	http.Redirect(w, r, "/zones", http.StatusSeeOther)
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	username, _ := h.sessionMgr.GetUsername(r)

	h.sessionMgr.DestroySession(w, r)

	if username != "" {
		_ = h.db.LogAudit(model.AuditEntry{
			Username:  username,
			Action:    "logout",
			IPAddress: util.GetClientIP(r),
		})
	}

	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
