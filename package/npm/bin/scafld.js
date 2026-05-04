#!/usr/bin/env node
"use strict";

const { spawn } = require("node:child_process");
const { binaryPath } = require("../lib/platform");

const binary = process.env.SCAFLD_BINARY || binaryPath();
const child = spawn(binary, process.argv.slice(2), { stdio: "inherit" });

child.on("error", (err) => {
  console.error(`scafld: unable to start native binary at ${binary}`);
  console.error(`scafld: ${err.message}`);
  console.error("scafld: reinstall the package or set SCAFLD_BINARY to a working binary");
  process.exit(127);
});

child.on("exit", (code, signal) => {
  if (signal) {
    process.kill(process.pid, signal);
    return;
  }
  process.exit(code == null ? 1 : code);
});
