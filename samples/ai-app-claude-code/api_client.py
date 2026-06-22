"""
Reading List API client for Claude Code.

Reads API_BASE_URL and API_KEY from .claude/settings.json (written by setup.sh).

API contract:
  https://raw.githubusercontent.com/wso2/bijira-samples/refs/heads/main/reading-list-api/openapi.yaml

Usage:
    import api_client

    api_client.books_list()
    api_client.books_add("Clean Code", "Robert C. Martin", status="to_read")
    api_client.books_get("some-uuid")
    api_client.books_update("some-uuid", status="read")
    api_client.books_delete("some-uuid")

Book status values: "to_read" | "reading" | "read"
"""

from __future__ import annotations

import json
import sys
import urllib.request
import urllib.error
from pathlib import Path

# ---------------------------------------------------------------------------
# Load settings written by setup.sh
# ---------------------------------------------------------------------------

_SETTINGS_PATH = Path(".claude") / "settings.json"

if not _SETTINGS_PATH.exists():
    sys.exit(
        f"[api_client] {_SETTINGS_PATH} not found.\n"
        "Run ./setup.sh first to start the gateway and configure the API key."
    )

with _SETTINGS_PATH.open() as _f:
    _env = json.load(_f).get("env", {})

_BASE_URL = _env.get("API_BASE_URL", "").rstrip("/")
_API_KEY  = _env.get("API_KEY", "")

if not _BASE_URL or not _API_KEY:
    sys.exit(
        "[api_client] API_BASE_URL or API_KEY missing in .claude/settings.json.\n"
        "Run ./setup.sh first."
    )


# ---------------------------------------------------------------------------
# HTTP helper
# ---------------------------------------------------------------------------

def _request(method: str, path: str, body: dict | None = None) -> dict | list | None:
    url = f"{_BASE_URL}{path}"
    data = json.dumps(body).encode() if body is not None else None
    headers = {
        "X-API-Key": _API_KEY,
        "Content-Type": "application/json",
        "Accept": "application/json",
    }
    req = urllib.request.Request(url, data=data, headers=headers, method=method)
    try:
        with urllib.request.urlopen(req) as resp:
            raw = resp.read()
            return json.loads(raw) if raw else None
    except urllib.error.HTTPError as e:
        detail = e.read().decode(errors="replace")
        print(f"[api_client] HTTP {e.code} {method} {url}: {detail}", file=sys.stderr)
        raise


# ---------------------------------------------------------------------------
# Public API  —  mirrors the OpenAPI contract
# ---------------------------------------------------------------------------

def books_list() -> list[dict]:
    """Return all books in the reading list.

    Status values: "to_read" | "reading" | "read"
    """
    result = _request("GET", "/books")
    return result.get("books", []) if isinstance(result, dict) else result


def books_add(title: str, author: str, status: str = "to_read") -> dict:
    """Add a new book.

    Args:
        title:  Book title.
        author: Author name.
        status: Initial reading status — "to_read" (default), "reading", or "read".

    Returns:
        The created book dict including its ``id``.
    """
    return _request("POST", "/books", {"title": title, "author": author, "status": status})


def books_get(book_id: str) -> dict:
    """Fetch a single book by its UUID."""
    return _request("GET", f"/books/{book_id}")


def books_update(book_id: str, status: str) -> dict:
    """Update the reading status of a book.

    Args:
        book_id: UUID of the book to update.
        status:  New status — "to_read", "reading", or "read".

    Returns:
        The updated book dict.
    """
    return _request("PUT", f"/books/{book_id}", {"status": status})


def books_delete(book_id: str) -> dict:
    """Delete a book by its UUID. Returns {"id": "...", "note": "..."}."""
    return _request("DELETE", f"/books/{book_id}")


# ---------------------------------------------------------------------------
# Quick smoke-test when run directly
# ---------------------------------------------------------------------------

if __name__ == "__main__":
    import pprint
    print("Books in reading list:")
    pprint.pprint(books_list())
