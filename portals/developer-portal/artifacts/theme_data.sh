#!/bin/bash
set -euo pipefail

CONFIG_FILE="./config.json"

# Read database configuration from config.json
DB_HOST=$(grep -A 6 '"db"' "$CONFIG_FILE" | grep '"host"' | sed 's/.*"host": "\(.*\)".*/\1/')
DB_PORT=$(grep -A 6 '"db"' "$CONFIG_FILE" | grep '"port"' | sed 's/.*"port": \(.*\),.*/\1/')
DB_NAME=$(grep -A 6 '"db"' "$CONFIG_FILE" | grep '"database"' | sed 's/.*"database": "\(.*\)".*/\1/')
DB_USER=$(grep -A 6 '"db"' "$CONFIG_FILE" | grep '"username"' | sed 's/.*"username": "\(.*\)".*/\1/')

ORG_NAME="${1:-ACME}"
ORG_ID="${2:-1ba42a09-45c0-40f8-a1bf-e4aa7cde1575}"
VIEW_NAME="${3:-default}"
THEME_ZIP_PATH="${4:-}"

ORG_DIR="./artifacts/default/orgContent/${ORG_NAME}"
BASE_DIR="./src/defaultContent"
ENABLE_ORG_OVERLAY="${ENABLE_ORG_OVERLAY:-false}"
RESET_EXISTING="${RESET_EXISTING:-true}"

if [ ! -d "$BASE_DIR" ]; then
  echo "Error: base content directory not found: $BASE_DIR"
  exit 1
fi

# Get the view ID
VIEW_ID=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -Atc "SELECT \"VIEW_ID\" FROM \"DP_VIEWS\" WHERE \"ORG_ID\"='${ORG_ID}' AND \"NAME\"='${VIEW_NAME}' LIMIT 1;")

if [ -z "$VIEW_ID" ]; then
  echo "Error: View not found: org=$ORG_NAME orgId=$ORG_ID viewName=$VIEW_NAME"
  echo "Run ./artifacts/org_data.sh first (it creates the default view)."
  exit 1
fi

echo "Loading base content from: $BASE_DIR"
if [ "$ENABLE_ORG_OVERLAY" = "true" ]; then
  if [ -d "$ORG_DIR" ]; then
    echo "Loading org overlay from: $ORG_DIR (ENABLE_ORG_OVERLAY=true)"
  else
    echo "Org overlay not found (ok): $ORG_DIR"
  fi
else
  echo "Org overlay disabled (ENABLE_ORG_OVERLAY=false)."
fi
if [ -n "$THEME_ZIP_PATH" ]; then
  echo "Loading theme zip overlay: $THEME_ZIP_PATH"
fi
echo "Target org=$ORG_NAME orgId=$ORG_ID view=$VIEW_NAME viewId=$VIEW_ID"

if [ "$RESET_EXISTING" = "true" ]; then
  echo "Resetting existing theme assets in DB (RESET_EXISTING=true)..."
  psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -v ON_ERROR_STOP=1 -c "
    DELETE FROM \"DP_ORGANIZATION_ASSETS\"
    WHERE \"ORG_ID\"='${ORG_ID}'
      AND \"VIEW_ID\"='${VIEW_ID}'
      AND \"FILE_TYPE\" IN ('style','image','layout','partial','template','markDown');
  " >/dev/null
else
  echo "Keeping existing theme assets in DB (RESET_EXISTING=false)."
fi

relpath() {
  python3 -c "import os,sys; print(os.path.relpath(sys.argv[1], sys.argv[2]))" "$1" "$2"
}

sql_escape() {
  # Escape single quotes for SQL string literals
  # shellcheck disable=SC2001
  echo "$1" | sed "s/'/''/g"
}

