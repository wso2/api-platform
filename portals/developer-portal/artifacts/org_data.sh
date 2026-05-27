#!/bin/bash
# Read database configuration from config.json
CONFIG_FILE="./config.json"
DB_HOST=$(grep -A 6 '"db"' "$CONFIG_FILE" | grep '"host"' | sed 's/.*"host": "\(.*\)".*/\1/')
DB_PORT=$(grep -A 6 '"db"' "$CONFIG_FILE" | grep '"port"' | sed 's/.*"port": \(.*\),.*/\1/')
DB_NAME=$(grep -A 6 '"db"' "$CONFIG_FILE" | grep '"database"' | sed 's/.*"database": "\(.*\)".*/\1/')
DB_USER=$(grep -A 6 '"db"' "$CONFIG_FILE" | grep '"username"' | sed 's/.*"username": "\(.*\)".*/\1/')

SQL_FILE="./artifacts/script.sql"

# Create Tables
psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -f "$SQL_FILE"

# Directory containing the files
DIRECTORY="./artifacts/default/orgContent/ACME"

# Organization data to insert
ORG_ID="1ba42a09-45c0-40f8-a1bf-e4aa7cde1575"
ORG_NAME="ACME"
BUSINESS_OWNER=""
BUSINESS_OWNER_CONTACT=""
BUSINESS_OWNER_EMAIL=""
ORG_HANDLE="ACME"
ROLE_CLAIM_NAME="roles"
GROUPS_CLAIM_NAME="groups"
ORGANIZATION_CLAIM_NAME="organizationID"
ORGANIZATION_IDENTIFIER="ACME"
ADMIN_ROLE="admin"
SUBSCRIBER_ROLE="Internal/subscriber"
SUPER_ADMIN_ROLE="superAdmin"

SQL="
INSERT INTO \"DP_ORGANIZATION\" (
    \"ORG_ID\",
    \"ORG_NAME\",
    \"BUSINESS_OWNER\",
    \"BUSINESS_OWNER_CONTACT\",
    \"BUSINESS_OWNER_EMAIL\",
    \"ORG_HANDLE\",
    \"ROLE_CLAIM_NAME\",
    \"GROUPS_CLAIM_NAME\",
    \"ORGANIZATION_CLAIM_NAME\",
    \"ORGANIZATION_IDENTIFIER\",
    \"ADMIN_ROLE\",
    \"SUBSCRIBER_ROLE\",
    \"SUPER_ADMIN_ROLE\",
    \"ORG_CONFIG\"
) VALUES (
    '$ORG_ID',
    '$ORG_NAME',
    '$BUSINESS_OWNER',
    '$BUSINESS_OWNER_CONTACT',
    '$BUSINESS_OWNER_EMAIL',
    '$ORG_HANDLE',
    '$ROLE_CLAIM_NAME',
    '$GROUPS_CLAIM_NAME',
    '$ORGANIZATION_CLAIM_NAME',
    '$ORGANIZATION_IDENTIFIER',
    '$ADMIN_ROLE',
    '$SUBSCRIBER_ROLE',
    '$SUPER_ADMIN_ROLE',
    '{}'
);
"

# Execute the SQL statement using psql
psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -c "$SQL"

# Create default view (or get existing one)
VIEW_ID=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -Atc "SELECT \"VIEW_ID\" FROM \"DP_VIEWS\" WHERE \"ORG_ID\" = '$ORG_ID' AND \"NAME\" = 'default' LIMIT 1;")
if [ -z "$VIEW_ID" ]; then
    VIEW_ID=$(uuidgen | tr '[:upper:]' '[:lower:]')
    VIEW_SQL="
    INSERT INTO \"DP_VIEWS\" (\"VIEW_ID\", \"ORG_ID\", \"NAME\", \"DISPLAY_NAME\")
    VALUES ('$VIEW_ID', '$ORG_ID', 'default', 'default');
    "
    psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -c "$VIEW_SQL"
fi

