from __future__ import annotations

import importlib.util
import tempfile
import unittest
from pathlib import Path


SCRIPT = Path(__file__).with_name("check-docs.py")
SPEC = importlib.util.spec_from_file_location("check_docs", SCRIPT)
assert SPEC and SPEC.loader
check_docs = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(check_docs)


class CheckDocsTests(unittest.TestCase):
    def test_heading_anchors_match_github_duplicates(self) -> None:
        anchors = check_docs.heading_anchors("# Hello, World!\n## Hello, World!\n")
        self.assertEqual(anchors, {"hello-world", "hello-world-1"})

    def test_local_link_errors_reports_missing_anchor(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            source = root / "README.md"
            target = root / "guide.md"
            source.write_text("[broken](guide.md#missing)\n", encoding="utf-8")
            target.write_text("# Present\n", encoding="utf-8")
            old_root = check_docs.ROOT
            check_docs.ROOT = root
            try:
                errors = check_docs.local_link_errors(source, source.read_text())
            finally:
                check_docs.ROOT = old_root
            self.assertEqual(len(errors), 1)
            self.assertIn("missing Markdown anchor", errors[0])

    def test_local_link_errors_accepts_same_page_anchor(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            source = root / "README.md"
            source.write_text("# Present\n[ok](#present)\n", encoding="utf-8")
            old_root = check_docs.ROOT
            check_docs.ROOT = root
            try:
                errors = check_docs.local_link_errors(source, source.read_text())
            finally:
                check_docs.ROOT = old_root
            self.assertEqual(errors, [])


if __name__ == "__main__":
    unittest.main()
