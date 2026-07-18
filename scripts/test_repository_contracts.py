from __future__ import annotations

import re
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parent.parent


class RepositoryContractTests(unittest.TestCase):
    def test_mise_matches_go_directive(self) -> None:
        go_mod = (ROOT / "go.mod").read_text(encoding="utf-8")
        mise = (ROOT / ".mise.toml").read_text(encoding="utf-8")
        go_version = re.search(r"^go (\S+)$", go_mod, re.MULTILINE)
        mise_version = re.search(r'^go = "([^"]+)"$', mise, re.MULTILINE)
        self.assertIsNotNone(go_version)
        self.assertIsNotNone(mise_version)
        self.assertEqual(go_version.group(1), mise_version.group(1))

    def test_linux_arm_release_is_natively_smoke_tested(self) -> None:
        workflow = (ROOT / ".github/workflows/release.yml").read_text(encoding="utf-8")
        arm_entry = workflow.split("- target: linux-arm64", 1)[1].split("- target:", 1)[0]
        self.assertIn("runner: ubuntu-24.04-arm", arm_entry)
        self.assertIn("smoke: true", arm_entry)

    def test_openwiki_publish_mirrors_generated_tree(self) -> None:
        workflow = (ROOT / ".github/workflows/openwiki-update.yml").read_text(encoding="utf-8")
        clear = workflow.index("rm -rf -- openwiki")
        download = workflow.index("name: Download generated documentation")
        self.assertLess(clear, download)
        self.assertIn("git add -A -- openwiki", workflow)
        self.assertIn("title=Missing OpenWiki API key", workflow)


if __name__ == "__main__":
    unittest.main()
