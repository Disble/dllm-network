import { execFileSync } from 'node:child_process';

const root = new URL('../', import.meta.url);

execFileSync('golangci-lint', ['run', './...'], {
  cwd: root,
  stdio: 'inherit',
});