# Create default label (or get existing one)
LABEL_ID=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -Atc "SELECT \"LABEL_ID\" FROM \"DP_LABELS\" WHERE \"ORG_ID\" = '$ORG_ID' AND \"NAME\" = 'default' LIMIT 1;")
if [ -z "$LABEL_ID" ]; then
    LABEL_ID=$(uuidgen | tr '[:upper:]' '[:lower:]')
    LABEL_SQL="
    INSERT INTO \"DP_LABELS\" (\"LABEL_ID\", \"ORG_ID\", \"NAME\", \"DISPLAY_NAME\")
    VALUES ('$LABEL_ID', '$ORG_ID', 'default', 'default');
    "
    psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -c "$LABEL_SQL"
fi

# Link label to view
VIEW_LABEL_ID=$(uuidgen | tr '[:upper:]' '[:lower:]')
VIEW_LABEL_SQL="
INSERT INTO \"DP_VIEW_LABELS\" (\"ID\", \"LABEL_ID\", \"VIEW_ID\", \"ORG_ID\")
VALUES ('$VIEW_LABEL_ID', '$LABEL_ID', '$VIEW_ID', '$ORG_ID')
ON CONFLICT (\"LABEL_ID\", \"VIEW_ID\", \"ORG_ID\") DO NOTHING;
"
psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -c "$VIEW_LABEL_SQL"

# Loop through files in the directory
process_directory() {
    local DIR="$1"
    local DIR_NAME="";
    for file in "$DIR"/*; do
        if [ -d "$file" ]; then
            #echo "Entering directory: $DIR_NAME"
            process_directory "$file"
        elif [ -f "$file" ]; then
            BASE_NAME=$(dirname "$file")
            FILE_PATH=$(echo "$BASE_NAME" | sed 's|./artifacts/default/orgContent/ACME/||')
            echo "Base name: $BASE_NAME"
            DIR_NAME=$(basename "$(dirname "$file")")
            echo "Dir name: $DIR_NAME"
            echo "Processing file: $file"

            # Get the file name and file extension
            FILE_NAME=$(basename "$file")
            if [[ "$FILE_NAME" == *.css ]]; then
                FILE_TYPE="style"
            elif [[ "$FILE_NAME" == *.hbs && "$DIR_NAME" == layout ]]; then
                FILE_TYPE="layout"
            elif [[ "$FILE_NAME" == *.hbs  && "$DIR_NAME" == partials ]]; then
                FILE_TYPE="partial"
            elif [[ "$FILE_NAME" == *.md && "$DIR_NAME" == content ]]; then
                FILE_TYPE="markDown"
            elif [[ "$FILE_NAME" == *.hbs ]]; then
                FILE_TYPE="template"
            else
                FILE_TYPE="image"
            fi
        
            echo "Processing file: $FILE_PATH"

            # Use \lo_import to import the file as a large object and store its OID
            #PATH=$(cd "$(dirname "$file")" && pwd)/$(basename "$file")

            OID=$(psql -U "$DB_USER" -d "$DB_NAME" -h "$DB_HOST" -p "$DB_PORT" -Atc "\lo_import '$file'" | awk '{print $NF}')

            echo "OID: $OID"

            INSERT_QUERY="INSERT INTO \"DP_ORGANIZATION_ASSETS\" (\"ASSET_ID\", \"FILE_CONTENT\", \"FILE_NAME\", \"FILE_TYPE\", \"FILE_PATH\", \"ORG_ID\", \"VIEW_ID\") VALUES ( gen_random_uuid(), lo_get($OID), '$FILE_NAME', '$FILE_TYPE', '$FILE_PATH', '$ORG_ID', '$VIEW_ID');"

            # Call the SQL function to insert the data into the DP_ORGANIZATION_ASSETS table
            # psql -q -U postgres -d test  -h  localhost -p 5432  -c "SELECT insert_large_object('$FILE_NAME', '$FILE_TYPE', '$FILE_PATH', '$ORG_ID', $OID);"
            psql -U "$DB_USER" -d "$DB_NAME" -h "$DB_HOST" -p "$DB_PORT" -c "$INSERT_QUERY"
            if [ $? -eq 0 ]; then
                echo "Successfully inserted: $FILE_NAME"
            else
                echo "Failed to insert: $FILE_NAME"
            fi
        fi
    done
}

process_directory "$DIRECTORY"
