package auth

import (
	"crypto/tls"
	"fmt"
	"strings"

	"ns116/internal/config"

	"github.com/go-ldap/ldap/v3"
)

type LDAPResult struct {
	Username string
	Email    string
	Groups   []string
}

type LDAPClient struct {
	cfg config.LDAPConfig
}

func NewLDAPClient(cfg config.LDAPConfig) *LDAPClient {
	return &LDAPClient{cfg: cfg}
}

// Authenticate performs a two-step LDAP auth:
// 1. Bind with the service account to search for the user
// 2. Bind with the user's DN + password to verify credentials
func (lc *LDAPClient) Authenticate(username, password string) (*LDAPResult, error) {
	conn, err := lc.connect()
	if err != nil {
		return nil, fmt.Errorf("ldap connect: %w", err)
	}
	defer conn.Close()

	// Step 1: Service account bind + user search
	if err := conn.Bind(lc.cfg.BindDN, lc.cfg.BindPassword); err != nil {
		return nil, fmt.Errorf("ldap service bind: %w", err)
	}

	filter := fmt.Sprintf(lc.cfg.UserFilter, ldap.EscapeFilter(username))
	searchReq := ldap.NewSearchRequest(
		lc.cfg.BaseDN,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases, 0, 30, false,
		filter,
		[]string{"dn", lc.cfg.UsernameAttr, lc.cfg.EmailAttr, "memberOf"},
		nil,
	)

	result, err := conn.Search(searchReq)
	if err != nil {
		return nil, fmt.Errorf("ldap search: %w", err)
	}
	if len(result.Entries) != 1 {
		return nil, fmt.Errorf("user not found or ambiguous: %d results", len(result.Entries))
	}

	entry := result.Entries[0]
	userDN := entry.DN

	// Step 2: User bind to verify password
	if err := conn.Bind(userDN, password); err != nil {
		return nil, fmt.Errorf("ldap user bind: %w", err)
	}

	groups := entry.GetAttributeValues("memberOf")
	if len(groups) == 0 {
		// Fallback: search for groups where the user is a member
		// We support custom filters for POSIX groups (memberUid=%u) or standard groups (member=%s)

		filterTmpl := lc.cfg.GroupFilter
		if filterTmpl == "" {
			filterTmpl = "(|(member=%s)(uniqueMember=%s))"
		}

		// Perform substitutions
		// %s -> User DN
		// %u -> User Login (UID/sAMAccountName)
		finalFilter := strings.ReplaceAll(filterTmpl, "%s", ldap.EscapeFilter(userDN))
		finalFilter = strings.ReplaceAll(finalFilter, "%u", ldap.EscapeFilter(entry.GetAttributeValue(lc.cfg.UsernameAttr)))

		groupSearch := ldap.NewSearchRequest(
			lc.cfg.BaseDN,
			ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
			finalFilter,
			[]string{"dn"},
			nil,
		)
		if groupResult, err := conn.Search(groupSearch); err == nil {
			for _, ge := range groupResult.Entries {
				groups = append(groups, ge.DN)
			}
		}
	}

	return &LDAPResult{
		Username: entry.GetAttributeValue(lc.cfg.UsernameAttr),
		Email:    entry.GetAttributeValue(lc.cfg.EmailAttr),
		Groups:   groups,
	}, nil
}

// ResolveRole maps LDAP groups to NS116 roles using group_mapping.
// Returns ("", false) if the user is not in any mapped group.
// Priority: "admin" is checked first, then "editor".
func (lc *LDAPClient) ResolveRole(groups []string) (string, bool) {
	// Check admin first (highest privilege wins)
	if adminGroup, ok := lc.cfg.GroupMapping["admin"]; ok {
		for _, g := range groups {
			if strings.EqualFold(g, adminGroup) {
				return "admin", true
			}
		}
	}

	// Then check editor
	if editorGroup, ok := lc.cfg.GroupMapping["editor"]; ok {
		for _, g := range groups {
			if strings.EqualFold(g, editorGroup) {
				return "editor", true
			}
		}
	}

	// User is not in any mapped group — deny access
	return "", false
}

func (lc *LDAPClient) connect() (*ldap.Conn, error) {
	tlsCfg := &tls.Config{InsecureSkipVerify: lc.cfg.SkipVerify}

	if strings.HasPrefix(lc.cfg.URL, "ldaps://") {
		return ldap.DialURL(lc.cfg.URL, ldap.DialWithTLSConfig(tlsCfg))
	}

	conn, err := ldap.DialURL(lc.cfg.URL)
	if err != nil {
		return nil, err
	}

	if lc.cfg.StartTLS {
		if err := conn.StartTLS(tlsCfg); err != nil {
			conn.Close()
			return nil, fmt.Errorf("starttls: %w", err)
		}
	}

	return conn, nil
}
