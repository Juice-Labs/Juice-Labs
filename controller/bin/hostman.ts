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

import { execFile } from "child_process";
import { ExecFileException } from "node:child_process";
import * as hoststore from "../src/hoststore_sqlite";

/*
TODO:
how to get terraform to start non-running instance

*/
function execShellCommand(cmd: string, args: string[]): Promise<string> {
  return new Promise((resolve, reject) => {
    execFile(
      cmd,
      args,
      undefined,
      (
        error: ExecFileException | null,
        stdout: string | Buffer,
        stderr: string | Buffer
      ) => {
        if (error) {
          reject(error);
        } else {
          resolve(stdout.toString());
        }
      }
    );
  });
}

async function terraform(args: string[]): Promise<string> {
  return await execShellCommand(
    "../ops/bin/terraform",
    ["-chdir=../ops"].concat(args)
  );
}

async function refresh(): Promise<string[]> {
  await terraform(["refresh"]);
  const tfstate = JSON.parse(await terraform(["show", "-json"]));
  const resource =
    ((tfstate["values"] || {})["root_module"] || {})["resources"] || [];
  const values = resource.map((r: any) => r["values"]);
  const running = values.filter((e: any) => e["instance_state"] === "running");
  return running.map((e: any) => e["public_dns"]);
}

async function syncDb(): Promise<void> {
  const hosts = await refresh();
  await hoststore.updateHosts(hosts);
  console.log("Found", hosts.length, "hosts:");
  for (let i of hosts) {
    console.log("  ", i);
  }
}

function usage(): number {
  console.error("hostman [sync|list]\n");
  return 1;
}

async function main(): Promise<number> {
  const op = process.argv[2];
  if (op === "sync") {
    await syncDb();
  } else if (op === "list") {
    const state = await hoststore.getHostState();
    for (let i of state) {
      console.log(i.isAlive ? "alive" : "dead", ":", i.agent.toString());
    }
  } else {
    return usage();
  }
  return 0;
}

main()
  .then((ret) => {
    process.exit(ret);
  })
  .catch((err) => {
    console.error(err);
    process.exit(1);
  });
