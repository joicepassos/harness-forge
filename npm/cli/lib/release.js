const crypto = require("node:crypto");
const fs = require("node:fs");
const fsp = require("node:fs/promises");
const http = require("node:http");
const https = require("node:https");
const os = require("node:os");
const path = require("node:path");
const { execFileSync } = require("node:child_process");

const MAX_MANIFEST_BYTES = 1024 * 1024;
const MAX_ARCHIVE_BYTES = 64 * 1024 * 1024;
const DEFAULT_RELEASE_BASE_URL = "https://github.com/joicepassos/harness-forge/releases/download";
const VERSION_PATTERN = /^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$/;

function platformFor(nodePlatform = process.platform, nodeArch = process.arch) {
  const platforms = {
    darwin: { x64: ["darwin", "amd64", "harnessforge"], arm64: ["darwin", "arm64", "harnessforge"] },
    linux: { x64: ["linux", "amd64", "harnessforge"], arm64: ["linux", "arm64", "harnessforge"] },
    win32: { x64: ["windows", "amd64", "harnessforge.exe"] }
  };
  const match = platforms[nodePlatform] && platforms[nodePlatform][nodeArch];
  if (!match) throw new Error(`Unsupported platform: ${nodePlatform}/${nodeArch}.`);
  const [goos, goarch, executable] = match;
  return { goos, goarch, executable };
}

function releaseBaseUrl(value = process.env.HARNESSFORGE_RELEASE_BASE_URL || DEFAULT_RELEASE_BASE_URL) {
  const url = new URL(value);
  const localTestServer = (url.hostname === "127.0.0.1" || url.hostname === "localhost") && url.protocol === "http:";
  if (url.protocol !== "https:" && !localTestServer) throw new Error("Release base URL must use HTTPS.");
  return url.href.replace(/\/$/, "");
}

function checksumFor(manifest, archive) {
  const expression = new RegExp(`^([0-9a-fA-F]{64})\\s+\\*?${archive.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")}$`, "m");
  const match = expression.exec(manifest);
  if (!match) throw new Error(`Checksum manifest does not contain ${archive}.`);
  return match[1].toLowerCase();
}

function binaryPath(packageRoot, platform = platformFor()) {
  return path.join(packageRoot, "native", `${platform.goos}-${platform.goarch}`, platform.executable);
}

function allowedUrl(url) {
  const localTestServer = (url.hostname === "127.0.0.1" || url.hostname === "localhost") && url.protocol === "http:";
  return url.protocol === "https:" || localTestServer;
}

function download(urlText, destination, maxBytes, redirects = 0) {
  if (redirects > 5) return Promise.reject(new Error("Too many release download redirects."));
  const url = new URL(urlText);
  if (!allowedUrl(url)) return Promise.reject(new Error("Release download must use HTTPS."));
  const client = url.protocol === "https:" ? https : http;
  return new Promise((resolve, reject) => {
    const request = client.get(url, { headers: { "User-Agent": "harnessforge-npm-installer" } }, (response) => {
      if ([301, 302, 303, 307, 308].includes(response.statusCode)) {
        const location = response.headers.location;
        response.resume();
        if (!location) return reject(new Error("Release download redirected without a location."));
        return resolve(download(new URL(location, url).href, destination, maxBytes, redirects + 1));
      }
      if (response.statusCode !== 200) {
        response.resume();
        return reject(new Error(`Release download failed with HTTP ${response.statusCode}.`));
      }
      const length = Number(response.headers["content-length"] || 0);
      if (length > maxBytes) {
        response.resume();
        return reject(new Error("Release download exceeds the supported size limit."));
      }
      const output = fs.createWriteStream(destination, { flags: "wx", mode: 0o600 });
      let received = 0;
      response.on("data", (chunk) => {
        received += chunk.length;
        if (received > maxBytes) response.destroy(new Error("Release download exceeds the supported size limit."));
      });
      response.on("error", reject);
      output.on("error", reject);
      output.on("finish", () => output.close(resolve));
      response.pipe(output);
    });
    request.setTimeout(30_000, () => request.destroy(new Error("Release download timed out.")));
    request.on("error", reject);
  });
}

async function install(packageRoot, version) {
  if (!VERSION_PATTERN.test(version) || version === "0.0.0-development") {
    throw new Error("The npm package must have a released semantic version.");
  }
  const platform = platformFor();
  const baseUrl = releaseBaseUrl();
  const archive = `harnessforge_${version}_${platform.goos}_${platform.goarch}.tar.gz`;
  const checksums = `harnessforge_${version}_checksums.txt`;
  const releaseUrl = `${baseUrl}/v${version}`;
  const temporaryRoot = await fsp.mkdtemp(path.join(os.tmpdir(), "harnessforge-npm-"));
  const archivePath = path.join(temporaryRoot, archive);
  const checksumPath = path.join(temporaryRoot, checksums);
  const extractDir = path.join(temporaryRoot, "extracted");
  try {
    await download(`${releaseUrl}/${checksums}`, checksumPath, MAX_MANIFEST_BYTES);
    await download(`${releaseUrl}/${archive}`, archivePath, MAX_ARCHIVE_BYTES);
    const expected = checksumFor(await fsp.readFile(checksumPath, "utf8"), archive);
    const actual = crypto.createHash("sha256").update(await fsp.readFile(archivePath)).digest("hex");
    if (actual !== expected) throw new Error(`Checksum mismatch for ${archive}; refusing to install.`);
    const entries = execFileSync("tar", ["-tzf", archivePath], { encoding: "utf8" }).split(/\r?\n/).filter(Boolean);
    if (entries.length !== 1 || entries[0] !== platform.executable) throw new Error("Release archive contains unexpected paths.");
    await fsp.mkdir(extractDir);
    execFileSync("tar", ["-xzf", archivePath, "-C", extractDir, platform.executable], { stdio: "ignore" });
    const source = path.join(extractDir, platform.executable);
    const sourceInfo = await fsp.lstat(source);
    if (!sourceInfo.isFile() || sourceInfo.isSymbolicLink()) throw new Error("Release archive has no regular HarnessForge executable.");
    const destination = binaryPath(packageRoot, platform);
    await fsp.mkdir(path.dirname(destination), { recursive: true });
    const staged = `${destination}.${process.pid}.tmp`;
    await fsp.copyFile(source, staged);
    if (platform.goos !== "windows") await fsp.chmod(staged, 0o755);
    await fsp.rename(staged, destination);
  } finally {
    await fsp.rm(temporaryRoot, { recursive: true, force: true });
  }
}

module.exports = { binaryPath, checksumFor, install, platformFor, releaseBaseUrl };
