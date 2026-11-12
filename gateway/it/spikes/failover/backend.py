#!/usr/bin/env python3
"""Fake LLM provider. Echoes what it received so attempts can be inspected.

Timing knobs exist to probe how Envoy's per-try timeouts classify a SLOW target as
opposed to a failing one, which is the distinction that decides whether failover engages:

  DELAY_BEFORE_HEADERS  seconds to stall before sending response headers. Models both a
                        slow time-to-first-token and a slow non-streaming completion --
                        from the proxy's side they are the same thing, a connection that
                        is open with nothing coming back.
  STREAM_CHUNKS         if > 0, send a chunked response of this many pieces instead of one
                        body, so a response can be in progress without being finished.
  CHUNK_GAP             seconds between those chunks. Set below per_try_idle_timeout to
                        model a target that is alive but degraded -- never idle long
                        enough to trip anything, yet arbitrarily slow overall.
"""
import json, os, sys, time
from http.server import BaseHTTPRequestHandler, HTTPServer

NAME = os.environ.get("PROVIDER", "unknown")
ALWAYS_FAIL = os.environ.get("ALWAYS_FAIL", "false").lower() == "true"
FAIL_STATUS = int(os.environ.get("FAIL_STATUS", "500"))
DELAY_BEFORE_HEADERS = float(os.environ.get("DELAY_BEFORE_HEADERS", "0"))
STREAM_CHUNKS = int(os.environ.get("STREAM_CHUNKS", "0"))
CHUNK_GAP = float(os.environ.get("CHUNK_GAP", "0"))


class H(BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"

    def _handle(self):
        length = int(self.headers.get("content-length") or 0)
        body = self.rfile.read(length).decode("utf-8") if length else ""
        t0 = time.time()
        print(f"HIT provider={NAME} fail={ALWAYS_FAIL} path={self.path} "
              f"delay={DELAY_BEFORE_HEADERS} chunks={STREAM_CHUNKS} gap={CHUNK_GAP} "
              f"body={body[:80]}", flush=True)

        if DELAY_BEFORE_HEADERS > 0:
            # Nothing is written during this window, so the connection is idle from
            # Envoy's point of view while the request itself is still in flight.
            time.sleep(DELAY_BEFORE_HEADERS)

        status = FAIL_STATUS if ALWAYS_FAIL else 200

        if STREAM_CHUNKS > 0:
            try:
                self.send_response(status)
                self.send_header("content-type", "text/event-stream")
                self.send_header("transfer-encoding", "chunked")
                self.end_headers()
                for i in range(STREAM_CHUNKS):
                    piece = json.dumps({"provider": NAME, "chunk": i}).encode()
                    self.wfile.write(b"%x\r\n%s\r\n" % (len(piece), piece))
                    self.wfile.flush()
                    if CHUNK_GAP > 0 and i < STREAM_CHUNKS - 1:
                        time.sleep(CHUNK_GAP)
                self.wfile.write(b"0\r\n\r\n")
                self.wfile.flush()
            except (BrokenPipeError, ConnectionResetError):
                # Envoy cancelled the attempt mid-stream; that is a result, not an error.
                print(f"CANCELLED provider={NAME} after {time.time()-t0:.1f}s", flush=True)
            return

        payload = json.dumps({
            "provider": NAME,
            "path": self.path,
            "received_body": body,
            "authorization": self.headers.get("authorization", ""),
            "served_after_s": round(time.time() - t0, 1),
        }).encode()
        try:
            self.send_response(status)
            self.send_header("content-type", "application/json")
            self.send_header("content-length", str(len(payload)))
            self.end_headers()
            self.wfile.write(payload)
        except (BrokenPipeError, ConnectionResetError):
            print(f"CANCELLED provider={NAME} after {time.time()-t0:.1f}s", flush=True)

    do_GET = do_POST = _handle

    def log_message(self, *a):
        pass


HTTPServer(("0.0.0.0", 8080), H).serve_forever()
