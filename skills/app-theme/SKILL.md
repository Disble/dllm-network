---
name: app-theme
description: "Trigger: styling, CSS, theme, colors, design tokens, charts, or new UI components in ollama-telemetry frontend. Apply the monochrome design system."
license: Apache-2.0
metadata:
  author: gentleman-programming
  version: "1.0"
---

## Activation Contract

Apply when writing, restyling, or reviewing any frontend UI in `ollama-telemetry/frontend` — components, CSS, charts, colors, or layout. This is the single source of truth for the app's look.

## Hard Rules

- The theme lives in `frontend/src/style.css` `:root` as CSS custom properties. Change the theme THERE only; components consume tokens via `var(--token)`. NEVER hardcode a hex/rgb color in a component or rule.
- Palette is monochrome near-black (Coolors), hue-less. Status colors are muted and low-saturation — never bright.
- Charts are hand-rolled SVG / flex `<div>` segments. NO chart library. Reuse the atoms below; new microcharts follow the same pattern.
- Display invariant `null != zero`: unavailable metrics render `UNAVAILABLE_LABEL` (`—`), never a fabricated `0`. Do not invent data the backend did not measure.
- Class naming is BEM-ish: `block__element--modifier` (e.g. `inference-detail__view-option--active`).
- Keep a `role=` only when load-bearing for a11y (carries an `aria-label`); add `// eslint-disable-next-line react-doctor/prefer-tag-over-role` with a one-line reason, as `WaterfallBar`/`inference-table` do.

## Decision Gates

| Need | Token / atom |
|------|--------------|
| Page / panel surface | `--bg`, `--surface`, `--surface-2` (only these exist — NO `--surface-1/3/4`) |
| Elevated / hover / selected state | `--row-hover`, `--row-selected`, `--accent-soft` (not a new surface) |
| Separation | `--border`, `--border-strong`, `--border-subtle` (hairlines, not shadows) |
| Text hierarchy | `--text` → `--text-muted` → `--text-subtle` |
| Accent | `--accent` (hue-less), `--accent-soft` |
| Status | `--success`, `--warn`/`--warn-bg`, `--error`/`--error-bg` |
| Line chart | `Sparkline` atom (SVG polyline) |
| Timing / waterfall | `TimingBar`, `WaterfallBar` atoms (flex div segments) |
| Pill / segmented toggle / chip | `border-radius: 999px`; toggle = track + options with `aria-pressed`; chip uses `--accent-soft` |

## Execution Steps

1. Read existing tokens and the nearest sibling component before adding styles; match its idiom.
2. Use tokens for every color, border, and surface — add a new token to `:root` if one is missing, then consume it.
3. For data viz, reuse or extend an existing atom; do not add a charting dependency.
4. Render `—` for null/unavailable values; gate on `tokens != null` before reading metrics.
5. eslint: only `window`/`document` are globals (use `window.navigator`, `window.setTimeout`); no root-level `const` in feature `*.tsx` — move constants to `*.constants.ts`.

## Output Contract

State which tokens/atoms were used and confirm no hardcoded colors, no chart library, and that null states render `—`.

## References

- `frontend/src/style.css` — `:root` design tokens (the theme).
- `frontend/src/shared/ui/atoms/` — `sparkline`, `timing-bar`, `waterfall-bar` chart atoms.
