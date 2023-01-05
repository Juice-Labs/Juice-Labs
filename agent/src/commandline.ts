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

export type Options = {
  quiet: boolean;
  verbose: number;
  nobanner: boolean;

  udpbroadcast: boolean;

  port: number;
  ip: string;

  launcher?: string;
  launcherArgs?: string[]
  controller?: string;
  influxConfig?: string;
  maxClients?: number;
  roundrobin: boolean;
  timeout: number
};

var argsParser = require("yargs/yargs")(process.argv.slice(2))
  .config()
  .option("quiet", {
    alias: "q",
    description: "Prevents all output",
    type: "boolean",
    default: false,
  })
  .option("controller", {
    description: "URL of controller",
    type: "string",
  })
  .option("influx-config", {
    alias: "influxConfig",
    description: "URL of influxdb host",
    type: "string",
  })
  .option("verbose", {
    alias: "v",
    description: "Increases the verbosity level, defaults to errors only",
    type: "count",
  })
  .option("nobanner", {
    description: "Prevents the output of the application banner",
    type: "boolean",
    default: false,
  })
  .option("ip", {
    description: "IP address to bind",
    type: "string",
    default: "0.0.0.0",
  })
  .option("port", {
    alias: "p",
    description: "Port to bind",
    type: "number"
  })
  .option("launcher", {
    description: "Specifies the path to the binary to launch",
    type: "string",
  })
  .option("max-clients", {
    alias: "maxClients",
    description: "maximum number of clients allowed",
    type: "number",
  })
  .option("timeout_ms", {
    alias: "timeout",
    description: "maximum timeout waiting for Renderer_Win to start, default 1000ms",
    type: "number",
    default: 5000
  })
  .help()
  .alias("help", "h").argv;

export const argv: Options = argsParser
argv.launcherArgs = argsParser._
