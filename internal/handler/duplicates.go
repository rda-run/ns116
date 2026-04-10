package handler

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"sort"
	"strings"

	"ns116/internal/auth"
	"ns116/internal/database"
	"ns116/internal/model"
	"ns116/internal/service"
	"ns116/internal/util"
)

type DuplicateHandler struct {
	r53        *service.DNSService
	sessionMgr *auth.SessionManager
	db         *database.DB
	tmpl       *template.Template
}

func NewDuplicateHandler(r53 *service.DNSService, sm *auth.SessionManager, db *database.DB, tmpl *template.Template) *DuplicateHandler {
	return &DuplicateHandler{r53: r53, sessionMgr: sm, db: db, tmpl: tmpl}
}

func (h *DuplicateHandler) List(w http.ResponseWriter, r *http.Request) {
	username, csrfToken, _ := h.sessionMgr.GetSessionInfo(r)
	user, _ := h.db.GetUserByUsername(username)

	h.tmpl.ExecuteTemplate(w, "layout", map[string]interface{}{
		"Title":     "Duplicate Records",
		"Username":  username,
		"CSRFToken": csrfToken,
		"Role":      roleOf(user),
		"Flash":     r.URL.Query().Get("msg"),
	})
}

func (h *DuplicateHandler) Data(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	zones, err := h.r53.ListZones(ctx)
	if err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to list zones: "+err.Error())
		return
	}

	ignoredRecords, err := h.db.GetIgnoredDuplicates()
	if err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to load ignored list: "+err.Error())
		return
	}

	ignoredCount, _ := h.db.GetIgnoredCount()

	valueMap := make(map[string][]model.RecordRef)

	for _, zone := range zones {
		records, err := h.r53.ListRecords(ctx, zone.ID)
		if err != nil {
			continue
		}

		for _, rec := range records {
			if rec.Type == "SOA" {
				continue
			}
			if rec.Type == "NS" && (rec.Name == zone.Name || rec.Name == zone.Name+".") {
				continue
			}

			var valuesToCheck []string
			if rec.IsAlias {
				valuesToCheck = []string{rec.AliasTarget}
			} else {
				valuesToCheck = rec.Values
			}

			for _, v := range valuesToCheck {
				normValue := strings.TrimSpace(strings.ToLower(v))
				if normValue == "" {
					continue
				}

				hash := hashValue(normValue)
				if ignoredRecords[hash] {
					continue
				}

				name := rec.Name
				if zone.Name != "" {
					name = shortName(rec.Name, zone.Name)
				}

				zoneLabel := zone.Name
				if zone.Label != "" {
					zoneLabel = zone.Label
				}

				valueMap[normValue] = append(valueMap[normValue], model.RecordRef{
					ZoneID:    zone.ID,
					ZoneLabel: zoneLabel,
					Name:      name,
					Type:      rec.Type,
				})
			}
		}
	}

	var duplicateGroups []model.DuplicateGroup
	for val, refs := range valueMap {
		if len(refs) < 2 {
			continue
		}

		duplicateGroups = append(duplicateGroups, model.DuplicateGroup{
			Value:     val,
			ValueHash: hashValue(val),
			Records:   refs,
		})
	}

	sort.Sort(byRecordCount(duplicateGroups))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"duplicates": duplicateGroups,
		"counters": map[string]int{
			"total":   len(duplicateGroups),
			"ignored": ignoredCount,
		},
	})
}

func (h *DuplicateHandler) writeJSONError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func (h *DuplicateHandler) Ignore(w http.ResponseWriter, r *http.Request) {
	username, _ := h.sessionMgr.GetUsername(r)
	_ = r.ParseForm()

	valueHash := r.FormValue("value_hash")
	valueText := r.FormValue("value_text")

	if valueHash == "" {
		http.Redirect(w, r, "/admin/duplicates", http.StatusSeeOther)
		return
	}

	if err := h.db.AddDuplicateIgnore(valueHash, valueText, username); err != nil {
		http.Redirect(w, r, "/admin/duplicates?msg="+fmt.Sprintf("Error: %v", err), http.StatusSeeOther)
		return
	}

	_ = h.db.LogAudit(model.AuditEntry{
		Username:  username,
		Action:    "ignore_duplicate",
		Detail:    fmt.Sprintf("value=%s hash=%s", valueText, valueHash),
		IPAddress: util.GetClientIP(r),
	})

	http.Redirect(w, r, "/admin/duplicates?msg=Duplicate value ignored successfully", http.StatusSeeOther)
}

func (h *DuplicateHandler) Reset(w http.ResponseWriter, r *http.Request) {
	username, _ := h.sessionMgr.GetUsername(r)

	if err := h.db.ResetIgnoredDuplicates(); err != nil {
		http.Redirect(w, r, "/admin/duplicates?msg="+fmt.Sprintf("Error: %v", err), http.StatusSeeOther)
		return
	}

	_ = h.db.LogAudit(model.AuditEntry{
		Username:  username,
		Action:    "reset_duplicate_ignores",
		Detail:    "All ignored duplicates have been reset",
		IPAddress: util.GetClientIP(r),
	})

	http.Redirect(w, r, "/admin/duplicates?msg=Ignored duplicates reset successfully", http.StatusSeeOther)
}

func (h *DuplicateHandler) renderError(w http.ResponseWriter, username, csrfToken, role, msg string) {
	h.tmpl.ExecuteTemplate(w, "layout", map[string]interface{}{
		"Title":     "Duplicate Records",
		"Username":  username,
		"CSRFToken": csrfToken,
		"Role":      role,
		"Error":     msg,
	})
}

func hashValue(s string) string {
	h := sha256.New()
	h.Write([]byte(s))
	return fmt.Sprintf("%x", h.Sum(nil))
}

// shortName is duplicated from records.go because it's unexported there.
func shortName(fqdn, zoneDomain string) string {
	suffix := "." + zoneDomain
	if fqdn == zoneDomain {
		return "@"
	}
	if strings.HasSuffix(fqdn, suffix) {
		return strings.TrimSuffix(fqdn, suffix)
	}
	return fqdn
}

type byRecordCount []model.DuplicateGroup

func (a byRecordCount) Len() int      { return len(a) }
func (a byRecordCount) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a byRecordCount) Less(i, j int) bool {
	if len(a[i].Records) != len(a[j].Records) {
		return len(a[i].Records) > len(a[j].Records)
	}
	return a[i].Value < a[j].Value
}
