import { execFileSync } from 'node:child_process';

import { resolveCommand } from './resolve-command.mjs';

/**
 * GOLANGCI_LINT_PATH is the absolute path to the golangci-lint executable
 * resolved from PATH at startup. Passing an absolute path to execFileSync
 * avoids relying on the inherited PATH when spawning the child process (S4036).
 */
const GOLANGCI_LINT_PATH = resolveCommand('golangci-lint');

const root = new URL('../', import.meta.url);

execFileSync(GOLANGCI_LINT_PATH, ['run', './...'], {
  cwd: root,
  stdio: 'inherit',
});
