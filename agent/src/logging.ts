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

"use strict";

import winston from "winston";

export type Options = {
  quiet: boolean;
  verbose: number;
};

const ALWAYS = "always";
const ERROR = "error";
const WARNING = "warning";
const INFO = "info";
const DEBUG = "debug";

const DEFAULT_LEVEL = INFO;

const levels = [ALWAYS, ERROR, WARNING, INFO, DEBUG];

const levelsObj = levels.reduce((obj, entry, index) => {
  obj[entry] = index + 1;
  return obj;
}, Object()) as unknown as winston.config.AbstractConfigSetLevels;

const console = new winston.transports.Console({
  stderrLevels: [ERROR],
  consoleWarnLevels: [WARNING],
});

const logger = winston.createLogger({
  format: winston.format.combine(
    winston.format.splat(),
    winston.format.printf((info) => {
      if (info.level == ALWAYS) {
        return `${info.message}`;
      }

      const padding = (info.padding && info.padding[info.level]) || "";
      return `${info.level}:${padding} ${info.message}`;
    })
  ),
  transports: [console],
  level: DEFAULT_LEVEL,
  levels: levelsObj,
});

export function configure(options: Options) {
  let level = levels.indexOf(DEFAULT_LEVEL) + options.verbose;
  if (level >= levels.length)
  {
    level = levels.length - 1;
  }

  logger.silent = options.quiet;
  logger.level = levels[level];

  debug(`Logger message level set to ${logger.level}`)
}

export function always(message: any, ...splat: any[]) {
  logger.log("always", message, ...splat);
}
export function error(message: any, ...splat: any[]) {
  logger.log("error", message, ...splat);
}
export function warning(message: any, ...splat: any[]) {
  logger.log("warning", message, ...splat);
}
export function info(message: any, ...splat: any[]) {
  logger.log("info", message, ...splat);
}
export function debug(message: any, ...splat: any[]) {
  logger.log("debug", message, ...splat);
}
