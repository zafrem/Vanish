import os
import tempfile
import unittest

from check_site import check

GOOD_HTML = """<!doctype html>
<html lang="en"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Vanish</title>
<meta name="description" content="d">
<link rel="canonical" href="https://vanish.codeforlife.dev/">
<meta property="og:title" content="t">
<link rel="stylesheet" href="styles.css">
</head><body>
<a href="#top">top</a><h1 id="top">Hi</h1>
<img src="assets/a.png" alt="A thing">
<a href="https://github.com/zafrem/Vanish">repo</a>
</body></html>"""


def make_site(index=GOOD_HTML, cname="vanish.codeforlife.dev\n", extra=None):
    d = tempfile.mkdtemp()
    os.makedirs(os.path.join(d, "assets"))
    files = {"index.html": index, "styles.css": "body{}", "CNAME": cname, "assets/a.png": "x"}
    files.update(extra or {})
    for name, content in files.items():
        with open(os.path.join(d, name), "w", encoding="utf-8") as f:
            f.write(content)
    return d


class CheckSiteTest(unittest.TestCase):
    def test_good_site_passes(self):
        self.assertEqual(check(make_site()), [])

    def test_missing_files(self):
        d = tempfile.mkdtemp()
        problems = check(d)
        self.assertTrue(any("index.html" in p for p in problems))

    def test_cname_must_match(self):
        self.assertTrue(any("CNAME" in p for p in check(make_site(cname="other.example\n"))))

    def test_third_party_resource_rejected(self):
        bad = GOOD_HTML.replace("</head>", '<script src="https://cdn.example.com/x.js"></script></head>')
        self.assertTrue(any("third-party" in p for p in check(make_site(index=bad))))

    def test_external_stylesheet_and_font_rejected(self):
        bad = GOOD_HTML.replace("</head>", '<link rel="stylesheet" href="https://fonts.googleapis.com/x"></head>')
        self.assertTrue(any("third-party" in p for p in check(make_site(index=bad))))

    def test_css_remote_url_rejected(self):
        d = make_site(extra={"styles.css": "@import url(https://x.example/a.css);"})
        self.assertTrue(any("third-party" in p for p in check(d)))

    def test_unknown_external_link_rejected(self):
        bad = GOOD_HTML.replace("</body>", '<a href="https://evil.example/">x</a></body>')
        self.assertTrue(any("external link" in p for p in check(make_site(index=bad))))

    def test_buymeacoffee_link_allowed_but_remote_badge_image_rejected(self):
        link = GOOD_HTML.replace("</body>", '<a href="https://buymeacoffee.com/zafrem">c</a></body>')
        self.assertEqual(check(make_site(index=link)), [])
        remote = GOOD_HTML.replace("</body>", '<img src="https://img.shields.io/badge/x" alt="b"></body>')
        self.assertTrue(any("third-party" in p for p in check(make_site(index=remote))))

    def test_missing_local_file_rejected(self):
        bad = GOOD_HTML.replace("assets/a.png", "assets/missing.png")
        self.assertTrue(any("missing.png" in p for p in check(make_site(index=bad))))

    def test_broken_anchor_rejected(self):
        bad = GOOD_HTML.replace('href="#top"', 'href="#nowhere"')
        self.assertTrue(any("#nowhere" in p for p in check(make_site(index=bad))))

    def test_img_without_alt_rejected(self):
        bad = GOOD_HTML.replace(' alt="A thing"', "")
        self.assertTrue(any("alt" in p for p in check(make_site(index=bad))))

    def test_missing_metadata_rejected(self):
        bad = GOOD_HTML.replace('<meta name="description" content="d">', "")
        self.assertTrue(any("description" in p for p in check(make_site(index=bad))))

    def test_banned_claim_rejected(self):
        bad = GOOD_HTML.replace("<h1", "<p>The server can never decrypt your data.</p><h1")
        self.assertTrue(any("banned" in p for p in check(make_site(index=bad))))

    def test_size_budget(self):
        d = make_site(extra={"assets/big.bin": "x" * (1_600_000)})
        self.assertTrue(any("size" in p for p in check(d)))


if __name__ == "__main__":
    unittest.main()
