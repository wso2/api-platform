#!/usr/bin/env bash
#
# Splits a monolithic Markdown/HTML API doc into multiple files,
# one per top-level heading (<h1> or #), plus an README.md with a ToC.
#
# Schema anchor references in non-schema files are rewritten so
# (#schemaXXX) and (#tocS_XXX) point to the schemas file.
#
# Usage: ./split-openapi.sh [input-file] [output-dir]
#   Defaults: input-file=openapi.md  output-dir=docs

set -euo pipefail

INPUT="${1:-openapi.md}"
OUTDIR="${2:-docs}"

# Files that will never be overwritten if they already exist in OUTDIR
PROTECTED_FILES=("authentication.md")

if [[ ! -f "$INPUT" ]]; then
  echo "Error: $INPUT not found" >&2
  exit 1
fi

mkdir -p "$OUTDIR"

total_lines=$(wc -l < "$INPUT")

# --- Find all top-level heading line numbers ---
# Matches: <h1 ...>Title</h1>  or  # Title
h1_lines=()
h1_titles=()

while IFS=: read -r lineno rest; do
  h1_lines+=("$lineno")
  # Extract title text
  re_h1='<h1[^>]*>(.*)</h1>'
  re_md='^# (.*)'
  if [[ "$rest" =~ $re_h1 ]]; then
    h1_titles+=("${BASH_REMATCH[1]}")
  elif [[ "$rest" =~ $re_md ]]; then
    h1_titles+=("${BASH_REMATCH[1]}")
  else
    h1_titles+=("Section")
  fi
done < <(grep -n '^<h1 \|^# ' "$INPUT")

num_sections=${#h1_lines[@]}

if [[ $num_sections -eq 0 ]]; then
  echo "Error: no top-level headings found in $INPUT" >&2
  exit 1
fi

echo "Found $num_sections top-level section(s):"

# --- Helper: slugify a title into a filename ---
slugify() {
  echo "$1" | tr '[:upper:]' '[:lower:]' \
    | sed -E 's/[^a-z0-9]+/-/g' \
    | sed -E 's/^-+|-+$//g'
}

# --- Split into sections ---
# First section (before the first h1, if any content) + first h1 = intro (goes into README.md)
# Remaining h1 sections = separate files

declare -a section_files=()
declare -a section_titles=()
schemas_file=""

for ((i = 0; i < num_sections; i++)); do
  start=${h1_lines[$i]}
  if [[ $i -lt $((num_sections - 1)) ]]; then
    end=$((${h1_lines[$((i + 1))]} - 1))
  else
    end=$total_lines
  fi

  title="${h1_titles[$i]}"
  echo "  [$start-$end] $title"

  if [[ $i -eq 0 ]]; then
    # First section is the intro — will be embedded in README.md
    # Include any content before the first h1 as well
    intro_content=$(sed -n "1,${end}p" "$INPUT")
    section_files+=("__intro__")
    section_titles+=("$title")
  else
    slug=$(slugify "$title")
    filename="${slug}.md"
    section_content=$(sed -n "${start},${end}p" "$INPUT")

    # Detect if this is the schemas section (title contains "schema" case-insensitive)
    if echo "$title" | grep -qi "schema"; then
      schemas_file="$filename"
    fi

    # Skip protected files that already exist
    is_protected=false
    for protected in "${PROTECTED_FILES[@]}"; do
      if [[ "$filename" == "$protected" && -f "$OUTDIR/$filename" ]]; then
        echo "  Skipping protected file: $filename"
        is_protected=true
        break
      fi
    done
    [[ "$is_protected" == true ]] && { section_files+=("$filename"); section_titles+=("$title"); continue; }

    echo "$section_content" > "$OUTDIR/$filename"
    section_files+=("$filename")
    section_titles+=("$title")
  fi
done

# --- Rewrite schema refs in non-schema files ---
if [[ -n "$schemas_file" ]]; then
  echo ""
  echo "Schemas file: $schemas_file"
  echo "Rewriting schema references in other files..."

  for ((i = 1; i < num_sections; i++)); do
    f="${section_files[$i]}"
    [[ "$f" == "$schemas_file" ]] && continue

    filepath="$OUTDIR/$f"
    # (#schemaXXX) -> (schemas_file#schemaXXX)
    # macOS sed requires an explicit backup suffix with -i; use .bak then remove (portable with GNU sed).
    sed -i.bak -E "s|\(#(schema[a-zA-Z0-9_]+)\)|(${schemas_file}#\1)|g" "$filepath" && rm -f "${filepath}.bak"
    # (#tocS_XXX) -> (schemas_file#tocS_XXX)
    sed -i.bak -E "s|\(#(tocS_[a-zA-Z0-9_]+)\)|(${schemas_file}#\1)|g" "$filepath" && rm -f "${filepath}.bak"
  done
fi

# --- Build README.md ---
{
  echo "$intro_content"
  echo ""
  echo "## Table of Contents"
  echo ""

  for ((i = 1; i < num_sections; i++)); do
    f="${section_files[$i]}"
    t="${section_titles[$i]}"
    echo "### [$t]($f)"
    echo ""

    # Add sub-headings (##) as a bullet list
    subheadings=$(grep '^## ' "$OUTDIR/$f" 2>/dev/null || true)
    if [[ -n "$subheadings" ]]; then
      while IFS= read -r line; do
        heading_text=$(echo "$line" | sed 's/^## //')
        anchor=$(echo "$heading_text" | tr '[:upper:]' '[:lower:]' | sed -E 's/[^a-z0-9 -]//g' | sed -E 's/ /-/g; s/^-+|-+$//g')
        echo "- [$heading_text](${f}#${anchor})"
      done <<< "$subheadings"
      echo ""
    fi
  done
} > "$OUTDIR/README.md"

# Also rewrite schema refs in README.md
if [[ -n "$schemas_file" ]]; then
  readme_path="$OUTDIR/README.md"
  sed -i.bak -E "s|\(#(schema[a-zA-Z0-9_]+)\)|(${schemas_file}#\1)|g" "$readme_path" && rm -f "${readme_path}.bak"
  sed -i.bak -E "s|\(#(tocS_[a-zA-Z0-9_]+)\)|(${schemas_file}#\1)|g" "$readme_path" && rm -f "${readme_path}.bak"
fi

echo ""
echo "Done! Files written to $OUTDIR/:"
ls -1 "$OUTDIR/"
