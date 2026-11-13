package handler

import (
	"encoding/json"
	"fmt"
	"html/template"
	"math"
	"net/http"
	"net/netip"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/route53/types"

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

	zoneName := zone.Name
	if zone.Label != "" {
		zoneName = zone.Label
	}

	success, failure := flash(r)
	h.tmpl.ExecuteTemplate(w, "layout", map[string]interface{}{
		"Title":      zoneName,
		"Username":   username,
		"CSRFToken":  csrfToken,
		"Role":       roleOf(user),
		"ZoneID":     zoneID,
		"ZoneName":   zoneName,
		"ZoneDomain": zone.Name,
		"Flash":      success,
		"Error":      failure,
	})
}

func (h *RecordHandler) Data(w http.ResponseWriter, r *http.Request) {
	zoneID := r.PathValue("zoneID")

	records, err := h.r53.ListRecords(r.Context(), zoneID)
	if err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to load records: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(records)
}

func (h *RecordHandler) writeJSONError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func qualifyName(name, zoneDomain string) string {
	name = strings.TrimSpace(name)
	if name == "" || name == "@" {
		return zoneDomain
	}
	if strings.HasSuffix(name, ".") {
		return name
	}
	if inZone(name+".", zoneDomain) {
		return name + "."
	}
	return name + "." + zoneDomain
}

// inZone reports whether the FQDN is the zone apex or a name below it.
func inZone(fqdn, zoneDomain string) bool {
	fqdn, zoneDomain = strings.ToLower(fqdn), strings.ToLower(zoneDomain)
	return fqdn == zoneDomain || strings.HasSuffix(fqdn, "."+zoneDomain)
}

// manageableType accepts every record type Route53 knows except SOA, which
// Route53 manages itself. Types missing from the form (DS, HTTPS, SPF...) stay
// allowed so that existing records of those types can still be deleted.
func manageableType(t string) bool {
	return t != "SOA" && slices.Contains(types.RRType("").Values(), types.RRType(t))
}

// validateChange is the server-side counterpart of what the UI already hides:
// no SOA, no apex NS, nothing outside the zone, and no malformed input.
// Route53 still has the final word on the value syntax of each type.
func validateChange(req model.RecordChangeRequest, zoneDomain string) error {
	if !manageableType(req.Type) {
		return fmt.Errorf("record type %q cannot be managed here", req.Type)
	}
	if !inZone(req.Name, zoneDomain) {
		return fmt.Errorf("record name %q is outside zone %s", req.Name, zoneDomain)
	}
	if req.Type == "NS" && strings.EqualFold(req.Name, zoneDomain) {
		return fmt.Errorf("the NS record of the zone apex cannot be changed")
	}
	if req.TTL < 0 || req.TTL > math.MaxInt32 {
		return fmt.Errorf("TTL %d is out of range", req.TTL)
	}
	if len(req.Values) == 0 {
		return fmt.Errorf("at least one value is required")
	}
	for _, v := range req.Values {
		if strings.TrimSpace(v) == "" {
			return fmt.Errorf("values cannot be empty")
		}
		if req.Type != "A" && req.Type != "AAAA" {
			continue
		}
		addr, err := netip.ParseAddr(v)
		if err != nil || addr.Is4() != (req.Type == "A") {
			return fmt.Errorf("%q is not a valid address for a %s record", v, req.Type)
		}
	}
	return nil
}

// zoneDomain returns the zone's domain name, or redirects back to the records
// page with the error. Changes are refused when the zone cannot be resolved,
// because they cannot be validated against it.
func (h *RecordHandler) zoneDomain(w http.ResponseWriter, r *http.Request, zoneID string) (string, bool) {
	zone, err := h.r53.GetZone(r.Context(), zoneID)
	if err != nil {
		msg := "Error: failed to load zone: " + err.Error()
		http.Redirect(w, r, fmt.Sprintf("/zones/%s/records?msg=%s", zoneID, url.QueryEscape(msg)), http.StatusSeeOther)
		return "", false
	}
	return zone.Name, true
}

func (h *RecordHandler) Create(w http.ResponseWriter, r *http.Request) {
	zoneID := r.PathValue("zoneID")
	username, _ := h.sessionMgr.GetUsername(r)
	_ = r.ParseForm()

	zoneDomain, ok := h.zoneDomain(w, r, zoneID)
	if !ok {
		return
	}

	req := model.RecordChangeRequest{
		Action: "CREATE",
		Name:   qualifyName(r.FormValue("name"), zoneDomain),
		Type:   r.FormValue("type"),
		TTL:    parseTTL(r.FormValue("ttl")),
		Values: r.Form["value"],
	}

	msg := "Record created successfully"
	err := validateChange(req, zoneDomain)
	if err == nil {
		err = h.r53.ChangeRecord(r.Context(), zoneID, req)
	}
	if err != nil {
		msg = "Error: " + err.Error()
	}

	_ = h.db.LogAudit(model.AuditEntry{
		Username:   username,
		Action:     "create_record",
		ZoneID:     zoneID,
		RecordName: req.Name,
		RecordType: req.Type,
		Detail:     auditDetail(fmt.Sprintf("values=[%s] ttl=%d", strings.Join(req.Values, ", "), req.TTL), err),
		IPAddress:  util.GetClientIP(r),
	})

	http.Redirect(w, r, fmt.Sprintf("/zones/%s/records?msg=%s", zoneID, url.QueryEscape(msg)), http.StatusSeeOther)
}

