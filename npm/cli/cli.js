#!/usr/bin/env node

const { existsSync } = require("node:fs");
const { spawnSync } = require("node:child_process");
const { binaryPath, install } = require("./lib/release");
const { version } = require("./package.json");

async function main({ executable = binaryPath(__dirname), executableExists = existsSync, installBinary = () => install(__dirname, version), run = spawnSync, args = process.argv.slice(2), report = console.error } = {}) {
  if (!executableExists(executable)) {
    report(`Downloading and verifying HarnessForge ${version} for this computer...`);
    try {
      await installBinary();
    } catch (error) {
      report(`Could not install HarnessForge: ${error.message}`);
      return 1;
    }
  }

  const result = run(executable, args, { stdio: "inherit" });
  if (result.error) {
    report(`Could not start HarnessForge: ${result.error.message}`);
    return 1;
  }
  return result.status ?? 1;
}

if (require.main === module) {
  main().then((status) => { process.exitCode = status; }).catch((error) => {
    console.error(`Could not start HarnessForge: ${error.message}`);
    process.exitCode = 1;
  });
}

module.exports = { main };
