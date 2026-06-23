import { execFileSync } from 'node:child_process';

import { resolveCommand } from './resolve-command.mjs';

/**
 * GO_PATH is the absolute path to the Go executable resolved from PATH at
 * startup. Passing an absolute path to execFileSync avoids relying on the
 * inherited PATH when spawning the child process (S4036).
 */
const GO_PATH = resolveCommand('go');

const root = new URL('../', import.meta.url);

const packageLines = execFileSync(GO_PATH, ['list', './...'], {
  cwd: root,
  encoding: 'utf8',
})
  .split(/\r?\n/)
  .map((line) => line.trim())
  .filter((line) => line.length > 0)
  .filter((line) => !line.includes('frontend/node_modules/'));

if (packageLines.length === 0) {
  throw new Error('No Go packages found for project validation.');
}

execFileSync(GO_PATH, ['test', '-coverprofile=coverage.out', '-covermode=atomic', ...packageLines], {
  cwd: root,
  stdio: 'inherit',
});
