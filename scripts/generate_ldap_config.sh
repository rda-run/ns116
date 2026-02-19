#!/bin/bash

# NSA116 LDAP Auto-Configuration Script
# This script analyzes your LDAP server and generates the correct configuration for config.yaml

GREEN=$(tput setaf 2)
RED=$(tput setaf 1)
YELLOW=$(tput setaf 3)
RESET=$(tput sgr0)

echo "${GREEN}=== NS116 LDAP Auto-Configuration Tool ===${RESET}"
echo "This script will connect to your LDAP server, analyze the structure"
echo "and generate the necessary configuration block for NS116."
echo ""

# 1. Check Dependencies
if ! command -v ldapsearch &> /dev/null; then
    echo "${RED}Error: 'ldapsearch' not found.${RESET}"
    echo "Please install the ldap-utils (Debian/Ubuntu) or openldap-clients (RHEL/CentOS) package."
    exit 1
fi

# 2. Collect Service Credentials
echo "${YELLOW}Step 1: Connection Credentials (Service Account)${RESET}"
read -p "LDAP URL (e.g., ldaps://ldap.example.com:636): " LDAP_URL
read -p "Base DN (e.g., DC=example,DC=com): " BASE_DN
read -p "Bind DN (Service Account User): " BIND_DN
read -s -p "Bind Password: " BIND_PASS
echo ""

# Test basic connection
echo ""
echo "Testing connection..."
if ldapsearch -x -H "$LDAP_URL" -D "$BIND_DN" -w "$BIND_PASS" -b "$BASE_DN" -s base "(objectClass=*)" > /dev/null 2>&1; then
    echo "${GREEN}✔ LDAP connection established successfully!${RESET}"
else
    echo "${RED}✖ Failed to connect to LDAP server.${RESET}"
    echo "Check the URL, credentials, and network connectivity."
    echo "Tip: If using a self-signed certificate, add 'TLS_REQCERT never' to /etc/ldap/ldap.conf"
    exit 1
fi

# 3. Analyze a Test User
echo ""
echo "${YELLOW}Step 2: User Analysis${RESET}"
echo "We need a valid user to understand the directory structure."
read -p "Enter the login of an existing user (e.g., jdoe): " TEST_USER

echo "Analyzing user '$TEST_USER'..."

# Try to discover the login attribute
LOGIN_ATTR=""
USER_DN=""
USER_EMAIL_ATTR="mail" # Default

# Try sAMAccountName (Active Directory)
if [ -z "$LOGIN_ATTR" ]; then
    OUTPUT=$(ldapsearch -x -H "$LDAP_URL" -D "$BIND_DN" -w "$BIND_PASS" -b "$BASE_DN" "(sAMAccountName=$TEST_USER)" dn mail 2>/dev/null)
    if echo "$OUTPUT" | grep -q "^dn:"; then
        LOGIN_ATTR="sAMAccountName"
        USER_FILTER="(sAMAccountName=%s)"
        echo "${GREEN}✔ Detected Active Directory style server (sAMAccountName).${RESET}"
    fi
fi

# Try uid (OpenLDAP / Standard)
if [ -z "$LOGIN_ATTR" ]; then
    OUTPUT=$(ldapsearch -x -H "$LDAP_URL" -D "$BIND_DN" -w "$BIND_PASS" -b "$BASE_DN" "(uid=$TEST_USER)" dn mail 2>/dev/null)
    if echo "$OUTPUT" | grep -q "^dn:"; then
        LOGIN_ATTR="uid"
        USER_FILTER="(uid=%s)"
        echo "${GREEN}✔ Detected OpenLDAP/Unix style server (uid).${RESET}"
    fi
fi

# Try cn (Generic)
if [ -z "$LOGIN_ATTR" ]; then
    OUTPUT=$(ldapsearch -x -H "$LDAP_URL" -D "$BIND_DN" -w "$BIND_PASS" -b "$BASE_DN" "(cn=$TEST_USER)" dn mail 2>/dev/null)
    if echo "$OUTPUT" | grep -q "^dn:"; then
        LOGIN_ATTR="cn"
        USER_FILTER="(cn=%s)"
        echo "${GREEN}✔ Detected generic server (cn).${RESET}"
    fi
