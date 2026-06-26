import http.server
import socketserver
import os, signal, sys

class Handler(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        self._respond(200, b"relay-target-ok\n")
    def do_POST(self):
        length = int(self.headers.get("Content-Length", 0))
        body = self.rfile.read(length) if length > 0 else b""
        sys.stderr.write("POST body: " + body.decode() + "\n")
        sys.stderr.flush()
        self._respond(200, b"relay-target-post-ok\n")
    def _respond(self, code, body):
        self.send_response(code)
        self.send_header("Content-Type", "text/plain")
        self.end_headers()
        self.wfile.write(body)

# Allow reuse
socketserver.TCPServer.allow_reuse_address = True
server = socketserver.TCPServer(("127.0.0.1", 2724), Handler)
print("Test server running on 127.0.0.1:2724", flush=True)
server.serve_forever()
