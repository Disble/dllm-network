# Development and validation

Use Bun at the repository root for the normal validation flow.

## Quick path

1. Install frontend dependencies with `bun install` inside `frontend/`.
2. Run `bun run test:go`.
3. Run `bun run test`.
4. Run `bun run lint`.
5. Run `bun run typecheck`.
6. Run `bun run validate`.

## Validation commands

| Command | Purpose |
|---------|---------|
| `bun run test:go` | Runs project Go tests through `scripts/go-test-project.mjs`, using `go list ./...` and excluding `frontend/node_modules` package discovery noise. |
| `bun run test` | Runs Go tests plus the frontend `test` script through a repository-root wrapper. |
| `bun run lint` | Runs the frontend `lint` script through a repository-root wrapper. |
| `bun run typecheck` | Runs the frontend `typecheck` script through a repository-root wrapper. |
| `bun run validate` | Runs the complete local validation flow: Go tests, frontend tests, lint, typecheck, and React Doctor. |
| `bun run doctor:react` | Runs the frontend `doctor:react` script through a repository-root wrapper. |

## Why Go validation uses a script

`go test ./...` walks into `frontend/node_modules/flatted/golang/pkg/flatted` after the frontend install. That path is not part of the product code. The root script keeps validation honest by discovering real project Go packages with `go list ./...` and excluding `frontend/node_modules` only. Real Go packages under the repository root still run.

## Frontend standards in practice

- Keep `app/` thin and free of direct runtime integration.
- Keep feature logic in `features/` hooks and helpers.
- Keep shared contracts and formatters in `shared/`.
- Respect readonly props, public JSDoc contracts, colocation rules, and architecture boundaries from the migrated `autoreas-mobile` standards.
