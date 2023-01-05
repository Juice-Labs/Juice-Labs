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

import { EventEmitter } from "events";
import dateformat from "dateformat";
import * as Logging from "./logging";
import { Target } from "./target";
import * as cp from "child_process";
import { Socket } from "net";

function getLogFileForClient(client_uuid: string): string {
  const now = new Date();
  const date = dateformat(now, "isoDate", true);
  const time = dateformat(now, "HHMMss", true);
  const logPath = `log/${date}-${time}-${client_uuid}.log`;
  return logPath;
}

export type CreateOptions = {
  pcibus?: string;
  deviceUuid?: string;
};

export class Router extends EventEmitter implements Router {
  binary: string;
  targets: Record<string, Target>;
  count: number;

  constructor(binary: string) {
    super();
    this.binary = binary;
    this.targets = {};
    this.count = 0;
  }

  async destroy() {
    Logging.debug("Shutting down Router");
    await Promise.all(Object.entries(this.targets).map(([_, value]) => value.destroy()));
    this.count = 0;
  }

  async create(client_uuid: string, args?: string[], options?: CreateOptions): Promise<void> {
    // Launch the target process
    Logging.debug("Spawning target process...");

    const logFile = getLogFileForClient(client_uuid);

    var spawn_args: string[] = [
      '--ipc',
      '--id', client_uuid,
      '--log_file', logFile
    ]

    let targetEnv: NodeJS.ProcessEnv | undefined = undefined;

    if(options !== undefined) {
      if(options.pcibus && options.pcibus.length > 0)
      {
        spawn_args.push('--pcibus');
        spawn_args.push(options.pcibus);
        Logging.info(`   selecting pcibus ${options.pcibus} with uuid ${options.deviceUuid}`);

        targetEnv = { CUDA_VISIBLE_DEVICES: options.deviceUuid };
      }
    }

    if(args !== undefined) {
      spawn_args = spawn_args.concat(...args)
    }

    const child = cp.spawn(this.binary, spawn_args, {
      // Take from childprocess code https://github.com/nodejs/node/blob/v16.x/lib/child_process.js#L151
      // Allowing us to open an ipc channel like childprocess.fork() does so we can send
      // sockets to the Renderer_Win.
      stdio: ['pipe', 'pipe', 'pipe', 'ipc'],
      serialization: 'json',
      env: targetEnv
    });

    const target = new Target(child, client_uuid, logFile);
    target.on("exit", (code: number, signal: string) => {
      if(code) {
        if(code == 0) {
          Logging.info(`${client_uuid} exited code ${code}`);
        }
        else {
          Logging.error(`${client_uuid} exited code ${code}`);
          if(code == 0xC0000135) {
            Logging.error('Application exited with STATUS_DLL_NOT_FOUND error. Please install the redistributables referenced at https://github.com/Juice-Labs/Juice-Labs.');
          }
        }
      }
      else {
        Logging.error(`${client_uuid} exited signal ${signal}`);
      }

      delete this.targets[client_uuid];
      this.count -= 1;
    });

    this.targets[client_uuid] = target;
    this.count += 1;
  }

  async forward(client_uuid: string, socket: Socket): Promise<boolean> {
    if(!(client_uuid in this.targets)) {
      Logging.error(`${client_uuid} is not a valid target`);
      return false;
    }

    try {
      return this.targets[client_uuid].process.send('connection', socket);
    }
    catch(e) {
      Logging.error(e);
      return false;
    }
  }
}
