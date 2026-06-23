import { execFileSync } from 'node:child_process';
import { readFileSync } from 'node:fs';

import { resolveCommand } from './resolve-command.mjs';

/**
 * BUN_PATH is the absolute path to the Bun executable resolved from PATH at
 * startup. Passing an absolute path to execFileSync avoids relying on the
 * inherited PATH when spawning the child process (S4036).
 */
const BUN_PATH = resolveCommand('bun');

const scriptName = process.argv[2];

if (!scriptName) {
  throw new Error('A frontend script name is required.');
}

const frontendDir = new URL('../frontend', import.meta.url);
const packageJson = JSON.parse(
  readFileSync(new URL('package.json', frontendDir), 'utf8'),
);
const allowedScripts = Object.keys(packageJson.scripts ?? {});

if (!allowedScripts.includes(scriptName)) {
  throw new Error(
    `Unknown frontend script "${scriptName}". Allowed scripts: ${allowedScripts.join(', ')}`,
  );
}

execFileSync(BUN_PATH, ['run', scriptName], {
  cwd: frontendDir,
  stdio: 'inherit',
});