func (h *RecordHandler) Edit(w http.ResponseWriter, r *http.Request) {
	zoneID := r.PathValue("zoneID")
	username, _ := h.sessionMgr.GetUsername(r)
	_ = r.ParseForm()

	zoneDomain, ok := h.zoneDomain(w, r, zoneID)
	if !ok {
		return
	}

	originalName := r.FormValue("original_name")
	originalType := r.FormValue("original_type")
	newName := qualifyName(r.FormValue("name"), zoneDomain)
	newType := r.FormValue("type")

	var diffs []string
	if originalName != newName {
		diffs = append(diffs, fmt.Sprintf("name: %s -> %s", originalName, newName))
	}
	if originalType != newType {
		diffs = append(diffs, fmt.Sprintf("type: %s -> %s", originalType, newType))
	}
	oldTTLStr := r.FormValue("original_ttl")
	newTTLStr := r.FormValue("ttl")
	if oldTTLStr != newTTLStr {
		diffs = append(diffs, fmt.Sprintf("ttl: %s -> %s", oldTTLStr, newTTLStr))
	}
	oldVals := strings.Join(r.Form["original_value"], ", ")
	newVals := strings.Join(r.Form["value"], ", ")
	if oldVals != newVals {
		diffs = append(diffs, fmt.Sprintf("values: [%s] -> [%s]", oldVals, newVals))
	}
	detailStr := strings.Join(diffs, "; ")

	newReq := model.RecordChangeRequest{
		Action: "UPSERT",
		Name:   newName,
		Type:   newType,
		TTL:    parseTTL(r.FormValue("ttl")),
		Values: r.Form["value"],
	}
	changes := []model.RecordChangeRequest{newReq}

	// A record set is identified by name and type. Changing either means
	// deleting the old set and creating the new one, sent as a single change
	// batch so Route53 applies both or neither.
	if originalName != newName || originalType != newType {
		newReq.Action = "CREATE"
		changes = []model.RecordChangeRequest{
			{
				Action: "DELETE",
				Name:   originalName,
				Type:   originalType,
				TTL:    parseTTL(r.FormValue("original_ttl")),
				Values: r.Form["original_value"],
			},
			newReq,
		}
	}

	msg := "Record updated successfully"
	var err error
	for _, c := range changes {
		if err = validateChange(c, zoneDomain); err != nil {
			break
		}
	}
	if err == nil {
		err = h.r53.ChangeRecord(r.Context(), zoneID, changes...)
	}
	if err != nil {
		msg = "Error updating record: " + err.Error()
	}

	_ = h.db.LogAudit(model.AuditEntry{
		Username:   username,
		Action:     "edit_record",
		ZoneID:     zoneID,
		RecordName: newReq.Name,
		RecordType: newReq.Type,
		Detail:     auditDetail(detailStr, err),
		IPAddress:  util.GetClientIP(r),
	})

	http.Redirect(w, r, fmt.Sprintf("/zones/%s/records?msg=%s", zoneID, url.QueryEscape(msg)), http.StatusSeeOther)
}

func (h *RecordHandler) Delete(w http.ResponseWriter, r *http.Request) {
	zoneID := r.PathValue("zoneID")
	username, _ := h.sessionMgr.GetUsername(r)
	_ = r.ParseForm()

	zoneDomain, ok := h.zoneDomain(w, r, zoneID)
	if !ok {
		return
	}

	req := model.RecordChangeRequest{
		Action: "DELETE",
		Name:   r.FormValue("name"),
		Type:   r.FormValue("type"),
		TTL:    parseTTL(r.FormValue("ttl")),
		Values: r.Form["value"],
	}

	msg := "Record deleted successfully"
	err := validateChange(req, zoneDomain)
	if err == nil {
		err = h.r53.ChangeRecord(r.Context(), zoneID, req)
	}
	if err != nil {
		msg = "Error: " + err.Error()
	}

	_ = h.db.LogAudit(model.AuditEntry{
		Username:   username,
		Action:     "delete_record",
		ZoneID:     zoneID,
		RecordName: req.Name,
		RecordType: req.Type,
		Detail:     auditDetail(fmt.Sprintf("ttl=%d values=[%s]", req.TTL, strings.Join(req.Values, ", ")), err),
		IPAddress:  util.GetClientIP(r),
	})

	http.Redirect(w, r, fmt.Sprintf("/zones/%s/records?msg=%s", zoneID, url.QueryEscape(msg)), http.StatusSeeOther)
}

// auditDetail marks the entry as failed when the change was rejected, so the
// audit log does not read as if a refused change had been applied.
func auditDetail(detail string, err error) string {
	if err == nil {
		return detail
	}
	return strings.TrimPrefix(detail+"; FAILED: "+err.Error(), "; ")
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
