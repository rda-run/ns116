package server

import (
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"

	"ns116/internal/auth"
	"ns116/internal/config"
	"ns116/internal/database"
	"ns116/internal/handler"
	"ns116/internal/service"
	"ns116/web"
	"time"
)

func mustParseTemplates(fsys fs.FS, funcMap template.FuncMap, files ...string) *template.Template {
	tmpl := template.New("").Funcs(funcMap)
	tmpl, err := tmpl.ParseFS(fsys, files...)
	if err != nil {
		log.Fatalf("Failed to parse templates %v: %v", files, err)
	}
	return tmpl
}

func Start(cfg *config.Config, version string) error {
	db, err := database.Open(cfg.Database.DSN, web.MigrationsFS())
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	sessionMgr, err := auth.NewSessionManager(db)
	if err != nil {
		return fmt.Errorf("failed to init session manager: %w", err)
	}

	_ = db.PurgeExpiredSessions()

	r53, err := service.NewDNSService(cfg, db)
	if err != nil {
		return fmt.Errorf("failed to init DNS service: %w", err)
	}

	tmplFS := web.TemplateFS()

	funcMap := template.FuncMap{
		"add":        func(a, b int) int { return a + b },
		"subtract":   func(a, b int) int { return a - b },
		"version":    func() string { return version },
		"formatDate": func(t time.Time) string { return t.Format("2006-01-02 15:04:05") },
	}

	loginTmpl := mustParseTemplates(tmplFS, funcMap, "templates/login.html")
	setupTmpl := mustParseTemplates(tmplFS, funcMap, "templates/setup.html")
	zonesTmpl := mustParseTemplates(tmplFS, funcMap, "templates/layout.html", "templates/zones.html")
	recordsTmpl := mustParseTemplates(tmplFS, funcMap, "templates/layout.html", "templates/records.html")
	adminUsersTmpl := mustParseTemplates(tmplFS, funcMap, "templates/layout.html", "templates/admin_users.html")
	adminAuditTmpl := mustParseTemplates(tmplFS, funcMap, "templates/layout.html", "templates/admin_audit.html")

	// Initialize LDAP client (nil if disabled)
	var ldapClient *auth.LDAPClient
	if cfg.LDAP.Enabled {
		ldapClient = auth.NewLDAPClient(cfg.LDAP)
		log.Println("LDAP authentication enabled")
		log.Printf("LDAP server: %s", cfg.LDAP.URL)
		log.Printf("LDAP groups mapped: %d role(s)", len(cfg.LDAP.GroupMapping))
	}

	setupH := handler.NewSetupHandler(db, setupTmpl)
	authH := handler.NewAuthHandler(db, sessionMgr, ldapClient, loginTmpl)
	zoneH := handler.NewZoneHandler(r53, sessionMgr, db, zonesTmpl)
	recH := handler.NewRecordHandler(r53, sessionMgr, db, recordsTmpl)
	adminH := handler.NewAdminHandler(db, sessionMgr, adminUsersTmpl)
	adminAuditH := handler.NewAdminHandler(db, sessionMgr, adminAuditTmpl)

	mux := http.NewServeMux()

	mux.HandleFunc("GET /setup", setupH.SetupPage)
	mux.HandleFunc("POST /setup", setupH.SetupSubmit)

	mux.Handle("GET /static/", web.StaticHandler())

	appMux := http.NewServeMux()

	appMux.HandleFunc("GET /login", authH.LoginPage)
	appMux.HandleFunc("POST /login", authH.LoginSubmit)
	appMux.HandleFunc("POST /logout", authH.Logout)

	appMux.HandleFunc("GET /zones", sessionMgr.RequireAuth(zoneH.List))
	appMux.HandleFunc("POST /zones/refresh", sessionMgr.RequireAuth(sessionMgr.ValidateCSRF(zoneH.RefreshZones)))
	appMux.HandleFunc("GET /zones/{zoneID}/records", sessionMgr.RequireAuth(recH.List))
	appMux.HandleFunc("POST /zones/{zoneID}/records/refresh", sessionMgr.RequireAuth(sessionMgr.ValidateCSRF(recH.RefreshRecords)))
	appMux.HandleFunc("POST /zones/{zoneID}/records/create", sessionMgr.RequireAuth(sessionMgr.ValidateCSRF(recH.Create)))
	appMux.HandleFunc("POST /zones/{zoneID}/records/edit", sessionMgr.RequireAuth(sessionMgr.ValidateCSRF(recH.Edit)))
	appMux.HandleFunc("POST /zones/{zoneID}/records/delete", sessionMgr.RequireAuth(sessionMgr.ValidateCSRF(recH.Delete)))

	appMux.HandleFunc("GET /admin/users", sessionMgr.RequireAdmin(adminH.ListUsers))
	appMux.HandleFunc("POST /admin/users/create", sessionMgr.RequireAdmin(sessionMgr.ValidateCSRF(adminH.CreateUser)))
	appMux.HandleFunc("POST /admin/users/delete", sessionMgr.RequireAdmin(sessionMgr.ValidateCSRF(adminH.DeleteUser)))
	appMux.HandleFunc("GET /admin/audit", sessionMgr.RequireAdmin(adminAuditH.AuditLog))

	appMux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/zones", http.StatusSeeOther)
	})

	mux.Handle("/", handler.RequireSetupComplete(db, appMux))

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Printf("NS116 server starting on %s", addr)
	return http.ListenAndServe(addr, mux)
}
