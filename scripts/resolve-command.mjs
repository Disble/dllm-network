import { existsSync } from 'node:fs';
import path from 'node:path';

/**
 * EXECUTABLE_EXTENSION is the platform-specific suffix used when probing for
 * command binaries on the file system. Windows executables carry a `.exe`
 * extension; POSIX systems do not.
 */
const EXECUTABLE_EXTENSION = process.platform === 'win32' ? '.exe' : '';

/**
 * resolveCommand locates the absolute path of a command binary by scanning the
 * directories listed in the `PATH` environment variable. The returned path is
 * then safe to pass to child_process APIs without relying on PATH resolution at
 * execution time (mitigating PATH-hijacking attacks such as S4036).
 *
 * @param {string} commandName - The executable name without extension.
 * @returns {string} An absolute path to the resolved executable.
 * @throws {Error} When the command cannot be found in PATH or resolves to a
 *   non-absolute path.
 */
export function resolveCommand(commandName) {
  const searchPath = process.env.PATH ?? '';

  for (const directory of searchPath.split(path.delimiter)) {
    if (!directory) {
      continue;
    }

    const candidate = path.join(directory, `${commandName}${EXECUTABLE_EXTENSION}`);

    if (existsSync(candidate)) {
      if (!path.isAbsolute(candidate)) {
        throw new Error(
          `Resolved command "${commandName}" is not an absolute path: ${candidate}`,
        );
      }
      return candidate;
    }
  }

  throw new Error(
    `Could not resolve absolute path for command "${commandName}" using PATH directories: ${searchPath}`,
  );
}
