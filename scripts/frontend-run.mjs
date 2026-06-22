import { execFileSync } from 'node:child_process';
import { readFileSync } from 'node:fs';

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

execFileSync('bun', ['run', scriptName], {
  cwd: frontendDir,
  stdio: 'inherit',
});
