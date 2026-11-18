# Sepolia PoC — status

Batch over the 19 receipts in `~/workspace/gitfix/exactodds/receipts/`
(the rule-pack exactness receipts whose repo-only existence motivated
this build).

- **Batch id:** `2026-10-22-exactodds-receipts-poc`
- **Leaves:** 19 · **Merkle root:** `30cd774cb620fbbebfe2f55f22971e7b4a03dfe39997d839b8e6840f49452a70`
- **batch.json + 19 proof files:** built and present in this directory.
- **Anchor transaction: ANCHORED 2026-10-22.** Tx
  `0x176fb27b7fb3ec9da3a3b89988398fb22549cca6ea0d012f1f2eafee0dccc9ab`
  on Base Sepolia, block **48781694**, block timestamp **1792678207**
  (2026-10-22T14:10:07Z), gas used 23,720. `batch.json` carries the
  full anchor record.
- **Funding path used:** public faucets were all gated, so the signer
  was funded with 0.05 ETH on Ethereum Sepolia (L1) and 0.01 ETH was
  bridged to Base Sepolia via the official L1StandardBridge
  (`0xfd0Bf71F60660E2f608ed56e1659C450eB113120`, `bridgeETH`; bridge tx
  `0xff7df6c3f2f2ab8f3e94aebfb1f3184236bb93111385209840f0cf9563c90d7d`).
  The bridge/portal addresses were verified on-chain through their
  SystemConfig (`0xf272670eb55e895584501d564AfEB048bEd26194`), whose
  `l2ChainId` is 84532.
- **Verification: 19/19 PASS.** Every receipt in this batch was run
  through `cuni-anchor verify` against the live anchor transaction;
  all digests walk to the anchored root and the anchor block timestamp
  above is the chain-read existence bound.

(The signer key is a fresh, testnet-only ephemeral key generated for
this PoC, stored mode-0600 at `~/.config/cuni-anchor/sepolia-poc.key`.
It controls nothing else and no mainnet funds. Re-running `anchor`
rebuilds the transaction with fresh fees/nonce, so the stale dry-run
transaction in `anchor-tx.hex` does not need to be used.)
