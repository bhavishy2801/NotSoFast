"""Thin standard-library client. Decisions and publication remain in the Go service."""
import json
import urllib.error
import urllib.request


class APIError(Exception):
    def __init__(self, reason, status=0):
        super().__init__(reason)
        self.reason, self.status = reason, status


class Client:
    def __init__(self, url="http://127.0.0.1:8787", token="", timeout=65):
        self.url, self.token, self.timeout = url.rstrip("/"), token, timeout
        self.calls = self.bytes_returned = 0
        self.opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))

    def call(self, action, **request):
        if action not in OPERATIONS:
            raise ValueError("unknown operation")
        payload = json.dumps(request, separators=(",", ":")).encode()
        req = urllib.request.Request(self.url + "/v1/" + action, data=payload,
                                     headers={"Authorization": "Bearer " + self.token,
                                              "Content-Type": "application/json"})
        self.calls += 1
        try:
            with self.opener.open(req, timeout=self.timeout) as response:
                body = response.read((32 << 20) + 1)
                if len(body) > 32 << 20:
                    raise APIError("RESPONSE_LIMIT")
                self.bytes_returned += len(body)
                return json.loads(body)["result"]
        except urllib.error.HTTPError as exc:
            body = exc.read(65536)
            self.bytes_returned += len(body)
            try:
                reason = json.loads(body)["error"]["reason"]
            except (ValueError, KeyError, TypeError):
                reason = "HTTP_ERROR"
            raise APIError(reason, exc.code) from None


OPERATIONS = ("register", "head", "snapshot", "search", "receipt", "compose", "verify",
              "search_missing", "refresh", "guarded_create", "operation")
