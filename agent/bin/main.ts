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
import { v4 as uuidv4 } from "uuid";
import {promisify} from "util";
import {exec} from "child_process";

const execAsync = promisify(exec);

import * as CommandLine from "../src/commandline";
import * as Logging from "../src/logging";
import * as Package from "../src/version";

// Initialize the logging system here to allow imports to use it
Logging.configure(CommandLine.argv);

import * as Settings from "../src/settings";
import { Router, CreateOptions } from "../src/router";
import { postWithTimeout } from "../src/fetchWithTimeout";

async function main(): Promise<void> {
  if (!CommandLine.argv.nobanner) {
    Logging.always("Juice Agent, v%s", Package.version);
    Logging.always("Copyright 2021-2022 Juice Technologies, Inc.");
  }

  if (CommandLine.argv.launcher === undefined) {
    throw "launcher is undefined";
  }

  async function getLauncherVersion() {
    try {
      const { stdout, } = await execAsync(`${CommandLine.argv.launcher} --version`);
      return stdout;
    } catch (err) {
      return 'unknown';
    }
  }
  
  const launcherVersion = await getLauncherVersion();

  if (!CommandLine.argv.nobanner) {
    Logging.always("Launcher %s, v%s", CommandLine.argv.launcher, launcherVersion);
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
  const controller = CommandLine.argv.controller;
  Logging.always(`Hostname: ${hostname}`);

  if(controller) {
    Logging.always(`Controller: ${controller}`);
  }

  const startTime = new Date().getTime();

  var currentControllers: any[];

  const startupGraphics = require('../src/graphics.js').graphics;

  let data = await startupGraphics();

  currentControllers = data.controllers;

  let app = express();

  app.use(morgan("combined"));
  app.use(
    express.urlencoded({
      extended: true,
    })
  );
  app.use(express.json());

  const router = new Router(CommandLine.argv.launcher);

  async function Connect(res: any, port: number, options: CreateOptions & {id?: string}) {
    const maxClients = CommandLine.argv.maxClients;
    if (maxClients !== undefined && router.count >= maxClients) {
      res.status(529).send('too many clients, try again later');
      return
    }

    try {
      const id = options?.id ?? uuidv4();

      await router.create(id, CommandLine.argv.launcherArgs, options);
      res.status(200).json({ id: id });
    } catch (e) {
      Logging.error(e);
      res.status(500).send(e);
    }
  };

  // The interface for the agent's /connect call should match the controllers,
  // except the agent does not return an IP address
  app
    .get('/connect', async (req, res) => {
      const pcibus = req.query.pcibus as string;
      const id = req.query.id as string;

      let options = {};
      if(id !== undefined && id.length > 0) {
        const idCheck = /W+/g;
        if(!idCheck.test(id)) {
          res.status(400).send("invalid id");
          return;
        }
      }

      if(pcibus !== undefined && pcibus.length > 0) {
        let deviceUuid : string | undefined = undefined;
        currentControllers.every(controller => {
          const address = (controller.busAddress as string).toLowerCase();
          const targetAddress = pcibus.toLowerCase();

          if(address.includes(targetAddress))
          {
            deviceUuid = controller.uuid;
            return false;
          }

          return true;
        });

        if(deviceUuid === undefined)
        {
          // CUDA device not found at PCI bus address
          res.status(400).send("pci bus device not found");
          return;
        }

        options = {
          ...options,
          pcibus: pcibus,
          deviceUuid: deviceUuid
        };
      }

      Connect(res, req.socket.localPort, options);
    })
    .post('/:client_id/connect', async (req, res) => {
      const client_id = req.params.client_id;

      try {
        // Pause reads on the socket so that messages from the Juice client
        // aren't read by the agent.  There is a race between the forwarded
        // socket being written to by the client and the socket being closed
        // by Node.js.  This happens because the client proceeds once it
        // receives the HTTP response from Renderer_Win and Node.js closes
        // its handle to the forwarded socket only once the IPC ack is
        // received from Renderer_Win -- until the socket is closed Node.js
        // will happily to read and ignore any data that arrives on that
        // socket.
        req.socket.pause();

        if(!await router.forward(client_id, req.socket))
          res.status(500).send(`${client_id} not found`);
        // Just end the response as the socket is forwarded to Renderer_Win
      } catch (e) {
        Logging.error(e);
        res.status(500).send(e);
      }
    });

  function getStatus() {
    const uptimeMs = new Date().getTime() - startTime;
    return {
      agent_version: Package.version,
      version: launcherVersion,
      uptime_ms: uptimeMs,
      num_sessions: router.targets.length,
    }
  }

  app.get("/status", async (req, res) => {
    const graphics = require('../src/graphics.js').graphics;

    let data = await graphics();

    currentControllers = data.controllers;

    let result = {
      status: "ok",
      hostname: hostname,
      controllers: data.controllers,
      ...getStatus()
    };

    try {
      res.status(200).json(result);
    } catch (e) {
      Logging.error(e);
      res.status(500).send(e);
    }
  });

  process.on("SIGINT", async () => {
    await router.destroy();
    process.exit(0);
  });

  // Start the http servers
  const listener = await http.createServer(app);

  // Default to an epheral port, but allow it to be overridden
  listener.listen(CommandLine.argv.port, async () => {
    const addr = listener.address();

    // Not sure of a better way to do this
    assert(addr && typeof addr !== "string");
    console.log(`listening on ${addr.address}:${addr.port}`);

    // Start gpu status broadcast
    var dgram = require("dgram");
    var gpuBroadcastSocket = dgram.createSocket("udp4");
    gpuBroadcastSocket.bind(function () {
      gpuBroadcastSocket.setBroadcast(true);
    });

    // Get the list of broadcast addresses on active adapters on the system
    const ipaddr = require('ipaddr.js');
    const os = require("os");

    const broadcastAddresses: string[] = [];
    const interfaces = os.networkInterfaces();
    for (let iface in interfaces) {
      for (let i in interfaces[iface]) {
        const f = interfaces[iface][i];
        if (f.family === "IPv4") {
          broadcastAddresses.push(ipaddr.IPv4.broadcastAddressFromCIDR(f.cidr).toString());
        }
      }
    }

    var nonce = 0;
    const host_uuid = uuidv4();
    
    const UDP_INTERVAL_MS = 1000;
    const FAIL_INTERVAL_MS = 5; /* * UDP_INTERVAL_MS; */
    const SUCCESS_INTERVAL_MS = 60 * 5; /* * UDP_INTERVAL_MS; */
    
    var controllerPingLast = 0;
    var currentGPUCount = 0;

    if(!CommandLine.argv.udpbroadcast)
    {
      Logging.info("GPU UDP update broadcast disabled.");
    }

    setInterval(function() {

      var si = require('../src/graphics.js');
      si.graphics().then((data: { controllers: any[]; }) => {

        // Fix the controller list.
        let updateControllers : any[] = [];

        currentControllers = data.controllers;

        data.controllers.forEach(cont => {
          if(cont.vram > 512)
          {
            updateControllers.push(cont);
          }
        });

        var gpu: {[k: string]: any} = {};
        gpu.hostname = hostname;
        gpu.port = CommandLine.argv.port;
        gpu.uuid = host_uuid;
        gpu.action = "UPDATE";
        gpu.nonce = nonce;
        gpu.gpu_count = updateControllers.length;
        gpu.data = updateControllers;


        if(CommandLine.argv.udpbroadcast)
        {
          var jsonGpuData = JSON.stringify(gpu);
          var message = Buffer.from(jsonGpuData);
  
          // For each adapter, broadcast the packet. Needed because
          // on Windows the OS will not do this for you. Linux does,
          // however.
          broadcastAddresses.forEach(address => {
            gpuBroadcastSocket.send(message, 0, message.length, CommandLine.argv.port, address); 
          });
        }

        if (controller !== undefined) 
        {
          if((controllerPingLast <= 0) || (currentGPUCount != gpu.gpu_count))
          {
            controllerPingLast = SUCCESS_INTERVAL_MS;

              const controllerUrl = new URL(controller);
              const pingUrl = new URL("/ping", controllerUrl);
          
              try {
                postWithTimeout(pingUrl, gpu);
              } catch (err) {
                controllerPingLast = FAIL_INTERVAL_MS;
              }
          }

          --controllerPingLast;
        }

        currentGPUCount = gpu.gpu_count;

        nonce++;
      });
    }, UDP_INTERVAL_MS);

    if (controller !== undefined) 
    {
      process.on('SIGTERM', () => {
        var gpu: {[k: string]: any} = {};
        gpu.hostname = hostname;
        gpu.port = CommandLine.argv.port;
        gpu.uuid = host_uuid;
        gpu.action = "UPDATE";
        gpu.nonce = nonce;
        gpu.gpu_count = 0;
        gpu.data = [];

        const controllerUrl = new URL(controller);
        const pingUrl = new URL("/ping", controllerUrl);
    
        try {
          postWithTimeout(pingUrl, gpu);
        } catch (err) {
        }
      });
    }
  });
}

main()
  .catch((e) => {
    console.error(e);
    process.exit(1);
  });
