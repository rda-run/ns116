package handler

import (
	"html/template"
	"net/http"

	"ns116/internal/database"
)

type SetupHandler struct {
	db   *database.DB
	tmpl *template.Template
}

func NewSetupHandler(db *database.DB, tmpl *template.Template) *SetupHandler {
	return &SetupHandler{db: db, tmpl: tmpl}
}

func (h *SetupHandler) SetupPage(w http.ResponseWriter, r *http.Request) {
	hasUsers, _ := h.db.HasUsers()
	if hasUsers {
		http.NotFound(w, r)
		return
	}
	h.tmpl.ExecuteTemplate(w, "setup.html", nil)
}

func (h *SetupHandler) SetupSubmit(w http.ResponseWriter, r *http.Request) {
	hasUsers, _ := h.db.HasUsers()
	if hasUsers {
		http.NotFound(w, r)
		return
	}

	_ = r.ParseForm()
	username := r.FormValue("username")
	password := r.FormValue("password")
	confirm := r.FormValue("confirm_password")

	if username == "" {
		h.renderError(w, "Username is required")
		return
	}
	if len(password) < 6 {
		h.renderError(w, "Password must be at least 6 characters")
		return
	}
	if password != confirm {
		h.renderError(w, "Passwords do not match")
		return
	}

	if err := h.db.CreateUser(username, password, "admin"); err != nil {
		h.renderError(w, "Failed to create user: "+err.Error())
		return
	}

	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (h *SetupHandler) renderError(w http.ResponseWriter, msg string) {
	h.tmpl.ExecuteTemplate(w, "setup.html", map[string]string{"Error": msg})
}

func RequireSetupComplete(db *database.DB, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hasUsers, _ := db.HasUsers()
		if !hasUsers {
			http.Redirect(w, r, "/setup", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}
