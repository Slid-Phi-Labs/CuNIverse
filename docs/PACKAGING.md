# CuNi packaging (v0.10.0)

**Goal**: installable `cuni` binary without cloning the full repo.  
**Status**: **v0.10.0**. Preferred path: `cargo install cuni`. Git tag and GitHub Release assets match for all four target triples.

## One-command install

```bash
cargo install cuni
# or from git:
cargo install --git https://github.com/ceedot-rock/cuni --tag v0.10.0
```

Requires a Rust toolchain. After install, `cuni --help` and `cuni check examples/full.cuni` work if the examples are present (or clone just for the suite).

## Prebuilt binaries (v0.10.0)

GitHub Release [v0.10.0](https://github.com/ceedot-rock/cuni/releases/tag/v0.10.0) ships:

| Asset | Notes |
|-------|-------|
| `cuni-0.10.0-x86_64-unknown-linux-gnu.tar.gz` | single `cuni` binary at tarball root |
| `cuni-0.10.0-aarch64-unknown-linux-gnu.tar.gz` | single `cuni` binary at tarball root |
| `cuni-0.10.0-x86_64-apple-darwin.tar.gz` | single `cuni` binary at tarball root |
| `cuni-0.10.0-aarch64-apple-darwin.tar.gz` | single `cuni` binary at tarball root |
| `cuni-0.10.0-aarch64-apple-darwin.tar.gz` | single `cuni` binary at tarball root |
| `CHECKSUMS.txt` | SHA-256 checksums for all four tarballs |

```bash
curl -sL https://github.com/ceedot-rock/cuni/releases/download/v0.10.0/cuni-0.10.0-x86_64-unknown-linux-gnu.tar.gz \
  | tar -xz && ./cuni --help
```

## Local release binary

```bash
git clone https://github.com/ceedot-rock/cuni.git && cd cuni
cargo build --release
# → target/release/cuni
```

Helper: [`packaging/scripts/build-release-assets.sh`](../packaging/scripts/build-release-assets.sh) `0.10.0`

## cargo-binstall

`Cargo.toml` includes `[package.metadata.binstall]` aimed at:

```text
{repo}/releases/download/v{version}/cuni-{version}-{target}.tar.gz
```

```bash
cargo binstall cuni
```

All four target triples are uploaded for v0.10.0 (see the asset table above).

### Release asset naming

| Target triple | Asset name |
|---------------|------------|
| `x86_64-unknown-linux-gnu` | `cuni-0.10.0-x86_64-unknown-linux-gnu.tar.gz` |
| `aarch64-unknown-linux-gnu` | `cuni-0.10.0-aarch64-unknown-linux-gnu.tar.gz` |
| `x86_64-apple-darwin` | `cuni-0.10.0-x86_64-apple-darwin.tar.gz` |
| `aarch64-apple-darwin` | `cuni-0.10.0-aarch64-apple-darwin.tar.gz` |

Each tarball contains a single `cuni` binary at the root.

## Homebrew

Formula: [`packaging/homebrew/cuni.rb`](../packaging/homebrew/cuni.rb) — points at the **v0.10.0** release tarballs, with SHA-256 values taken from the release `CHECKSUMS.txt`.

```bash
brew install --build-from-source ./packaging/homebrew/cuni.rb
```