fi

if [ -z "$LOGIN_ATTR" ]; then
    echo "${RED}✖ Could not find user '$TEST_USER' using sAMAccountName, uid, or cn.${RESET}"
    echo "Check if the user exists in the provided Base DN."
    exit 1
fi

# Extract User DN
USER_DN=$(echo "$OUTPUT" | grep "^dn:" | head -n1 | sed 's/^dn: //')
echo "  User DN: $USER_DN"

# 4. Analyze Groups
echo ""
echo "${YELLOW}Step 3: Group Analysis${RESET}"

GROUP_STRATEGY="unknown"
ADMIN_GROUP_DN=""

# Strategy A: memberOf (Active Directory)
echo "Checking for 'memberOf' support..."
MEMBEROF=$(ldapsearch -x -H "$LDAP_URL" -D "$BIND_DN" -w "$BIND_PASS" -b "$BASE_DN" "($LOGIN_ATTR=$TEST_USER)" memberOf 2>/dev/null | grep "^memberOf:")

if [ ! -z "$MEMBEROF" ]; then
    echo "${GREEN}✔ 'memberOf' attribute found on user.${RESET}"
    GROUP_STRATEGY="memberOf"
    
    # List found groups
    echo "Detected groups:"
    echo "$MEMBEROF" | sed 's/^memberOf: //' | while read group; do
        echo " - $group"
    done
    
    # Take the first group for config example
    ADMIN_GROUP_DN=$(echo "$MEMBEROF" | head -n1 | sed 's/^memberOf: //')
else
    echo "  'memberOf' not found or empty. Trying reverse search..."
    
    # Strategy B: Reverse Search (OpenLDAP - groupOfNames/groupOfUniqueNames)
    # Search for groups that have this user as a member
    GROUP_SEARCH=$(ldapsearch -x -H "$LDAP_URL" -D "$BIND_DN" -w "$BIND_PASS" -b "$BASE_DN" "(|(member=$USER_DN)(uniqueMember=$USER_DN))" dn 2>/dev/null | grep "^dn:")
    
    if [ ! -z "$GROUP_SEARCH" ]; then
         echo "${GREEN}✔ Groups found via reverse search (member/uniqueMember).${RESET}"
         GROUP_STRATEGY="reverse_search"
         
         echo "Detected groups:"
         echo "$GROUP_SEARCH" | sed 's/^dn: //' | while read group; do
            echo " - $group"
         done
         
         ADMIN_GROUP_DN=$(echo "$GROUP_SEARCH" | head -n1 | sed 's/^dn: //')
    else
        echo "${RED}⚠ No groups found for this user.${RESET}"
        echo "You will need to manually fill in the Group DN in the configuration."
        ADMIN_GROUP_DN="CN=DNS-Admins,OU=Groups,$BASE_DN"
    fi
fi

# 5. Generate Configuration
echo ""
echo "${GREEN}=== Generated Configuration for ns116/config.yaml ===${RESET}"
echo "Copy and paste the block below into your config.yaml file:"
echo ""
echo "ldap:"
echo "  enabled: true"
echo "  url: \"$LDAP_URL\""
echo "  bind_dn: \"$BIND_DN\""
echo "  bind_password: \"$BIND_PASS\""
echo "  base_dn: \"$BASE_DN\""
echo "  user_filter: \"$USER_FILTER\""
echo "  username_attr: \"$LOGIN_ATTR\""
echo "  email_attr: \"$USER_EMAIL_ATTR\""
echo "  group_mapping:"
echo "    # Replace with the actual groups from your directory"
echo "    admin: \"$ADMIN_GROUP_DN\""
echo "    editor: \"CN=DNS-Editors,OU=Groups,$BASE_DN\""

echo ""
if [ "$GROUP_STRATEGY" = "reverse_search" ]; then
    echo "${YELLOW}Note: Since your LDAP uses reverse search (OpenLDAP), NS116 will automatically detect groups.${RESET}"
elif [ "$GROUP_STRATEGY" = "memberOf" ]; then
     echo "${YELLOW}Note: NS116 will use the memberOf attribute to check groups.${RESET}"
fi
echo ""
