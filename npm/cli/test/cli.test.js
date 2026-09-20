const assert = require("node:assert/strict");
const test = require("node:test");
const { main } = require("../cli");

test("first use installs without an npm lifecycle script", async () => {
  const packageInfo = require("../package.json");
  assert.equal(packageInfo.scripts.postinstall, undefined);
  const events = [];
  const status = await main({
    executable: "test-binary",
    executableExists: () => false,
    installBinary: async () => { events.push("verified install"); },
    run: (executable, args) => { events.push(`${executable} ${args.join(" ")}`); return { status: 0 }; },
    args: ["version"],
    report: () => {}
  });
  assert.equal(status, 0);
  assert.deepEqual(events, ["verified install", "test-binary version"]);
});

test("an install failure prevents the binary from running", async () => {
  const messages = [];
  const status = await main({
    executable: "test-binary",
    executableExists: () => false,
    installBinary: async () => { throw new Error("checksum mismatch"); },
    run: () => { throw new Error("must not run"); },
    report: (message) => messages.push(message)
  });
  assert.equal(status, 1);
  assert.match(messages.join("\n"), /checksum mismatch/);
});

test("later invocations run the existing binary directly", async () => {
  let installs = 0;
  const status = await main({
    executable: "test-binary",
    executableExists: () => true,
    installBinary: async () => { installs += 1; },
    run: (_executable, args) => { assert.deepEqual(args, ["--help"]); return { status: 0 }; },
    args: ["--help"],
    report: () => {}
  });
  assert.equal(status, 0);
  assert.equal(installs, 0);
});
