#!/usr/bin/env python3
"""Mock LLM backend for exercising the gateway fault flows.

Standard library only, so it runs on a bare python:3-alpine image.

The scenario is picked per request, by the `x-mock-scenario` request header (the gateway forwards
it upstream) or, failing that, a `mock` query parameter. With neither, the backend answers like a
healthy OpenAI chat-completions endpoint.

  ok              200 OpenAI chat completion (default)
  long            200 chat completion whose content is long (trips a response guardrail)
  echo            200 echoes method, path, headers and body as JSON
  openai-429      429 already in the OpenAI error envelope
  anthropic-529   529 in Anthropic's error shape
  vendor-418      418 a vendor's own JSON
  plain-500       500 plain text
  html-502        502 an HTML page
  empty-429       429 with no body
  empty-500       500 with no body
  status-<code>   <code> with a small JSON body, e.g. status-503
  slow            sleeps `x-mock-delay` seconds (default 5) then answers 200
  reset           closes the connection without answering (the router reports the reset)

Every response carries `x-mock-backend: true`, so a test can tell the backend's own response
from one the gateway produced.
"""

import json
import os
import socket
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import parse_qs, urlparse

PORT = int(os.environ.get("MOCK_PORT", "8000"))

CHAT_COMPLETION = {
    "id": "chatcmpl-mock-1",
    "object": "chat.completion",
    "created": 1760000000,
    "model": "gpt-4o",
    "choices": [
        {
            "index": 0,
            "message": {"role": "assistant", "content": "Hello from the mock backend."},
            "finish_reason": "stop",
        }
    ],
    "usage": {"prompt_tokens": 5, "completion_tokens": 6, "total_tokens": 11},
}

JSON = "application/json"


def chat(content=None):
    body = json.loads(json.dumps(CHAT_COMPLETION))
    if content is not None:
        body["choices"][0]["message"]["content"] = content
    return json.dumps(body)


FIXED = {
    "ok": (200, JSON, lambda: chat()),
    "long": (200, JSON, lambda: chat(" ".join(["word"] * 60))),
    "openai-429": (
        429,
        JSON,
        lambda: json.dumps(
            {
                "error": {
                    "message": "Rate limit reached for gpt-4o in organization org-mock.",
                    "type": "requests",
                    "param": None,
                    "code": "rate_limit_exceeded",
                }
            }
        ),
    ),
    "anthropic-529": (
        529,
        JSON,
        lambda: json.dumps(
            {"type": "error", "error": {"type": "overloaded_error", "message": "Overloaded"}}
        ),
    ),
    "vendor-418": (418, JSON, lambda: json.dumps({"vendor_error": "I am a teapot"})),
    "plain-500": (500, "text/plain", lambda: "Internal error inside the mock backend"),
    "html-502": (
        502,
        "text/html",
        lambda: "<html><body><h1>502 Bad Gateway</h1><p>mock</p></body></html>",
    ),
    "empty-429": (429, None, None),
    "empty-500": (500, None, None),
}


class Handler(BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"
    server_version = "mock-llm-backend"

    def log_message(self, fmt, *args):
        print("%s %s" % (self.headers.get("x-mock-scenario", "-"), fmt % args), flush=True)

    def _scenario(self):
        q = parse_qs(urlparse(self.path).query)
        return (self.headers.get("x-mock-scenario") or q.get("mock", ["ok"])[0]).strip().lower()

    def _read_body(self):
        length = int(self.headers.get("content-length") or 0)
        return self.rfile.read(length) if length > 0 else b""

    def _send(self, status, content_type, body):
        data = body.encode() if isinstance(body, str) else (body or b"")
        self.send_response(status)
        self.send_header("x-mock-backend", "true")
        if content_type:
            self.send_header("content-type", content_type)
        self.send_header("content-length", str(len(data)))
        self.end_headers()
        if self.command != "HEAD" and data:
            self.wfile.write(data)

    def _handle(self):
        body = self._read_body()
        scenario = self._scenario()

        if scenario == "reset":
            # SO_LINGER 0 turns close() into a TCP RST: the router sees a reset before any header.
            self.connection.setsockopt(
                socket.SOL_SOCKET, socket.SO_LINGER, b"\x01\x00\x00\x00\x00\x00\x00\x00"
            )
            self.close_connection = True
            self.connection.close()
            return

        if scenario == "slow":
            time.sleep(float(self.headers.get("x-mock-delay", "5")))
            return self._send(200, JSON, chat("Slow hello."))

        if scenario == "echo":
            return self._send(
                200,
                JSON,
                json.dumps(
                    {
                        "method": self.command,
                        "path": self.path,
                        "headers": {k.lower(): v for k, v in self.headers.items()},
                        "body": body.decode(errors="replace"),
                    }
                ),
            )

        if scenario.startswith("status-"):
            try:
                code = int(scenario.split("-", 1)[1])
            except ValueError:
                code = 400
            return self._send(code, JSON, json.dumps({"mock_status": code}))

        status, content_type, make = FIXED.get(scenario, FIXED["ok"])
        return self._send(status, content_type, make() if make else None)

    do_GET = do_POST = do_PUT = do_PATCH = do_DELETE = do_HEAD = _handle


if __name__ == "__main__":
    print("mock LLM backend listening on :%d" % PORT, flush=True)
    ThreadingHTTPServer(("0.0.0.0", PORT), Handler).serve_forever()
