"""Sandbox for every subprocess CuNi Studio starts (security fix 2026-11-22).

Studio compiles and RUNS user programs. Three layers keep a hostile program
from owning the host:

1. ``CUNI_DENY_EXT=1`` is forced into every child's environment, so the
   compiler refuses ``ext`` blocks (raw target source) at parse time,
   including inside ``use``d modules.
2. When the server runs as root, children drop to an unprivileged uid/gid
   (``CUNI_PLAYGROUND_SANDBOX_UID``, default 65534 "nobody") with no
   supplementary groups, so they cannot write /data or read root-only files.
3. ``prlimit`` caps CPU seconds, file size and core dumps per child.

Secrets are stripped from the child environment either way.
Local development can opt out of layer 1 only with
``CUNI_PLAYGROUND_ALLOW_EXT=1``.
"""
from __future__ import annotations

import os
import shutil
import subprocess
import tempfile
from pathlib import Path

SANDBOX_UID = int(os.environ.get("CUNI_PLAYGROUND_SANDBOX_UID", "65534"))
FSIZE_BYTES = int(os.environ.get("CUNI_PLAYGROUND_FSIZE_BYTES", str(64 * 1024 * 1024)))
_SECRET_MARKERS = ("TOKEN", "SECRET", "PASSWORD", "PRIVATE", "API_KEY", "ACCESS_KEY", "CREDENTIAL")


def ext_allowed() -> bool:
    return os.environ.get("CUNI_PLAYGROUND_ALLOW_EXT", "").strip() == "1"


def _is_root() -> bool:
    return hasattr(os, "geteuid") and os.geteuid() == 0


def _go_cache() -> str:
    d = Path(tempfile.gettempdir()) / "cuni-sandbox-gocache"
    d.mkdir(mode=0o700, exist_ok=True)
    if _is_root() and SANDBOX_UID > 0:
        os.chown(d, SANDBOX_UID, SANDBOX_UID)
    return str(d)


def child_env(cwd: Path | str | None = None) -> dict[str, str]:
    env = {
        k: v
        for k, v in os.environ.items()
        if not any(m in k.upper() for m in _SECRET_MARKERS)
    }
    home = str(cwd) if cwd else tempfile.gettempdir()
    env["HOME"] = home
    env["GOCACHE"] = _go_cache()
    env["GOPATH"] = str(Path(tempfile.gettempdir()) / "cuni-sandbox-gopath")
    env["GOFLAGS"] = "-mod=mod"
    env["GOTOOLCHAIN"] = "local"
    if not ext_allowed():
        env["CUNI_DENY_EXT"] = "1"
    return env


def own_workdir(path: Path | str) -> None:
    """Hand a fresh temp dir to the sandbox user so children can write in it."""
    if _is_root() and SANDBOX_UID > 0:
        os.chown(path, SANDBOX_UID, SANDBOX_UID)


def wrap(args: list[str], timeout: int) -> list[str]:
    prl = shutil.which("prlimit")
    if not prl:
        return list(args)
    cpu = max(5, int(timeout) + 5)
    return [prl, f"--cpu={cpu}", f"--fsize={FSIZE_BYTES}", "--core=0", "--"] + list(args)


def run(args: list[str], cwd: Path | str | None, timeout: int) -> subprocess.CompletedProcess:
    kw: dict = {}
    if _is_root() and SANDBOX_UID > 0:
        kw = {"user": SANDBOX_UID, "group": SANDBOX_UID, "extra_groups": []}
    return subprocess.run(
        wrap(args, timeout),
        cwd=str(cwd) if cwd else None,
        env=child_env(cwd),
        capture_output=True,
        text=True,
        timeout=timeout,
        **kw,
    )
