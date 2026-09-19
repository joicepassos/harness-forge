const assert = require("node:assert/strict");
const test = require("node:test");
const { checksumFor, platformFor, releaseBaseUrl } = require("../lib/release");

test("maps supported Node platforms to release archives", () => {
  assert.deepEqual(platformFor("win32", "x64"), { goos: "windows", goarch: "amd64", executable: "harnessforge.exe" });
  assert.deepEqual(platformFor("darwin", "arm64"), { goos: "darwin", goarch: "arm64", executable: "harnessforge" });
  assert.throws(() => platformFor("linux", "ia32"), /Unsupported platform/);
});

test("reads only the named archive checksum", () => {
  const archive = "harnessforge_1.0.2_linux_amd64.tar.gz";
  const digest = "a".repeat(64);
  assert.equal(checksumFor(`${digest}  ${archive}\n${"b".repeat(64)}  other.tar.gz\n`, archive), digest);
  assert.throws(() => checksumFor(`${digest}  other.tar.gz\n`, archive), /does not contain/);
});

test("accepts HTTPS release URLs and rejects arbitrary HTTP", () => {
  assert.equal(releaseBaseUrl("https://example.test/releases/"), "https://example.test/releases");
  assert.equal(releaseBaseUrl("http://127.0.0.1:8080/releases"), "http://127.0.0.1:8080/releases");
  assert.throws(() => releaseBaseUrl("http://example.test/releases"), /must use HTTPS/);
});
