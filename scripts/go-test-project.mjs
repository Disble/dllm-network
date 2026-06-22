import { execFileSync } from 'node:child_process';

const root = new URL('../', import.meta.url);

const packageLines = execFileSync('go', ['list', './...'], {
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

execFileSync('go', ['test', '-coverprofile=coverage.out', '-covermode=atomic', ...packageLines], {
  cwd: root,
  stdio: 'inherit',
});
