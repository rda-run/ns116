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
USER_UID=""
USER_EMAIL_ATTR="mail" # Default

# Try sAMAccountName (Active Directory)
if [ -z "$LOGIN_ATTR" ]; then
    OUTPUT=$(ldapsearch -x -H "$LDAP_URL" -D "$BIND_DN" -w "$BIND_PASS" -b "$BASE_DN" "(sAMAccountName=$TEST_USER)" dn sAMAccountName mail 2>/dev/null)
    if echo "$OUTPUT" | grep -q "^dn:"; then
        LOGIN_ATTR="sAMAccountName"
        USER_FILTER="(sAMAccountName=%s)"
        echo "${GREEN}✔ Detected Active Directory style server (sAMAccountName).${RESET}"
    fi
fi

# Try uid (OpenLDAP / Standard)
if [ -z "$LOGIN_ATTR" ]; then
    OUTPUT=$(ldapsearch -x -H "$LDAP_URL" -D "$BIND_DN" -w "$BIND_PASS" -b "$BASE_DN" "(uid=$TEST_USER)" dn uid mail 2>/dev/null)
    if echo "$OUTPUT" | grep -q "^dn:"; then
        LOGIN_ATTR="uid"
        USER_FILTER="(uid=%s)"
        echo "${GREEN}✔ Detected OpenLDAP/Unix style server (uid).${RESET}"
    fi
fi

# Try cn (Generic)
if [ -z "$LOGIN_ATTR" ]; then
    OUTPUT=$(ldapsearch -x -H "$LDAP_URL" -D "$BIND_DN" -w "$BIND_PASS" -b "$BASE_DN" "(cn=$TEST_USER)" dn cn mail 2>/dev/null)
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

# Extract User DN and UID (plain username)
USER_DN=$(echo "$OUTPUT" | grep "^dn:" | head -n1 | sed 's/^dn: //')
USER_UID=$(echo "$OUTPUT" | grep "^$LOGIN_ATTR:" | head -n1 | sed "s/^$LOGIN_ATTR: //")
echo "  User DN: $USER_DN"
echo "  User Login: $USER_UID"

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
    
    echo "Detected groups:"
    echo "$MEMBEROF" | sed 's/^memberOf: //' | while read group; do
        echo " - $group"
    done
    ADMIN_GROUP_DN=$(echo "$MEMBEROF" | head -n1 | sed 's/^memberOf: //')

else
    echo "  'memberOf' not found. Trying reverse search..."
    
    # Strategy B: Reverse Search (Standard DN based: groupOfNames/groupOfUniqueNames)
    echo "  Checking for standard DN-based groups (member/uniqueMember)..."
    GROUP_SEARCH=$(ldapsearch -x -H "$LDAP_URL" -D "$BIND_DN" -w "$BIND_PASS" -b "$BASE_DN" "(|(member=$USER_DN)(uniqueMember=$USER_DN))" dn 2>/dev/null | grep "^dn:")
    
    if [ ! -z "$GROUP_SEARCH" ]; then
         echo "${GREEN}✔ Groups found via reverse search (member/uniqueMember).${RESET}"
         GROUP_STRATEGY="reverse_search_dn"
         
         echo "Detected groups:"
         echo "$GROUP_SEARCH" | sed 's/^dn: //' | while read group; do
            echo " - $group"
         done
         ADMIN_GROUP_DN=$(echo "$GROUP_SEARCH" | head -n1 | sed 's/^dn: //')

    else
        # Strategy C: POSIX Groups (memberUid with plain username)
        echo "  Checking for POSIX groups (memberUid)..."
        GROUP_SEARCH_POSIX=$(ldapsearch -x -H "$LDAP_URL" -D "$BIND_DN" -w "$BIND_PASS" -b "$BASE_DN" "(&(objectClass=posixGroup)(memberUid=$USER_UID))" dn 2>/dev/null | grep "^dn:")

        if [ ! -z "$GROUP_SEARCH_POSIX" ]; then
             echo "${GREEN}✔ POSIX Groups found (memberUid).${RESET}"
             GROUP_STRATEGY="posix_group"
             
             echo "Detected groups:"
             echo "$GROUP_SEARCH_POSIX" | sed 's/^dn: //' | while read group; do
                echo " - $group"
             done
             ADMIN_GROUP_DN=$(echo "$GROUP_SEARCH_POSIX" | head -n1 | sed 's/^dn: //')
    else
        echo "${RED}⚠ No groups found for this user.${RESET}"
        echo "Please verify if the user is a member of any group."
    fi
fi

# 5. Specify and Validate Roles
echo ""
echo "${YELLOW}Step 4: Role Configuration${RESET}"
echo "We detected potential group candidates above. Now, please specify which groups should be mapped to NS116 roles."
echo "You can copy-paste from the list above."

