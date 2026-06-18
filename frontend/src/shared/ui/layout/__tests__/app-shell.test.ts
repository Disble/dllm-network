/**
 * Tests for the application shell (AppShell + TitleBar + NavRail): it frames the
 * content, shows the brand, exposes the nav rail, and the window controls no-op
 * safely when the Wails runtime is absent (plain browser / jsdom).
 */
import { createElement } from 'react';
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';

import { AppShell } from '../app-shell';

afterEach(() => {
  cleanup();
});

describe('AppShell', () => {
  it('renders the brand, the nav rail and the content', () => {
    render(createElement(AppShell, null, createElement('p', null, 'DASHBOARD CONTENT')));

    expect(screen.getByText('Ollama Telemetry')).toBeTruthy();
    expect(screen.getByLabelText('Sections')).toBeTruthy();
    expect(screen.getByText('DASHBOARD CONTENT')).toBeTruthy();
  });

  it('window controls do not throw without the Wails runtime', () => {
    render(createElement(AppShell, null, createElement('p', null, 'x')));

    expect(() => {
      fireEvent.click(screen.getByLabelText('Minimise'));
      fireEvent.click(screen.getByLabelText('Maximise'));
      fireEvent.click(screen.getByLabelText('Close'));
    }).not.toThrow();
  });
});
