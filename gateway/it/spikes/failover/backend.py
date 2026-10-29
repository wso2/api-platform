#!/usr/bin/env python3
"""Fake LLM provider. Echoes what it received so attempts can be inspected."""
import json, os, sys
from http.server import BaseHTTPRequestHandler, HTTPServer

NAME = os.environ.get("PROVIDER", "unknown")
ALWAYS_FAIL = os.environ.get("ALWAYS_FAIL", "false").lower() == "true"
FAIL_STATUS = int(os.environ.get("FAIL_STATUS", "500"))

class H(BaseHTTPRequestHandler):
    def _handle(self):
        length = int(self.headers.get("content-length") or 0)
        body = self.rfile.read(length).decode("utf-8") if length else ""
        print(f"HIT provider={NAME} fail={ALWAYS_FAIL} path={self.path} body={body[:120]}", flush=True)
        status = FAIL_STATUS if ALWAYS_FAIL else 200
        payload = json.dumps({
            "provider": NAME,
            "path": self.path,
            "received_body": body,
            "authorization": self.headers.get("authorization", ""),
        }).encode()
        self.send_response(status)
        self.send_header("content-type", "application/json")
        self.send_header("content-length", str(len(payload)))
        self.end_headers()
        self.wfile.write(payload)
    do_GET = do_POST = _handle
    def log_message(self, *a): pass

HTTPServer(("0.0.0.0", 8080), H).serve_forever()
