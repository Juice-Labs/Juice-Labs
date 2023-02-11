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

import { ChildProcess } from "child_process";
import { EventEmitter } from "events";

import * as Logging from "./logging";
import * as Settings from "./settings";

export class Target extends EventEmitter {
  process: ChildProcess;
  client_uuid: string;
  logFile: string;

  constructor(target: ChildProcess, client_uuid: string, logFile: string) {
    super();

    this.process = target;
    this.client_uuid = client_uuid;
    this.logFile = logFile;

    // Set up some event listeners on the target
    this.process.on("exit", (code: number, signal: string) => {
      this.emit("exit", code, signal);
    });
    this.process.on("error", (err) => {
      Logging.error(err);
      this.emit("error", err);
    });
    if (this.process.stdout) {
        this.process.stdout.on("data", (data) => {
            Logging.info(`${client_uuid}: ${data.toString().trim()}`);
        });
    }
    if (this.process.stderr) {
        this.process.stderr.on("data", (data) => {
            Logging.error(`${client_uuid}: ${data.toString().trim()}`);
        });
    }
  }

  async destroy() {
    // Destroy the target process
    return new Promise<void>((resolve) => {
      if (!this.process.killed) {
        Logging.debug("Giving the target process a chance to exit nicely");
        this.process.kill("SIGINT");

        // Set a timeout
        const timeout = setTimeout(() => {
          if (!this.process.killed) {
            Logging.debug("Target process is still running, terminating");
            this.process.kill();
          }
        }, Settings.launchDestroySIGINTTimeout);

        const onExit = () => {
          Logging.debug("Target %s terminated", this.client_uuid);
          clearTimeout(timeout);
          resolve();
        };
        this.process.on("exit", onExit);
      } else {
        // Target is already dead, just resolve
        resolve();
      }
    });
  }
}
