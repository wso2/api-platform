#!/bin/bash
CONFIG_FILE="./config.json"
DB_HOST=$(grep -A 6 '"db"' "$CONFIG_FILE" | grep '"host"' | sed 's/.*"host": "\(.*\)".*/\1/')
DB_PORT=$(grep -A 6 '"db"' "$CONFIG_FILE" | grep '"port"' | sed 's/.*"port": \(.*\),.*/\1/')
DB_NAME=$(grep -A 6 '"db"' "$CONFIG_FILE" | grep '"database"' | sed 's/.*"database": "\(.*\)".*/\1/')
DB_USER=$(grep -A 6 '"db"' "$CONFIG_FILE" | grep '"username"' | sed 's/.*"username": "\(.*\)".*/\1/')
ORG_ID="1ba42a09-45c0-40f8-a1bf-e4aa7cde1575"

 insert_api_data() {

   # Read JSON fields using jq
    API_ID="$1"
    REFERENCE_ID=""
    API_NAME="$1"
    STATUS="PUBLISHED"
    API_HANDLE="$1"
    API_DESCRIPTION="$2"
    API_VERSION="$3"
    API_TYPE="$4"
    VISIBILITY="PUBLIC"
    VISIBLE_GROUPS=""
    TECHNICAL_OWNER=""
    BUSINESS_OWNER=""
    TECHNICAL_OWNER_EMAIL=""
    BUSINESS_OWNER_EMAIL=""
    PRODUCTION_URL="$5"
    SANDBOX_URL="$5"
    METADATA_SEARCH={}
    PROVIDER="DEFAULT"
    TAGS="$6"

    # Insert data into PostgreSQL table
    psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -c "
    INSERT INTO \"DP_API_METADATA\" (
        \"API_ID\", \"REFERENCE_ID\", \"ORG_ID\", \"API_NAME\", \"STATUS\", \"API_HANDLE\", \"API_DESCRIPTION\",
        \"API_VERSION\", \"API_TYPE\", \"VISIBILITY\", \"VISIBLE_GROUPS\", \"TECHNICAL_OWNER\",
        \"BUSINESS_OWNER\", \"TECHNICAL_OWNER_EMAIL\", \"BUSINESS_OWNER_EMAIL\", \"PRODUCTION_URL\",
        \"SANDBOX_URL\", \"METADATA_SEARCH\", \"PROVIDER\", \"TAGS\"
    ) VALUES (
        '$API_ID', '$REFERENCE_ID', '$ORG_ID', '$API_NAME', '$STATUS', '$API_HANDLE', '$API_DESCRIPTION',
        '$API_VERSION', '$API_TYPE', '$VISIBILITY', '$VISIBLE_GROUPS', '$TECHNICAL_OWNER',
        '$BUSINESS_OWNER', '$TECHNICAL_OWNER_EMAIL', '$BUSINESS_OWNER_EMAIL', '$PRODUCTION_URL',
        '$SANDBOX_URL', '$METADATA_SEARCH'::json, '$PROVIDER', '$TAGS'
    ) ON CONFLICT (\"API_ID\") DO NOTHING;
    "

    # Link API to the default label so it shows up in view-based listings.
    # The API listing query requires APIs to have labels that are attached to the current view (default view uses 'default' label).
    psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -c "
    INSERT INTO \"DP_API_LABELS\" (\"ID\", \"LABEL_ID\", \"API_ID\", \"ORG_ID\")
    SELECT gen_random_uuid()::text, l.\"LABEL_ID\", '$API_ID', '$ORG_ID'
    FROM \"DP_LABELS\" l
    WHERE l.\"ORG_ID\" = '$ORG_ID' AND l.\"NAME\" = 'default'
    ON CONFLICT (\"LABEL_ID\", \"API_ID\", \"ORG_ID\") DO NOTHING;
    "
    echo "Data inserted successfully!"
}

 insert_image_data() {
    API_ID="$1"
    IMAGE_NAME="$2"
    IMAGE_TAG="$3"
    psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -c "
    INSERT INTO \"DP_API_IMAGEDATA\" (
        \"API_ID\", \"IMAGE_NAME\", \"IMAGE_TAG\"
    ) VALUES (
        '$API_ID', '$IMAGE_NAME', '$IMAGE_TAG'
    ) ON CONFLICT (\"API_ID\", \"IMAGE_NAME\", \"IMAGE_TAG\") DO NOTHING;
    "
 }

 insert_subscription_policy_data() {
    psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -c "
    INSERT INTO \"DP_SUBSCRIPTION_POLICY\" (
        \"ORG_ID\", \"POLICY_ID\", \"POLICY_NAME\", \"DISPLAY_NAME\", \"BILLING_PLAN\", \"DESCRIPTION\"
    ) VALUES (
        '$ORG_ID', '1ba42a09-45c0-40f8-a1bf-e4aa7cde1234', 'Gold', 'Gold', 'FREE', 'Allows 1000 total tokens and 100 requests per minute'
    ) ON CONFLICT (\"POLICY_ID\") DO NOTHING;
    INSERT INTO \"DP_SUBSCRIPTION_POLICY\" (
        \"ORG_ID\", \"POLICY_ID\", \"POLICY_NAME\", \"DISPLAY_NAME\", \"BILLING_PLAN\", \"DESCRIPTION\"
    ) VALUES (
        '$ORG_ID', '1ba42a09-45c0-40f8-a1bf-e4aa7cde2345', 'Silver', 'Silver', 'FREE', 'Allows 2000 requests per minute'
    ) ON CONFLICT (\"POLICY_ID\") DO NOTHING;
    INSERT INTO \"DP_SUBSCRIPTION_POLICY\" (
        \"ORG_ID\", \"POLICY_ID\", \"POLICY_NAME\", \"DISPLAY_NAME\", \"BILLING_PLAN\", \"DESCRIPTION\"
    ) VALUES (
        '$ORG_ID', '1ba42a09-45c0-40f8-a1bf-e4aa7cde3456', 'Bronze', 'Bronze', 'FREE', 'Allows 1000 requests per minute'
    ) ON CONFLICT (\"POLICY_ID\") DO NOTHING;
    INSERT INTO \"DP_SUBSCRIPTION_POLICY\" (
        \"ORG_ID\", \"POLICY_ID\", \"POLICY_NAME\", \"DISPLAY_NAME\", \"BILLING_PLAN\", \"DESCRIPTION\"
    ) VALUES (
        '$ORG_ID', '1ba42a09-45c0-40f8-a1bf-e4aa7cde4567', 'Unlimited', 'Unlimited', 'FREE', 'Allows unlimited requests'
    ) ON CONFLICT (\"POLICY_ID\") DO NOTHING;
    "
 }

 insert_api_subscription_data() {
    psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -c "
    INSERT INTO \"DP_API_SUBSCRIPTION_POLICY\" (
        \"POLICY_ID\", \"API_ID\"
    ) VALUES 
        ('1ba42a09-45c0-40f8-a1bf-e4aa7cde1234', 'AccommodationAPI'),
        ('1ba42a09-45c0-40f8-a1bf-e4aa7cde2345', 'AccommodationAPI'),
        ('1ba42a09-45c0-40f8-a1bf-e4aa7cde3456', 'AccommodationAPI'),
        ('1ba42a09-45c0-40f8-a1bf-e4aa7cde4567', 'AccommodationAPI'),
        ('1ba42a09-45c0-40f8-a1bf-e4aa7cde1234', 'CountriesAPI'),
        ('1ba42a09-45c0-40f8-a1bf-e4aa7cde2345', 'CountriesAPI'),
        ('1ba42a09-45c0-40f8-a1bf-e4aa7cde4567', 'CountriesAPI')
    ON CONFLICT (\"API_ID\", \"POLICY_ID\") DO NOTHING;
    "
 }

process_files() {
    local DIR="$1"
    API_ID="$2"
    CONTENT_TYPE="$3"

    for file in "$DIR"/*; do
        if [ -f "$file" ]; then
            FILE_NAME=$(basename "$file")

            echo "Processing file: $FILE_NAME"

            # Import the file as a large object and get the OID
            OID=$(psql -U "$DB_USER" -d "$DB_NAME" -h "$DB_HOST" -p "$DB_PORT" -Atc "\lo_import '$file'" | awk '{print $NF}')
            if [ -z "$OID" ]; then
                echo "Error importing file: $FILE_NAME"
                continue
            fi
            echo "OID for $FILE_NAME: $OID"

            # Insert file details into the DP_API_CONTENT table
            TARGET_TYPE="$CONTENT_TYPE"
            if [[ "$FILE_NAME" == "apiDefinition.json" ]]; then
                TARGET_TYPE="API_DEFINITION"
            elif [[ "$FILE_NAME" == "apiDefinition.xml" ]]; then
                TARGET_TYPE="API_DEFINITION"
            elif [[ "$FILE_NAME" == "apiDefinition.graphql" ]]; then
                TARGET_TYPE="API_DEFINITION"
            elif [[ "$FILE_NAME" == "schemaDefinition.json" ]]; then
                TARGET_TYPE="SCHEMA_DEFINITION"
            fi

            INSERT_QUERY=$(cat <<EOF
INSERT INTO "DP_API_CONTENT" ("API_FILE", "FILE_NAME", "TYPE", "API_ID")
VALUES (lo_get($OID), '$FILE_NAME', '$TARGET_TYPE', '$API_ID')
ON CONFLICT ("API_ID", "FILE_NAME", "TYPE") DO NOTHING;
EOF
)

            psql -U "$DB_USER" -d "$DB_NAME" -h "$DB_HOST" -p "$DB_PORT" -c "$INSERT_QUERY"
            if [ $? -eq 0 ]; then
                echo "Successfully inserted: $FILE_NAME"
            else
                echo "Failed to insert: $FILE_NAME"
            fi
        fi
    done
}

insert_api_data "AccommodationAPI" "API for managing accommodations and reservations in a specified city" "v1.0" "REST" "https://api.example.com/production" "accommodation hotel lodging"
process_files "./artifacts/default/apiContent/AccommodationAPI/content" "AccommodationAPI" "MARKETING"
process_files "./artifacts/default/apiContent/AccommodationAPI/images" "AccommodationAPI" "IMAGE"
insert_image_data "AccommodationAPI" "accommodation.jpeg" "api-icon"
insert_image_data "AccommodationAPI" "transferImg.svg" "api-benefit-1"
insert_image_data "AccommodationAPI" "newPaymentImg.svg" "api-benefit-2"
insert_image_data "AccommodationAPI" "psd2Img.svg" "api-benefit-3"

insert_api_data "CountriesAPI" "API for retrieving information about countries, their capitals, population, and other geographical data." "v1.0" "GraphQL" "https://countries.trevorblades.com/graphql" "countries geography population"
process_files "./artifacts/default/apiContent/CountriesAPI/content" "CountriesAPI" "MARKETING"
process_files "./artifacts/default/apiContent/CountriesAPI/images" "CountriesAPI" "IMAGE"
insert_image_data "CountriesAPI" "countries.jpeg" "api-icon"

insert_api_data "LeisureActivitiesAPI" "API for retrieving information about leisure activities and managing reservations" "v1.0" "SOAP" "https://leisure.api.sandbox.abc.com" "leisure activities reservations"
process_files "./artifacts/default/apiContent/LeisureActivitiesAPI/content" "LeisureActivitiesAPI" "MARKETING"
process_files "./artifacts/default/apiContent/LeisureActivitiesAPI/images" "LeisureActivitiesAPI" "IMAGE"
insert_image_data "LeisureActivitiesAPI" "leisure.jpeg" "api-icon"

insert_api_data "NavigationAPI" "API for retrieving navigation data, including routes, GPS coordinates, traffic updates, and real-time directions." "v1.0" "WS" "https://sandbox.acme.com/api/v1/navigation" "navigation routes GPS"
process_files "./artifacts/default/apiContent/NavigationAPI/content" "NavigationAPI" "MARKETING"
process_files "./artifacts/default/apiContent/NavigationAPI/images" "NavigationAPI" "IMAGE"
insert_image_data "NavigationAPI" "navigation.jpeg" "api-icon"

insert_subscription_policy_data
insert_api_subscription_data
