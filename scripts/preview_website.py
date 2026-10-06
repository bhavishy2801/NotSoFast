"""Local static preview with byte-range support for the scrubbed MP4."""
import argparse
import functools
import http.server
import re
from pathlib import Path


class RangeHandler(http.server.SimpleHTTPRequestHandler):
    def send_head(self):
        self.remaining = None
        header = self.headers.get('Range')
        path = Path(self.translate_path(self.path))
        if not header or not path.is_file():
            return super().send_head()
        size = path.stat().st_size
        match = re.fullmatch(r'bytes=(\d*)-(\d*)', header)
        if not match or not any(match.groups()):
            self.send_error(416)
            return None
        left, right = match.groups()
        start = int(left) if left else max(0, size-int(right))
        end = min(size-1, int(right)) if left and right else size-1
        if start > end or start >= size:
            self.send_response(416)
            self.send_header('Content-Range', f'bytes */{size}')
            self.send_header('Content-Length', '0')
            self.end_headers()
            return None
        stream = path.open('rb')
        stream.seek(start)
        self.remaining = end-start+1
        self.send_response(206)
        self.send_header('Content-Type', self.guess_type(str(path)))
        self.send_header('Content-Range', f'bytes {start}-{end}/{size}')
        self.send_header('Content-Length', str(self.remaining))
        self.end_headers()
        return stream

    def end_headers(self):
        self.send_header('Accept-Ranges', 'bytes')
        super().end_headers()

    def copyfile(self, source, outputfile):
        try:
            if self.remaining is None:
                return super().copyfile(source, outputfile)
            while self.remaining:
                chunk = source.read(min(65536, self.remaining))
                if not chunk:break
                outputfile.write(chunk)
                self.remaining -= len(chunk)
        except ConnectionError:
            pass  # Browsers cancel old range requests when scrubbing or navigating.


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('--port', type=int, default=4173)
    args = parser.parse_args()
    directory = Path(__file__).resolve().parents[1]/'website'
    http.server.ThreadingHTTPServer(('127.0.0.1', args.port), functools.partial(RangeHandler, directory=str(directory))).serve_forever()
