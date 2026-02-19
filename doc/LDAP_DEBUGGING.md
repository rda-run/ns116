# LDAP Debugging and Configuration Guide

This document provides a step-by-step guide to debugging LDAP authentication
issues in NS116, specifically focusing on the **"User does not belong to any
group"** error.

## Prerequisites

You need the `ldapsearch` command-line tool installed on your machine or the
server to query the LDAP directory.

- **Debian/Ubuntu:** `sudo apt-get install ldap-utils`
- **RHEL/CentOS:** `sudo dnf install openldap-clients`
- **macOS:** Pre-installed or via `brew install openldap`

## 1. Understand How NS116 Validates Groups

The application uses two strategies to find groups, in this order:

1. **`memberOf` Attribute:** Checks if the user object has `memberOf` attributes
   containing the group DNs. (Common in Active Directory).
2. **Reverse Group Search:** If no `memberOf` is found, it searches the LDAP
   directory for Group objects where the `member` or `uniqueMember` attribute
   equals the User's DN. (Common in OpenLDAP).

If both strategies fail to return a group that matches your `config.yaml`
mapping, access is denied.

## 2. Step-by-Step Debugging

### Step A: Find the User's Full DN and Attributes

First, we need to see exactly how the LDAP server sees your user. Run this
command (replace values with your `config.yaml` settings):

```bash
# Replace these variables with your actual config values
LDAP_URL="ldaps://ldap.example.com:636"
BIND_DN="CN=ns116-svc,OU=ServiceAccounts,DC=example,DC=com"
BIND_PASS="your_service_account_password"
SEARCH_BASE="OU=Users,DC=example,DC=com"
USERNAME="jdoe" # The username you are trying to login with

# Command
ldapsearch -x -H "$LDAP_URL" \
  -D "$BIND_DN" \
  -w "$BIND_PASS" \
  -b "$SEARCH_BASE" \
  "(sAMAccountName=$USERNAME)" \
  dn memberOf
```

**Analyze the Output:**

1. **Check the `dn` line:** configuration `base_dn` must include this path.
2. **Check for `memberOf` lines:**
   - **Scenario 1:** You see lines like `memberOf:
     CN=DNS-Admins,OU=Groups,DC=example,DC=com`.
     - **Action:** Copy that *exact* string into your `config.yaml`.
   - **Scenario 2:** You do **not** see any `memberOf` lines.
     - **Action:** Proceed to Step B.

### Step B: Verify Group Membership Logic (OpenLDAP / Reverse Search)

If `memberOf` was empty, the application tries to find groups that contain the
user. You need the exact **User DN** returned from Step A (e.g., `CN=John
Doe,OU=Users,DC=example,DC=com`).

Run this command to simulate what the application does:

```bash
USER_DN="CN=John Doe,OU=Users,DC=example,DC=com" # From Step A result
GROUP_BASE="OU=Groups,DC=example,DC=com" # Usually same as base_dn or specific group OU

ldapsearch -x -H "$LDAP_URL" \
  -D "$BIND_DN" \
  -w "$BIND_PASS" \
  -b "$GROUP_BASE" \
  "(|(member=$USER_DN)(uniqueMember=$USER_DN))" \
  dn
```

**Analyze the Output:**

- If this returns a group DN (e.g., `dn:
  CN=Ns116-Admins,OU=Groups,DC=example,DC=com`), then the application *can* see
  the membership.
- **Action:** Copy that group DN into your `config.yaml`.

## 3. Configuring `config.yaml` Correctly

Open your `config.yaml` and update the `group_mapping` section with the **exact
DNs** found in the steps above.

```yaml
ldap:
  enabled: true
  url: "ldaps://ldap.example.com:636"
  
  # ... binding settings ...

  # IMPORTANT: This filter decides how we find the user in Step A
  # Active Directory: (sAMAccountName=%s)
  # OpenLDAP: (uid=%s)
  user_filter: "(sAMAccountName=%s)"

  group_mapping:
    # The value here must MATCH EXACTLY what ldapsearch returned.
    # It is usually the full Distinguished Name (DN).
    admin: "CN=DNS-Admins,OU=Groups,DC=example,DC=com"
    editor: "CN=DNS-Editors,OU=Groups,DC=example,DC=com"
```

## 4. Common Pitfalls

### Case Sensitivity & Spacing

LDAP is generally case-insensitive, but string matching in Go can be strict
about spaces.

- **Bad:** `CN=Admins, OU=Groups, DC=example, DC=com` (Extra spaces after
  commas)
- **Good:** `CN=Admins,OU=Groups,DC=example,DC=com` (Matches typical LDAP
  output)

**Recommendation:** Copy-paste the DN directly from the `ldapsearch` output.

### Nested Groups

**NS116 does not support nested groups.**

- If User A is in Group "Devs", and Group "Devs" is in Group "DNS-Admins".
- Initializing `group_mapping` to "DNS-Admins" will **fail**.
- **Fix:** Add User A directly to "DNS-Admins", or point the mapping to "Devs".

### "User not found" vs "Access Denied"

- **"Invalid credentials"**: The Bind DN/Password is wrong, or the User Search
  failed (User DN not found).
- **"Access denied: you are not in an authorized group"**: The User password is
  correct, but the Group strategies (Step A & B) failed to match the config.

### TLS/SSL Issues

If `ldapsearch` fails with "Can't contact LDAP server":

- Check if you need `InsecureSkipVerify: true` (if using self-signed certs).
- Ensure the port is correct (`636` for LDAPS, `389` for StartTLS/Plain).