# Function to check if user is in a group (string match from founded groups)
check_membership() {
    local target_group="$1"
    # MEMBEROF or GROUP_SEARCH contains the list of groups found
    if echo "$MEMBEROF$GROUP_SEARCH$GROUP_SEARCH_POSIX" | grep -Fq "$target_group"; then
        return 0 # True
    else
        return 1 # False
    fi
}

# Admin Group
while true; do
    read -p "Enter LDAP Group DN for ADMIN role (default: $ADMIN_GROUP_DN): " INPUT_ADMIN
    ACTUAL_ADMIN="${INPUT_ADMIN:-$ADMIN_GROUP_DN}"
    
    if [ -z "$ACTUAL_ADMIN" ]; then
        echo "${RED}Admin group cannot be empty.${RESET}"
        continue
    fi
    
    if check_membership "$ACTUAL_ADMIN"; then
        echo "${GREEN}✔ User '$TEST_USER' is a member of this Admin group.${RESET}"
    else
        echo "${YELLOW}⚠ User '$TEST_USER' is NOT a member of '$ACTUAL_ADMIN'.${RESET}"
        echo "This is fine if you are configuring for other users, but double-check spelling."
    fi
    break
done

# Editor Group
read -p "Enter LDAP Group DN for EDITOR role (optional): " INPUT_EDITOR
ACTUAL_EDITOR="$INPUT_EDITOR"

IS_ADMIN=0
IS_EDITOR=0

if check_membership "$ACTUAL_ADMIN"; then IS_ADMIN=1; fi
if [ ! -z "$ACTUAL_EDITOR" ] && check_membership "$ACTUAL_EDITOR"; then IS_EDITOR=1; fi

# Check Conflict/Precedence
echo ""
echo "${YELLOW}Checking Permissions for '$TEST_USER'...${RESET}"
if [ $IS_ADMIN -eq 1 ] && [ $IS_EDITOR -eq 1 ]; then
    echo "User is in BOTH Admin and Editor groups."
    echo "NS116 Logic: ${GREEN}ADMIN wins${RESET} (First match priority: Admin -> Editor)."
elif [ $IS_ADMIN -eq 1 ]; then
    echo "User is in Admin group only -> Role: ${GREEN}ADMIN${RESET}"
elif [ $IS_EDITOR -eq 1 ]; then
    echo "User is in Editor group only -> Role: ${GREEN}EDITOR${RESET}"
else
    echo "${RED}User is in NEITHER group -> Access Denied.${RESET}"
fi

# 5. Generate Configuration
echo ""
echo "${GREEN}=== Generated Configuration for ns116/config.yaml ===${RESET}"
echo "Copy and paste the block below into your config.yaml file:"
echo ""
echo "ldap:"
echo "  enabled: true"
# ... standard config structure ...
echo "  url: \"$LDAP_URL\""
echo "  bind_dn: \"$BIND_DN\""
echo "  bind_password: \"$BIND_PASS\""
echo "  base_dn: \"$BASE_DN\""
echo "  user_filter: \"$USER_FILTER\""
echo "  username_attr: \"$LOGIN_ATTR\""
echo "  email_attr: \"$USER_EMAIL_ATTR\""
if [ "$GROUP_STRATEGY" = "posix_group" ]; then
    echo "  # POSIX Group Configuration (memberUid)"
    echo "  group_filter: \"(&(objectClass=posixGroup)(memberUid=%u))\""
else
    echo "  # Standard Group Configuration (member/uniqueMember)"
    echo "  # Default Value (implied): (|(member=%s)(uniqueMember=%s))"
    echo "  group_filter: \"\""
fi
echo "  group_mapping:"
echo "    # Replace with the actual groups from your directory"
echo "    admin: \"$ACTUAL_ADMIN\""
echo "    editor: \"${ACTUAL_EDITOR:-CN=DNS-Editors,OU=Groups,$BASE_DN}\""

echo ""
echo "${YELLOW}=== IMPORTANT NOTES ===${RESET}"
if [ "$GROUP_STRATEGY" = "reverse_search_dn" ]; then
    echo "Your LDAP uses standard groups (member/uniqueMember). NS116 supports this natively."
elif [ "$GROUP_STRATEGY" = "memberOf" ]; then
     echo "Your LDAP supports memberOf. NS116 supports this natively."
elif [ "$GROUP_STRATEGY" = "posix_group" ]; then
     echo "${GREEN}Your LDAP uses POSIX groups (memberUid).${RESET}"
     echo "We have added the 'group_filter' parameter to support this configuration."
fi
echo ""
