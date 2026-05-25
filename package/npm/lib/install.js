"use strict";

const fs = require("node:fs");
const crypto = require("node:crypto");
const http = require("node:http");
const https = require("node:https");
const path = require("node:path");
const { fileURLToPath } = require("node:url");
const { assetName, binaryPath, checksumsURL, downloadURL, releaseVersion } = require("./platform");

if (process.env.SCAFLD_SKIP_DOWNLOAD === "1") {
  console.log("scafld: skipping native binary download because SCAFLD_SKIP_DOWNLOAD=1");
  process.exit(0);
}

if (releaseVersion() === "0.0.0" && !process.env.SCAFLD_INSTALL_BASE_URL) {
  console.log("scafld: development package version detected; skipping native binary download");
  process.exit(0);
}

const destination = binaryPath();
fs.mkdirSync(path.dirname(destination), { recursive: true });

install().catch((err) => {
  console.error(`scafld: failed to install native binary: ${err.message}`);
  process.exit(1);
});

async function install() {
  const expected = await expectedChecksum();
  await download(downloadURL(), destination, 0);
  const actual = sha256File(destination);
  if (actual !== expected) {
    fs.rmSync(destination, { force: true });
    throw new Error(`checksum mismatch for ${assetName()}: expected ${expected}, got ${actual}`);
  }
  console.log(`scafld: verified ${assetName()} sha256 ${actual}`);
}

async function expectedChecksum() {
  const text = await fetchText(checksumsURL(), 0);
  const selected = assetName();
  for (const line of text.split(/\r?\n/)) {
    const match = line.trim().match(/^([0-9a-fA-F]{64})\s+\*?(.+)$/);
    if (match && match[2] === selected) {
      return match[1].toLowerCase();
    }
  }
  throw new Error(`checksums.txt does not contain ${selected}`);
}

function sha256File(file) {
  return crypto.createHash("sha256").update(fs.readFileSync(file)).digest("hex");
}

function download(url, destination, redirects) {
  if (redirects > 5) {
    return Promise.reject(new Error("too many redirects"));
  }
  const parsed = new URL(url);
  if (parsed.protocol === "file:") {
    return copyFileURL(parsed, destination);
  }

  return new Promise((resolve, reject) => {
    const tmp = `${destination}.tmp-${process.pid}`;
    const request = clientFor(parsed).get(url, (response) => {
      if (response.statusCode >= 300 && response.statusCode < 400 && response.headers.location) {
        response.resume();
        resolve(download(response.headers.location, destination, redirects + 1));
        return;
      }

      if (response.statusCode !== 200) {
        response.resume();
        reject(new Error(`GET ${url} returned HTTP ${response.statusCode}`));
        return;
      }

      const file = fs.createWriteStream(tmp, { mode: 0o755 });
      response.pipe(file);
      file.on("finish", () => {
        file.close((err) => {
          if (err) {
            reject(err);
            return;
          }
          fs.chmodSync(tmp, 0o755);
          fs.renameSync(tmp, destination);
          console.log(`scafld: installed native binary ${destination}`);
          resolve();
        });
      });
      file.on("error", (err) => {
        fs.rmSync(tmp, { force: true });
        reject(err);
      });
    });
    request.on("error", (err) => {
      fs.rmSync(tmp, { force: true });
      reject(err);
    });
  });
}

function fetchText(url, redirects) {
  if (redirects > 5) {
    return Promise.reject(new Error("too many redirects"));
  }
  const parsed = new URL(url);
  if (parsed.protocol === "file:") {
    return fs.promises.readFile(fileURLToPath(parsed), "utf8");
  }

  return new Promise((resolve, reject) => {
    const request = clientFor(parsed).get(url, (response) => {
      if (response.statusCode >= 300 && response.statusCode < 400 && response.headers.location) {
        response.resume();
        resolve(fetchText(response.headers.location, redirects + 1));
        return;
      }
      if (response.statusCode !== 200) {
        response.resume();
        reject(new Error(`GET ${url} returned HTTP ${response.statusCode}`));
        return;
      }
      let body = "";
      response.setEncoding("utf8");
      response.on("data", (chunk) => {
        body += chunk;
      });
      response.on("end", () => resolve(body));
    });
    request.on("error", reject);
  });
}

async function copyFileURL(parsed, destination) {
  const tmp = `${destination}.tmp-${process.pid}`;
  try {
    await fs.promises.copyFile(fileURLToPath(parsed), tmp);
    await fs.promises.chmod(tmp, 0o755);
    await fs.promises.rename(tmp, destination);
    console.log(`scafld: installed native binary ${destination}`);
  } catch (err) {
    fs.rmSync(tmp, { force: true });
    throw err;
  }
}

function clientFor(parsed) {
  if (parsed.protocol === "https:") {
    return https;
  }
  if (parsed.protocol === "http:") {
    return http;
  }
  throw new Error(`unsupported URL protocol: ${parsed.protocol}`);
}
