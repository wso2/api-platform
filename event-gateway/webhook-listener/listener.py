import argparse
import json
import sys
import termios
import tty
import threading
from http.server import BaseHTTPRequestHandler, HTTPServer
from urllib.parse import urlparse, parse_qs


# Shared state
listening = True
state_lock = threading.Lock()


class WebhookHandler(BaseHTTPRequestHandler):
    def do_POST(self):
        with state_lock:
            active = listening

        if not active:
            self.send_response(503)
            self.send_header("Content-Type", "text/plain")
            self.end_headers()
            self.wfile.write(b"Listener is paused.\n")
            return

        content_length = int(self.headers.get("Content-Length", 0))
        post_data = self.rfile.read(content_length)

        print(f"\n--- New Webhook Received on {self.path} ---")
        print("Headers:")
        for key, value in self.headers.items():
            print(f"  {key}: {value}")

        print("\nBody:")
        try:
            parsed_data = json.loads(post_data.decode("utf-8"))
            print(json.dumps(parsed_data, indent=4))
        except json.JSONDecodeError:
            try:
                print(post_data.decode("utf-8"))
            except UnicodeDecodeError:
                print(post_data)
        print("-------------------------------------------\n")

        self.send_response(200)
        self.send_header("Content-Type", "text/plain")
        self.end_headers()
        self.wfile.write(b"Webhook received successfully!\n")

    def do_GET(self):
        with state_lock:
            active = listening

        if not active:
            self.send_response(503)
            self.send_header("Content-Type", "text/plain")
            self.end_headers()
            self.wfile.write(b"Listener is paused.\n")
            return

        parsed = urlparse(self.path)
        query = parse_qs(parsed.query)
        challenge = query.get("hub.challenge", [None])[0]

        print(f"\n--- Verification / GET Request on {self.path} ---")
        print("Headers:")
        for key, value in self.headers.items():
            print(f"  {key}: {value}")
        print("-------------------------------------------\n")

        self.send_response(200)
        self.send_header("Content-Type", "text/plain")
        self.end_headers()

        if challenge is not None:
            self.wfile.write(challenge.encode("utf-8"))
        else:
            self.wfile.write(b"Listener is active. Awaiting requests...\n")

    def log_message(self, format, *args):
        # Suppress default access log to keep output clean
        pass


def getch():
    """Read a single character from stdin without echoing."""
    fd = sys.stdin.fileno()
    old = termios.tcgetattr(fd)
    try:
        tty.setraw(fd)
        return sys.stdin.read(1)
    finally:
        termios.tcsetattr(fd, termios.TCSADRAIN, old)


def keyboard_control():
    global listening
    print("Press [SPACE] to toggle listening on/off. Press [q] to quit.\n")
    while True:
        ch = getch()
        if ch == " ":
            with state_lock:
                listening = not listening
                state = "ACTIVE" if listening else "PAUSED"
            print(f"\n[Toggle] Listener is now {state}. Press [SPACE] to toggle, [q] to quit.\n")
        elif ch in ("q", "Q", "\x03"):  # q, Q, or Ctrl+C
            print("\nShutting down listener.")
            sys.exit(0)


def run(port):
    server = HTTPServer(("", port), WebhookHandler)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    print(f"Webhook listener started on http://0.0.0.0:{port}")
    print("Status: ACTIVE")
    keyboard_control()


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Simple Terminal Webhook Listener")
    parser.add_argument(
        "-p", "--port",
        type=int,
        default=8090,
        help="Port to listen on (default: 8090)",
    )
    args = parser.parse_args()
    run(args.port)
