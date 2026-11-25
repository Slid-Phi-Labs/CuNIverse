# CuNIverse

**One product, one law: the same output on every target, or refuse.**

CuNi is a compiler trunk plus seven arms. The trunk is `cuni check`:
`.cuni` in, byte-identical output across 53 catalog entries or REFUSE.
No arm is a fork, a second compiler, or a separate product — each one
delivers the same trunk to a different surface.

Registry: [ARMS.md](ARMS.md) · machine-readable: [arms.json](arms.json).

| Part | Path | What it is |
|---|---|---|
| Compiler (trunk) | `/` (root) | The language, emitters, catalog, CLI: `cuni check`, `cuni run`, `cuni emit`. Start at [README.md](README.md). |
| Studio | `playground/` | Hosted browser workspace: emit, check, Agent mode, Publish. |
| Bank | `src/bank.rs` · `playground/bank.html` | Paste N, get X: ingest → emit → prove, or refuse. |
| Audit | `src/audit.rs` | Prove a foreign implementation against a money law; file a signed JSON receipt. |
| Onchain | `src/emit.rs` | Six chain profiles: proven logic core plus the chain shell. |
| Exactness | `exactness/` | The EXACT-or-REFUSE skill: every local seat, fingerprints or a diff. |
| Transparency | `transparency/` | Append-only Merkle log (RFC 6962) anchoring signed exactness receipts. |
| Tap | `homebrew/` | The Homebrew tap: `brew install ceedot-rock/cuni/cuni`. |

The private [`ceedot-rock/cuni-exact`](https://github.com/ceedot-rock/cuni-exact)
workbench is separate TruKern material and is not an arm.

## Provenance

This monorepo was fused from four repos with full history preserved
(merge commits, not squashes):

- `cuni` → root (branch `master`)
- `cuni-transparency` → `transparency/` (was `main`)
- `cuni-exactness` → `exactness/` (was `main`)
- `homebrew-cuni` → `homebrew/` (was `main`)

One tag caveat: `v0.1.0` existed in three source repos; here it resolves
to cuni's. All commits remain reachable through the merge parents.

Forget trust. Prove.
