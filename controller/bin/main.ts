// Copyright (c) 2023 Juice Technologies, Inc
// 
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
// 
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
// 
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

import * as fs from "fs/promises";
import * as http from "http";
import express from "express";
import morgan from "morgan";
import assert from "assert";
import {promisify} from "util";
import {exec} from "child_process";

const execAsync = promisify(exec);

import * as CommandLine from "../src/commandline";
import * as Logging from "../src/logging";
import * as Package from "../src/version";

// Initialize the logging system here to allow imports to use it
Logging.configure(CommandLine.argv);

import * as Settings from "../src/settings";
import * as hoststore from "../src/hoststore_sqlite";
import { getWithTimeout } from "../src/fetchWithTimeout";

function randomInteger(min : number, max : number) {
  return Math.floor(Math.random() * (max - min + 1)) + min;
}

async function main(): Promise<void> {
  if (!CommandLine.argv.nobanner) {
    Logging.always("Juice Controller, v%s", Package.version);
    Logging.always("Copyright 2021-2023 Juice Technologies, Inc.");
  }

  try {
    await fs.mkdir(Settings.logDir);
  } catch (err) {
    if ((err as NodeJS.ErrnoException).code !== "EEXIST") {
      throw err;
    }
  }

  async function getHostname() {
    const hostProcess = await execAsync("hostname");
    const lines = hostProcess.stdout.split(/\r?\n/).filter(line => line.length > 0);
    if (lines.length !== 1) {
      throw Error(`unexpected output from hostname: ${hostProcess.stdout}`);
    }
    return lines[0];
  }
  
  const hostname = await getHostname();
  Logging.always(`Hostname: ${hostname}`);

  const startTime = new Date().getTime();

  let app = express();

  app.use(morgan("combined"));
  app.use(
    express.urlencoded({
      extended: true,
    })
  );
  app.use(express.json());

  app.get('/connect', async (req, res) => {
      const good = await hoststore.getOnlineHosts();
      if (good.length === 0) {
        res.status(500).json({ msg: "No valid hosts" });
      } else {
        let agentNum = randomInteger(0, good.length - 1);
        const randAgent = good[agentNum];
        const resp = await getWithTimeout(hoststore.getURL(randAgent, "/connect"));

        const ports = resp["port"];
        const client_uuid = resp["id"];
        if (!ports || !client_uuid) {
          res
            .status(500)
            .send(`bad response from agent: ${JSON.stringify(resp)}`);
        } else {
          res.status(200).json({
            id: client_uuid,
            host: randAgent.url.hostname,
            port: ports,
          });
        }
      }
    });

  app.get("/status", async (req, res) => {
    try {
      var hosts = [];
      const statii = await hoststore.getHostState();
      for(let i = 0; i < statii.length; ++i)
      {
        hosts.push(statii[i].agent.data);
      }

      res.status(200).json({
        status: "ok",
        version: Package.version,
        uptime_ms: new Date().getTime() - startTime,
        hosts: hosts
      });
    } catch (e) {
      Logging.error(e);
      res.status(500).send(e);
    }
  });

  app.post("/ping", async (req, res) => {
      try {
        // Strip local IP v6 prefix if it appears
        const host = req.ip.replace(/^(::ffff:)/, "");
        let port = req.body.port;
        req.body.ip = host;
        if (typeof port !== "number") {
          res.status(400).send("Missing port");
        }
        else {
          await hoststore.addHost(host, port, req.body.gpu_count, req.body);
          res.status(200).json({});
        }
      } catch (e) {
        Logging.error(e);
        res.status(500).send(e);
      }
    }
  );

  process.on("SIGINT", async () => {
    process.exit(0);
  });

  // Start the http servers
  const listener = await http.createServer(app);

  // Default to an epheral port, but allow it to be overridden
  listener.listen(CommandLine.argv.port, async () => {
    const addr = listener.address();

    // Not sure of a better way to do this
    assert(addr && typeof addr !== "string");
    console.log(`Listening on ${addr.address}:${addr.port}`);
  });
}

main()
  .catch((e) => {
    console.error(e);
    process.exit(1);
  });
