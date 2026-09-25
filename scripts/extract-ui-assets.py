"""Extract icons and phone CSS from docs/visual/screens.html for web spike."""
from __future__ import annotations

import re
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
HTML = ROOT / "docs" / "visual" / "screens.html"
html = HTML.read_text(encoding="utf-8")

# 22 interface icons (i-*), without mark/logo
symbols = re.findall(r'<symbol id="i-[^"]+"[^>]*>.*?</symbol>', html, re.S)
icons_path = ROOT / "web" / "static" / "icons.svg"
icons_path.parent.mkdir(parents=True, exist_ok=True)
icons_path.write_text(
    '<svg xmlns="http://www.w3.org/2000/svg" width="0" height="0" '
    'style="position:absolute" aria-hidden="true">\n'
    + "\n".join(symbols)
    + "\n</svg>\n",
    encoding="utf-8",
)
print(f"wrote {icons_path} ({len(symbols)} symbols)")

root_match = re.search(r":root \{[^}]+\}", html)
assert root_match
root = root_match.group(0)

tokens_path = ROOT / "web" / "src" / "lib" / "styles" / "tokens.css"
tokens_path.parent.mkdir(parents=True, exist_ok=True)
tokens_path.write_text(
    root
    + """

.ph.dark {
  --paper: #211E1C;
  --card: #2A2624;
  --ink: #EFE9E0;
  --muted: #9A9188;
  --faint: #6E665E;
  --line: #3A3532;
}
""",
    encoding="utf-8",
)

phone_css = re.search(r"/\* телефон \*/(.*?)\n  @media", html, re.S)
assert phone_css
ui_path = ROOT / "web" / "src" / "lib" / "styles" / "ui.css"
ui_path.write_text(
    "/* Semantic classes from docs/visual/screens.html (inside .ph) */\n" + phone_css.group(1).strip() + "\n",
    encoding="utf-8",
)
print(f"wrote {tokens_path} and {ui_path}")
