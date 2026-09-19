const path = require("node:path");
const packageInfo = require("../package.json");
const { install } = require("../lib/release");

install(path.resolve(__dirname, ".."), packageInfo.version)
  .then(() => console.log(`Installed HarnessForge ${packageInfo.version}.`))
  .catch((error) => {
    console.error(`HarnessForge installation failed: ${error.message}`);
    process.exitCode = 1;
  });
