import { execFileSync } from 'node:child_process';

const scriptName = process.argv[2];

if (!scriptName) {
  throw new Error('A frontend script name is required.');
}

const frontendDir = new URL('../frontend', import.meta.url);

execFileSync('bun', ['run', scriptName], {
  cwd: frontendDir,
  stdio: 'inherit',
});