file_type_for() {
  local rel="$1"
  local name="$2"

  if [[ "$name" == *.css ]]; then
    echo "style"
    return
  fi

  if [[ "$name" == *.md ]]; then
    echo "markDown"
    return
  fi

  if [[ "$name" == *.hbs ]]; then
    # classify based on folder
    if [[ "$rel" == layout/* ]]; then
      echo "layout"
      return
    fi
    # any nested "partials/" folder should be treated as a partial
    if [[ "$rel" == partials/* || "$rel" == */partials/* ]]; then
      echo "partial"
      return
    fi
    if [[ "$rel" == pages/* ]]; then
      echo "template"
      return
    fi
    # fallback
    echo "partial"
    return
  fi

  # images + everything else
  echo "image"
}

upsert_asset() {
  local abs_file="$1"
  local rel="$2"

  local file_name
  file_name=$(basename "$abs_file")
  local file_name_sql
  file_name_sql=$(sql_escape "$file_name")

  local file_path
  file_path=$(dirname "$rel")
  if [ "$file_path" = "." ]; then
    file_path=""
  fi
  local file_path_sql
  file_path_sql=$(sql_escape "$file_path")

  local file_type
  file_type=$(file_type_for "$rel" "$file_name")
  local file_type_sql
  file_type_sql=$(sql_escape "$file_type")

  # Import file as large object and read it back as bytea
  local oid
  # Escape single quotes in file path for the psql meta-command.
  local abs_file_escaped
  abs_file_escaped=$(sql_escape "$abs_file")
  oid=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -Atc "\\lo_import '${abs_file_escaped}'" | awk '{print $NF}')

  if [ -z "$oid" ]; then
    echo "Failed to import file as large object: $abs_file"
    return 1
  fi

  local insert_sql
  insert_sql=$(cat <<EOF
INSERT INTO "DP_ORGANIZATION_ASSETS" ("ASSET_ID", "FILE_CONTENT", "FILE_NAME", "FILE_TYPE", "FILE_PATH", "ORG_ID", "VIEW_ID")
VALUES (gen_random_uuid(), lo_get(${oid}), '${file_name_sql}', '${file_type_sql}', '${file_path_sql}', '${ORG_ID}', '${VIEW_ID}')
ON CONFLICT ("FILE_NAME", "FILE_TYPE", "FILE_PATH", "ORG_ID", "VIEW_ID") DO UPDATE
SET "FILE_CONTENT" = lo_get(${oid});
EOF
)

  psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -v ON_ERROR_STOP=1 -c "$insert_sql" >/dev/null

  # Cleanup large object to avoid LO bloat
  psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -Atc "SELECT lo_unlink(${oid});" >/dev/null || true

  echo "Upserted: type=$file_type path='${file_path}' file='${file_name}'"
}

process_root() {
  local root="$1"
  local strip_prefix="${2:-}"
  local label="${3:-$root}"

  echo "Processing: $label"
  while IFS= read -r -d '' file; do
    rel=$(relpath "$file" "$root")
    if [ -n "$strip_prefix" ]; then
      rel="${rel#"$strip_prefix"}"
    fi
    # Only support the standard theme folders
    case "$rel" in
      layout/*|pages/*|partials/*|styles/*|images/*)
        upsert_asset "$file" "$rel"
        ;;
      *)
        # ignore anything else (e.g. __MACOSX, theme/ root marker, etc.)
        ;;
    esac
  done < <(find "$root" -type f -print0)
}

# 1) base pack
process_root "$BASE_DIR" "" "base: ./src/defaultContent"

# 2) org overlay pack (optional)
if [ "$ENABLE_ORG_OVERLAY" = "true" ] && [ -d "$ORG_DIR" ]; then
  process_root "$ORG_DIR" "" "org overlay: $ORG_DIR"
fi

# 3) theme zip overlay (optional)
if [ -n "$THEME_ZIP_PATH" ]; then
  if [ ! -f "$THEME_ZIP_PATH" ]; then
    echo "Error: theme zip not found: $THEME_ZIP_PATH"
    exit 1
  fi

  TMP_DIR="$(mktemp -d)"
  trap 'rm -rf "$TMP_DIR"' EXIT

  # Extract using python (works even if unzip isn't installed)
  python3 - <<'PY' "$THEME_ZIP_PATH" "$TMP_DIR"
import sys, zipfile
zip_path, out_dir = sys.argv[1], sys.argv[2]
with zipfile.ZipFile(zip_path) as z:
    z.extractall(out_dir)
PY

  # Most uploads contain a top-level "theme/" folder
  if [ -d "$TMP_DIR/theme" ]; then
    process_root "$TMP_DIR" "theme/" "zip overlay: $THEME_ZIP_PATH (theme/...)"
  else
    process_root "$TMP_DIR" "" "zip overlay: $THEME_ZIP_PATH"
  fi
fi

echo "Theme data insertion completed!"
