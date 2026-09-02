---
format: https://specscore.md/features-index-specification
---

# Features

Feature specifications for this project.

## Index

| Feature | Status | Description |
|---------|--------|-------------|
| [Telegram Pair-Matching bot (Solo, vs Bot, vs Humans)](telegram-pair-matching-bot/README.md) | Amending | Play **Pair-Matching** (a memory/concentration game) inside Telegram through SneatBot's `/games`. |
| [Pair Matching rules engine](pair-matching-rules/README.md) | Stable | The pairgame rules engine: flip resolution (no turn order, independent per-player pending picks, any-player sniping credited to the flipper), board-layout derivation and secrecy, the Solo snapshot codec, and the robot strategies shared identically by Solo, vs Bot, and vs Humans. |
| [Pair Matching stored-mode sessions](pair-matching-sessions/README.md) | Stable | Server-side persistence and session composition for the two stored Pair-Matching modes (vs Bot, vs Humans): the dal4pairgame DBO and dalgo persistence, and the pairsession API (create/join/start/flip/robot-move/view) plus anchored-message bookkeeping. |

## Open Questions

None at this time.

---
*This document follows the https://specscore.md/features-index-specification*
