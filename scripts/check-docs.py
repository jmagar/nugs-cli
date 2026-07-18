#!/usr/bin/env python3
"""Validate local Markdown links, portable paths, and Config field coverage."""

from __future__ import annotations

import html
import re
import sys
from pathlib import Path
from urllib.parse import unquote


ROOT = Path(__file__).resolve().parent.parent
PUBLIC_DOCS = [ROOT / "README.md", ROOT / "CLAUDE.md", ROOT / "CHANGELOG.md"]
PUBLIC_DOCS.extend(sorted((ROOT / "docs").glob("*.md")))
PUBLIC_DOCS.append(ROOT / "INCREMENTAL_CATALOG_UPDATE.md")

LINK_RE = re.compile(r"(?<!!)\[[^]]*]\(([^)]+)\)")
JSON_TAG_RE = re.compile(r"json:\"([^\",]+)")
HEADING_RE = re.compile(r"^\s{0,3}#{1,6}\s+(.+?)\s*#*\s*$")


def heading_anchors(text: str) -> set[str]:
    anchors: set[str] = set()
    occurrences: dict[str, int] = {}
    for line in text.splitlines():
        match = HEADING_RE.match(line)
        if not match:
            continue
        heading = html.unescape(match.group(1))
        heading = re.sub(r"<[^>]+>", "", heading)
        heading = re.sub(r"[`*_~]", "", heading).strip().lower()
        slug = re.sub(r"[^\w\- ]", "", heading, flags=re.UNICODE)
        slug = re.sub(r"\s+", "-", slug)
        duplicate = occurrences.get(slug, 0)
        occurrences[slug] = duplicate + 1
        anchors.add(slug if duplicate == 0 else f"{slug}-{duplicate}")
    return anchors


def local_link_errors(path: Path, text: str) -> list[str]:
    errors: list[str] = []
    for raw_target in LINK_RE.findall(text):
        target = raw_target.strip().split(maxsplit=1)[0].strip("<>")
        if not target or target.startswith(("http://", "https://", "mailto:")):
            continue
        file_part, _, fragment = target.partition("#")
        file_part = unquote(file_part)
        fragment = unquote(fragment)
        resolved = path.resolve() if not file_part else (path.parent / file_part).resolve()
        try:
            resolved.relative_to(ROOT)
        except ValueError:
            errors.append(f"{path.relative_to(ROOT)}: link escapes repository: {target}")
            continue
        if not resolved.exists():
            errors.append(f"{path.relative_to(ROOT)}: missing local link target: {target}")
            continue
        if fragment and resolved.suffix.lower() == ".md":
            anchors = heading_anchors(resolved.read_text(encoding="utf-8"))
            if fragment not in anchors:
                errors.append(
                    f"{path.relative_to(ROOT)}: missing Markdown anchor: {target}"
                )
    return errors


def config_coverage_errors() -> list[str]:
    model_text = (ROOT / "internal/model/types.go").read_text(encoding="utf-8")
    config_text = (ROOT / "docs/CONFIG.md").read_text(encoding="utf-8")
    config_struct = model_text.split("type Config struct {", 1)[1].split("\n}", 1)[0]
    fields = {tag for tag in JSON_TAG_RE.findall(config_struct) if tag != "-"}
    missing = sorted(field for field in fields if f"`{field}`" not in config_text)
    if missing:
        return ["docs/CONFIG.md: undocumented Config JSON fields: " + ", ".join(missing)]
    return []


def main() -> int:
    errors: list[str] = []
    for path in PUBLIC_DOCS:
        text = path.read_text(encoding="utf-8")
        errors.extend(local_link_errors(path, text))
        for line_no, line in enumerate(text.splitlines(), 1):
            if re.search(r"/home/[A-Za-z0-9._-]+/", line):
                errors.append(
                    f"{path.relative_to(ROOT)}:{line_no}: developer-home absolute path"
                )
    errors.extend(config_coverage_errors())

    if errors:
        print("Documentation validation failed:", file=sys.stderr)
        for error in errors:
            print(f"- {error}", file=sys.stderr)
        return 1
    print("Documentation links, portability, and Config field coverage are valid.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
