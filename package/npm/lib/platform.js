"use strict";

const os = require("node:os");
const path = require("node:path");

function target() {
  const goos = {
    darwin: "darwin",
    linux: "linux",
    win32: "windows"
  }[process.platform];
  const goarch = {
    x64: "amd64",
    arm64: "arm64"
  }[process.arch];

  if (!goos || !goarch) {
    throw new Error(`unsupported platform: ${process.platform}/${process.arch}`);
  }

  return { goos, goarch, ext: goos === "windows" ? ".exe" : "" };
}

function packageVersion() {
  return require("../package.json").version;
}

function releaseVersion() {
  return (process.env.SCAFLD_INSTALL_VERSION || packageVersion()).replace(/^v/, "");
}

function releaseTag() {
  return `v${releaseVersion()}`;
}

function assetName(version = releaseVersion()) {
  const t = target();
  return `scafld_${version.replace(/^v/, "")}_${t.goos}_${t.goarch}${t.ext}`;
}

function binaryPath() {
  return path.join(__dirname, "..", "vendor", `scafld${target().ext}`);
}

function repo() {
  return process.env.SCAFLD_GITHUB_REPOSITORY || "nilstate/scafld";
}

function downloadURL() {
  const base = process.env.SCAFLD_INSTALL_BASE_URL;
  if (base) {
    return `${base.replace(/\/$/, "")}/${assetName()}`;
  }
  return `https://github.com/${repo()}/releases/download/${releaseTag()}/${assetName()}`;
}

function checksumsURL() {
  const base = process.env.SCAFLD_INSTALL_BASE_URL;
  if (base) {
    return `${base.replace(/\/$/, "")}/checksums.txt`;
  }
  return `https://github.com/${repo()}/releases/download/${releaseTag()}/checksums.txt`;
}

module.exports = { assetName, binaryPath, checksumsURL, downloadURL, releaseTag, releaseVersion, target };
