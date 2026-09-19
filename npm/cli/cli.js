#!/usr/bin/env node

const { existsSync } = require("node:fs");
const { spawnSync } = require("node:child_process");
const { binaryPath } = require("./lib/release");

const executable = binaryPath(__dirname);
if (!existsSync(executable)) {
  console.error("HarnessForge was not installed. Reinstall the npm package without --ignore-scripts.");
  process.exit(1);
}

const result = spawnSync(executable, process.argv.slice(2), { stdio: "inherit" });
if (result.error) {
  console.error(`Could not start HarnessForge: ${result.error.message}`);
  process.exit(1);
}
process.exit(result.status ?? 1);
