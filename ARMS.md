# CuNi — one product, seven arms

CuNi is **one product**. It has a single trunk — the compiler and its
exactness gate — and seven named arms that deliver that trunk to a
different surface. An arm is a way to reach CuNi, not a separate
product, a fork, or a different language.

The rule is the same on every arm: **one source, the same behavior on
every seat, or refuse.** No arm gets an approximate mode.

## Trunk

| | |
|---|---|
| Path | [`src/`](src) — the compiler, CLI, catalog, and emitters |
| Entry | `cuni check` · `cuni emit` · `cuni run` |
| Catalog | 53 entries: the top 50 languages + 3 onchain profiles |
| Verdict | `exactness: PASS` / `exactness: FAIL` / refusal — never a guess |
| Machine registry | [`arms.json`](arms.json) |

Every arm below sits on this trunk. Completing an arm does not create a
second compiler; it wires the same gate to another door.

## The seven arms

| Arm | Path | What it does | Start here |
|---|---|---|---|
| **Studio** | [`playground/`](playground) | Hosted browser workspace: emit, `cuni check`, Agent mode, Publish. | [playground/README.md](playground/README.md) |
| **Bank** | [`src/bank.rs`](src/bank.rs) · [`playground/bank.html`](playground/bank.html) | Paste N, get X: ingest → emit → prove, or refuse. v1 `--from` is `py` or `cuni`. | [docs/BANK.md](docs/BANK.md) |
| **Audit** | [`src/audit.rs`](src/audit.rs) | Prove a foreign implementation against a money law; file a signed JSON receipt. PASS or REFUSE is always filed. | [docs/FINANCIAL.md](docs/FINANCIAL.md) |
| **Onchain** | [`src/emit.rs`](src/emit.rs) | Six chain profiles (`--emit-ink`, `--emit-move`, `--emit-vyper`, `--emit-cairo`, `--emit-clarity`, `--emit-cadence`): logic core proven, chain shell in the chain's idiom. | [docs/ONCHAIN.md](docs/ONCHAIN.md) |
| **Exactness** | [`exactness/`](exactness) | The EXACT-or-REFUSE skill: build every local seat, return fingerprints or a diff. | [exactness/README.md](exactness/README.md) |
| **Transparency** | [`transparency/`](transparency) | Append-only Merkle log (RFC 6962) anchoring signed exactness receipts. Proves a receipt existed; it does not vouch for its claim. | [transparency/README.md](transparency/README.md) |
| **Tap** | [`homebrew/`](homebrew) | The Homebrew tap: `brew install ceedot-rock/cuni/cuni`. | [homebrew/README.md](homebrew/README.md) |

## What is not an arm

- **[`ceedot-rock/cuni-exact`](https://github.com/ceedot-rock/cuni-exact)** — the private
  exactness workbench. It is separate TruKern material, not part of the
  public CuNi product or its catalog.
- Archived repositories (`cuni`, `cuni-exactness`, `cuni-transparency`,
  `homebrew-cuni`) keep their history and point here. They are the same
  product at an older address, not extra arms.

## Adding or changing an arm

The registry and this page must move together:

1. Add the arm to [`arms.json`](arms.json) (id, name, path, entry, doc).
2. Add the row above with the same name and doc path.
3. Run `cargo test --test arms` — it fails if the registry, the paths,
   or this page drift apart.

The test is deliberately small: it does not judge an arm's quality, only
that the product has one honest, machine-readable shape.
