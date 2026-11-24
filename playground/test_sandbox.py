"""Security regression tests for hosted Studio (fix 2026-11-22).

`ext` blocks carry raw Python/Go/JS that `cuni check` executes, so hosted
Studio must refuse them, and every child process must run unprivileged.

Run: python3 -m unittest playground.test_sandbox -v
The compiler test needs a built cuni (CUNI_BIN or target/release/cuni) and is
skipped otherwise; the uid test needs root and is skipped otherwise.
"""

from __future__ import annotations

import os
import shutil
import sys
import tempfile
import unittest
from pathlib import Path

PLAY = Path(__file__).resolve().parent
if str(PLAY) not in sys.path:
    sys.path.insert(0, str(PLAY))

import sandbox  # noqa: E402

EXT_SRC = (
    "ext whoami() -> str do\n"
    '    py: __import__("getpass").getuser()\n'
    '    go: "go"\n'
    '    js: require("os").userInfo().username\n'
    "end\n\n"
    "say(whoami())\n"
)


def _cuni() -> str | None:
    cand = os.environ.get("CUNI_BIN") or str(PLAY.parent / "target" / "release" / "cuni")
    return cand if Path(cand).is_file() else shutil.which("cuni")


class ChildEnv(unittest.TestCase):
    def test_deny_ext_forced_and_secrets_stripped(self):
        os.environ["SOME_API_TOKEN"] = "x"
        os.environ.pop("CUNI_PLAYGROUND_ALLOW_EXT", None)
        try:
            env = sandbox.child_env("/tmp")
        finally:
            os.environ.pop("SOME_API_TOKEN", None)
        self.assertEqual(env.get("CUNI_DENY_EXT"), "1")
        self.assertNotIn("SOME_API_TOKEN", env)
        self.assertEqual(env["HOME"], "/tmp")

    def test_prlimit_wrap(self):
        args = sandbox.wrap(["echo", "hi"], 40)
        self.assertEqual(args[-2:], ["echo", "hi"])
        if shutil.which("prlimit"):
            self.assertIn("--cpu=45", args)
            self.assertIn("--core=0", args)


@unittest.skipUnless(hasattr(os, "geteuid") and os.geteuid() == 0, "needs root to test privilege drop")
class PrivilegeDrop(unittest.TestCase):
    def test_child_runs_as_sandbox_uid(self):
        with tempfile.TemporaryDirectory() as td:
            sandbox.own_workdir(td)
            r = sandbox.run(["id", "-u"], td, 10)
            self.assertEqual(r.stdout.strip(), str(sandbox.SANDBOX_UID))


@unittest.skipUnless(_cuni(), "needs a built cuni binary")
class CompilerRefusesExt(unittest.TestCase):
    def test_check_refuses_ext_in_sandbox(self):
        with tempfile.TemporaryDirectory() as td:
            sandbox.own_workdir(td)
            main = Path(td) / "main.cuni"
            main.write_text(EXT_SRC, encoding="utf-8")
            r = sandbox.run([_cuni(), "check", str(main), "--only", "py,js", "--timeout", "20"], td, 60)
            out = (r.stdout or "") + (r.stderr or "")
            self.assertIn("CUNI_DENY_EXT", out)
            self.assertNotIn("PASS", out)

    def test_portable_source_still_passes(self):
        with tempfile.TemporaryDirectory() as td:
            sandbox.own_workdir(td)
            main = Path(td) / "main.cuni"
            main.write_text("let x = 2 + 3\nsay(x)\n", encoding="utf-8")
            r = sandbox.run([_cuni(), "check", str(main), "--only", "py,js", "--timeout", "20"], td, 60)
            self.assertIn("exactness: PASS", r.stdout)


if __name__ == "__main__":
    unittest.main()
