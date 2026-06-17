import { render } from '@testing-library/react';
import { createElement } from 'react';
import { describe, expect, it } from 'vitest';

import { DashboardApp } from '../dashboard-app';

// Smoke test: render the FULL app tree exactly as production does — DashboardApp
// with NO props, so every hook resolves the singleton snapshot source. This is
// the composition that white-screened in the real Wails window; the existing
// tests always injected a source prop and never exercised this path.
describe('DashboardApp (full tree smoke)', () => {
  it('mounts without throwing using the default singleton source', () => {
    expect(() => render(createElement(DashboardApp))).not.toThrow();
  });
});
