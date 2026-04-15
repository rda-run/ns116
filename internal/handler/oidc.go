package handler

import (
	"fmt"
	"net/http"
	"strings"

	"ns116/internal/auth"
	"ns116/internal/auth/oidc"
	"ns116/internal/database"
	"ns116/internal/model"
	"ns116/internal/util"
)

type OIDCHandler struct {
	db         *database.DB
	sessionMgr *auth.SessionManager
	oidcSvc    *oidc.Service
}

func NewOIDCHandler(db *database.DB, sm *auth.SessionManager, oidcSvc *oidc.Service) *OIDCHandler {
	return &OIDCHandler{db: db, sessionMgr: sm, oidcSvc: oidcSvc}
}

func (h *OIDCHandler) Start(w http.ResponseWriter, r *http.Request) {
	provider := r.PathValue("provider")
	authURL, err := h.oidcSvc.StartAuth(w, r, provider)
	if err != nil {
		oidc.RedirectToLoginWithError(w, r, "OIDC login failed to start")
		return
	}
	http.Redirect(w, r, authURL, http.StatusSeeOther)
}

func (h *OIDCHandler) Callback(w http.ResponseWriter, r *http.Request) {
	defer h.oidcSvc.ClearRequestCookie(w)

	ident, err := h.oidcSvc.HandleCallback(r.Context(), r)
	if err != nil {
		oidc.RedirectToLoginWithError(w, r, "OIDC login failed")
		return
	}

	pcfg, ok := h.oidcSvc.ProviderConfig(ident.Provider)
	if !ok {
		oidc.RedirectToLoginWithError(w, r, "OIDC login failed")
		return
	}

	// Resolve role from groups claim using provider mapping.
	role := ""
	groups, err := oidcClaimGroups(ident.Claims, pcfg.GroupsClaim)
	if err == nil {
		if inAny(groups, pcfg.GroupMapping["admin"]) {
			role = "admin"
		} else if inAny(groups, pcfg.GroupMapping["editor"]) {
			role = "editor"
		}
	}
	if role == "" {
		oidc.RedirectToLoginWithError(w, r, "Access denied")
		return
	}

	user, err := h.db.GetUserByUsername(ident.Username)
	if err != nil {
		oidc.RedirectToLoginWithError(w, r, "Login error")
		return
	}
	if user != nil && !user.Active {
		oidc.RedirectToLoginWithError(w, r, "Access denied")
		return
	}

	if err := h.db.CreateOIDCUser(ident.Username, role); err != nil {
		oidc.RedirectToLoginWithError(w, r, "Login error")
		return
	}
	user, err = h.db.GetUserByUsername(ident.Username)
	if err != nil || user == nil || !user.Active {
		oidc.RedirectToLoginWithError(w, r, "Login error")
		return
	}

	h.sessionMgr.CreateSession(w, user.Username)

	_ = h.db.LogAudit(model.AuditEntry{
		Username:  user.Username,
		Action:    "login",
		Detail:    fmt.Sprintf("auth=oidc provider=%s", ident.Provider),
		IPAddress: util.GetClientIP(r),
	})

	http.Redirect(w, r, "/zones", http.StatusSeeOther)
}

func oidcClaimGroups(claims map[string]any, key string) ([]string, error) {
	// keep handler logic simple; rely on oidc package helpers via a minimal shim
	return oidcClaimStrings(claims, key)
}

func oidcClaimStrings(claims map[string]any, key string) ([]string, error) {
	// duplicated wrapper to avoid exporting helpers prematurely; safe and small
	// NOTE: key defaults to "groups" in config.
	return oidcClaimStringsInner(claims, key)
}

func oidcClaimStringsInner(claims map[string]any, key string) ([]string, error) {
	// Inline minimal extraction to keep handler independent of unexported oidc helpers.
	cur := any(claims)
	for _, part := range strings.Split(key, ".") {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("claim %q not found", key)
		}
		cur, ok = m[part]
		if !ok {
			return nil, fmt.Errorf("claim %q not found", key)
		}
	}
	switch v := cur.(type) {
	case []string:
		return v, nil
	case []any:
		out := make([]string, 0, len(v))
		for _, it := range v {
			s, ok := it.(string)
			if ok && s != "" {
				out = append(out, s)
			}
		}
		return out, nil
	default:
		return nil, fmt.Errorf("claim %q is not a string array", key)
	}
}

func inAny(have []string, want []string) bool {
	if len(have) == 0 || len(want) == 0 {
		return false
	}
	set := map[string]struct{}{}
	for _, g := range have {
		set[g] = struct{}{}
	}
	for _, w := range want {
		if _, ok := set[w]; ok {
			return true
		}
	}
	return false
}
