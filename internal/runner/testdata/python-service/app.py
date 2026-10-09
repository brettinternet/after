from http.server import BaseHTTPRequestHandler, HTTPServer
from urllib.request import Request, urlopen
import os
import sys

clock_url, provider_url = sys.argv[1:3]


class Handler(BaseHTTPRequestHandler):
    def do_POST(self):
        if self.path != "/echo":
            self.send_error(404)
            return
        length = int(self.headers.get("Content-Length", "0"))
        body = self.rfile.read(length)
        key = self.headers.get("Idempotency-Key", "")
        try:
            with urlopen(clock_url, timeout=2) as clock:
                clock.read(32)
            request = Request(
                provider_url + "/record",
                data=body,
                headers={"Idempotency-Key": key, "Content-Type": "text/plain"},
                method="POST",
            )
            with urlopen(request, timeout=2) as response:
                result = response.read(4097)
                status = response.status
        except Exception:
            self.send_error(502)
            return
        self.send_response(status)
        self.send_header("Content-Length", str(len(result)))
        self.end_headers()
        self.wfile.write(result)

    def log_message(self, *_args):
        return


server = HTTPServer(("127.0.0.1", 8080), Handler)
with os.fdopen(3, "w") as readiness:
    readiness.write("http://127.0.0.1:8080\n")
    readiness.flush()
server.serve_forever()
