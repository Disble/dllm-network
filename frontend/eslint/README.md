# Frontend standards adaptation

These ESLint rules adapt the proven `autoreas-mobile` architecture standards to the Wails React web frontend used by `ollama-telemetry`.

## Quick path

1. Keep `src/app/` limited to composition.
2. Keep user-facing behavior in `src/features/`.
3. Keep shared contracts and helpers in `src/shared/`.
4. Run `bun run lint`, `bun run typecheck`, and `bun run doctor:react` after frontend changes.

## Practical rules

| Area | Practical expectation |
|------|-----------------------|
| `app/` | Compose screens and feature entrypoints only. No direct infrastructure imports. |
| `features/` | Own feature hooks, helpers, presenters, and tests. |
| `shared/` | Host shared contracts, constants, and helpers that do not depend on feature code. |
| readonly props | Public component Props stay readonly at the boundary. |
| public JSDoc | Exported hooks, constants, and type contracts need documentation where lint requires it. |
| Colocation | Hooks, helpers, constants, schemas, and types stay in their designated files. |

## Why this exists

The original `autoreas-mobile` rules targeted Expo/mobile structure. This project keeps the same architectural intent while mapping it to a Wails web directory layout and Bun-based validation workflow.
