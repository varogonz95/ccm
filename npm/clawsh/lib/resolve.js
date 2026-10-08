// Finds the clawsh binary for this platform, in order:
//   1. CLAWSH_BINARY_PATH, if set;
//   2. the installed optional dependency clawsh-<os>-<cpu>;
//   3. a copy downloaded earlier into the user cache;
//   4. a fresh download of that same package from the npm registry,
//      checked against the registry's sha512 integrity before use.
// Step 4 covers installs that skip optional dependencies (pnpm/bun policies,
// --omit=optional, a lockfile made on another OS).
"use strict";

const crypto = require("node:crypto");
const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");
const zlib = require("node:zlib");

const VERSION = require("../package.json").version;
const PLATFORMS = {
  "darwin-arm64": true, "darwin-x64": true,
  "linux-arm64": true, "linux-x64": true,
  "win32-arm64": true, "win32-x64": true,
};

function platformPackage(platform = process.platform, arch = process.arch) {
  const key = `${platform}-${arch}`;
  if (!PLATFORMS[key]) {
    throw new Error(`no prebuilt binary for ${key}; build from source: https://github.com/varogonz95/clawsh`);
  }
  return `clawsh-${key}`;
}

function binName(platform = process.platform) {
  return platform === "win32" ? "clawsh.exe" : "clawsh";
}

function cacheDir() {
  if (process.env.CLAWSH_CACHE_DIR) return process.env.CLAWSH_CACHE_DIR;
  const home = os.homedir();
  if (process.platform === "win32") return path.join(process.env.LOCALAPPDATA || path.join(home, "AppData", "Local"), "clawsh", "cache");
  if (process.platform === "darwin") return path.join(home, "Library", "Caches", "clawsh");
  return path.join(process.env.XDG_CACHE_HOME || path.join(home, ".cache"), "clawsh");
}

function registryURL() {
  // npm and npx export the configured registry to the scripts they run.
  const r = process.env.npm_config_registry || "https://registry.npmjs.org/";
  return r.endsWith("/") ? r : r + "/";
}

// Returns the bytes of package/bin/<name> from an npm package tarball (.tgz).
function extractFromTarball(tgz, name) {
  const tar = zlib.gunzipSync(tgz);
  const want = `package/bin/${name}`;
  for (let off = 0; off + 512 <= tar.length; ) {
    const header = tar.subarray(off, off + 512);
    if (header.every((b) => b === 0)) break; // end of archive
    const field = (start, len) => header.toString("utf8", start, start + len).replace(/\0.*$/s, "");
    const size = parseInt(field(124, 12).trim() || "0", 8);
    const prefix = field(345, 155);
    const entry = prefix ? `${prefix}/${field(0, 100)}` : field(0, 100);
    const type = String.fromCharCode(header[156] || 48);
    const body = off + 512;
    if (entry === want && (type === "0" || type === "\0")) return tar.subarray(body, body + size);
    off = body + Math.ceil(size / 512) * 512;
  }
  throw new Error(`${want} not found in package tarball`);
}

function verifyIntegrity(buf, integrity) {
  const [algo, expected] = String(integrity || "").split("-", 2);
  if (algo !== "sha512" || !expected) throw new Error(`unsupported integrity value: ${integrity}`);
  const actual = crypto.createHash("sha512").update(buf).digest("base64");
  if (actual !== expected) throw new Error("downloaded package failed its sha512 integrity check");
}

async function fetchOK(url, what) {
  let res;
  try {
    res = await fetch(url);
  } catch (err) {
    throw new Error(`could not download ${what} from ${url}: ${err.cause?.message || err.message}`);
  }
  if (!res.ok) throw new Error(`could not download ${what} from ${url}: HTTP ${res.status}`);
  return res;
}

async function download(pkg, dest) {
  const meta = await (await fetchOK(`${registryURL()}${pkg}/${VERSION}`, `${pkg}@${VERSION} metadata`)).json();
  const tgz = Buffer.from(await (await fetchOK(meta.dist.tarball, `${pkg}@${VERSION}`)).arrayBuffer());
  verifyIntegrity(tgz, meta.dist.integrity);
  const bin = extractFromTarball(tgz, path.basename(dest));
  fs.mkdirSync(path.dirname(dest), { recursive: true });
  const tmp = `${dest}.${process.pid}.tmp`;
  fs.writeFileSync(tmp, bin, { mode: 0o755 });
  fs.renameSync(tmp, dest); // atomic, so a concurrent first run never sees half a file
}

async function resolveBinary() {
  if (process.env.CLAWSH_BINARY_PATH) return process.env.CLAWSH_BINARY_PATH;

  const pkg = platformPackage();
  const name = binName();
  try {
    return require.resolve(`${pkg}/bin/${name}`);
  } catch {
    // optional dependency not installed; fall through
  }

  const cached = path.join(cacheDir(), VERSION, pkg, name);
  if (fs.existsSync(cached)) return cached;

  process.stderr.write(`clawsh: ${pkg} isn't installed; downloading it once from the npm registry...\n`);
  try {
    await download(pkg, cached);
  } catch (err) {
    throw new Error(`${err.message}\nInstall ${pkg}@${VERSION} yourself, or set CLAWSH_BINARY_PATH to a clawsh binary.`);
  }
  return cached;
}

module.exports = { resolveBinary, platformPackage, binName, extractFromTarball, verifyIntegrity, cacheDir };
