# CuNi Receipt Anchoring — independent timestamps for CuNi receipts

CuNi receipts prove a game's rules are exact. They do not, by themselves,
prove *when* a receipt existed: a receipt file in an operator-controlled
repo could in principle be swapped after the fact, and a player or
regulator would have to trust that it wasn't.

This module removes that trust. Receipts are batched into a SHA-256
Merkle tree; the tree's root is anchored on **Base** in a single
transaction. From then on, anyone can verify — using only the receipt,
its proof file, and a public chain RPC — that this exact receipt existed
no later than a specific block timestamp. No Slid Phi Labs server is
involved at verification time.

Go, standard library only (Keccak-256 and secp256k1 are implemented
in-tree; there are no third-party dependencies to audit or vendor).

## What it proves — and what it does NOT

**It proves:**

- A receipt **existed, byte-identical**, no later than the anchor
  block's timestamp (the digest in the tree is over the receipt's
  canonical form, so any later edit — a flipped verdict, an edited
  summary — changes the digest and fails verification).
- The batch root is **on a public chain**, placed there at a block
  height anyone can look up; neither the operator nor anyone else can
  backdate it.

**It does NOT prove:**

- That the game was **fair** or the result **correct** — anchoring proves
  a statement was made by a time, not that the statement is true.
  Fairness is what the CuNi receipt itself (and the provably-fair game
  protocol) attests; the anchor only timestamps it.
- That the anchor signer is Slid Phi Labs — v1 has no on-chain identity
  binding. The signer address is recorded in every proof; binding it to
  the lab is a publication step (below), not a cryptographic one yet.

In short: **existence + integrity, timestamped by the chain — not truth,
not identity.**

## How it works

1. **Digest.** Each receipt is reduced to SHA-256 over its canonical JSON
   (keys sorted, no insignificant whitespace, numbers kept as written —
   see `canonical.go`). The receipt's own `source_hash` is *not* used as
   the leaf: in CuNi receipts it commits to the game source, which two
   different receipts can share. It is carried in the proof and
   cross-checked instead.
2. **Batch.** Digests become leaves of a binary SHA-256 Merkle tree
   (odd levels duplicate their last node). One batch record
   (`batch.json`) holds the batch id, leaf count, and root; one proof
   file per receipt holds its Merkle path.
3. **Anchor.** One 0-value EIP-1559 transaction per batch, sent to the
   signer's own address, with 68 bytes of calldata:

       "CUNI" || SHA-256("cuni-anchor/v1:" || batch_id) || merkle_root

   No contract deployment. The batch record is updated with the tx hash,
   block number, and the block timestamp read back from the chain.
4. **Verify.** Given a receipt + its proof, the verifier recomputes the
   digest, walks the path to the root, fetches the anchor transaction
   from a public Base RPC, checks the calldata carries this batch's key
   and root, and reports the block timestamp — read from the chain
   during verification, never trusted from the proof file.

## Usage

```sh
go build -o cuni-anchor ./cmd/cuni-anchor

# Batch receipts (any CuNi receipt JSON files)
cuni-anchor batch -id 2026-10-22-games -out ./batch-dir receipt1.json receipt2.json

# Anchor (signer key via environment ONLY — never a flag, never in source)
export CUNI_ANCHOR_PRIVATE_KEY=<hex key of the anchoring wallet>
cuni-anchor anchor -batch ./batch-dir            # Base Sepolia by default
cuni-anchor anchor -batch ./batch-dir -rpc https://mainnet.base.org

# Verify — a player/regulator needs nothing from the operator but the
# receipt and its proof:
cuni-anchor verify -receipt receipt.json -proof receipt.proof.json
```

Preview without broadcasting: `cuni-anchor anchor -batch <dir> -dry-run`
builds and signs the transaction from live chain data and prints it.

## Cost

One transaction per batch, regardless of batch size. Measured on Base
Sepolia for the 68-byte anchor calldata: **24,058 gas** (node estimate;
intrinsic floor 22,088). At Base mainnet's gas price observed
2026-10-22 (0.006 gwei) that is ≈ $0.00035 of L2 execution per batch,
plus the OP-Stack L1 data fee (small for 68 bytes; all-in mainnet cost
per batch estimated at **$0.001–0.01**). Per receipt: divide by batch
size — ≈ $0.00002–0.0005 at 19 receipts/batch, ≈ $0.000001–0.00001 at
1,000. For comparison, the commercial pitch this replaces charged
**$0.05 per receipt**. Figures are estimates from the stated inputs, not
quotes; the L1 fee varies with Ethereum mainnet gas.

## Mainnet go-live — Corey's call, not done here

1. **Funded Base wallet.** Create/fund a dedicated mainnet anchoring
   wallet (small balance — anchoring is cheap; a few dollars of ETH is
   months of batches). Key custody is Corey's decision: hot key on the
   anchoring host vs. hardware signer. The key reaches this tool only
   via `CUNI_ANCHOR_PRIVATE_KEY` at run time.
2. **Publish the signer address** alongside the receipts (site + repo),
   so verifiers can recognize the lab's anchor sender. Until then,
   proofs show an address nobody can attribute.
3. **Point the anchor step at mainnet** (`-rpc https://mainnet.base.org`)
   and pin the public RPC(s) the verifier defaults to.
4. Decide batch cadence (per game-day? per N receipts?) — the economics
   above make cadence a policy choice, not a cost constraint.

## Layout

| File | Role |
|---|---|
| `canonical.go` | Canonical JSON + receipt digest (+ `source_hash` extraction) |
| `merkle.go` | Merkle tree, roots, inclusion proofs |
| `batch.go` | Batch/proof records, calldata format |
| `anchor.go` | Transaction build/sign/broadcast, batch finalization |
| `verify.go` | The third-party verification (library form) |
| `rpc.go` | Minimal Ethereum JSON-RPC client |
| `keccak.go`, `secp256k1.go`, `rlp.go` | In-tree crypto primitives |
| `cmd/cuni-anchor/` | CLI: keygen / digest / batch / anchor / verify |
| `examples/sepolia-poc/` | Worked PoC over the 19 ExactOdds receipts |
