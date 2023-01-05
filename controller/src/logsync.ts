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

import path from "path";
import * as fs from "fs/promises";
import express from "express";

import { mkdirSync } from "fs";
const logDir = path.resolve("log");

try {
  mkdirSync(logDir);
} catch (err) {}

function getAbsLogPath(log: string): string {
  const abspath = path.resolve(logDir, log);
  if (!abspath.startsWith(logDir)) {
    throw Error("bad path traversal for " + log);
  }
  return abspath;
}

async function getLogSize(log: string): Promise<number> {
  try {
    const abs = getAbsLogPath(log);
    const stats = await fs.stat(abs);
    return stats.size;
  } catch (err) {
    if (err.code === "ENOENT") {
      return 0;
    } else {
      throw err;
    }
  }
}

async function updateLog(log: string, data: Buffer, offset: number) {
  const abslog = getAbsLogPath(log);

  let fd;
  try {
    try {
      // "'r+': Open file for reading and writing. An exception occurs if the file does not exist."
      fd = await fs.open(abslog, "r+");
    } catch (err) {
      if (err.code == "ENOENT") {
        // "Open file for reading and writing. The file is created
        // (if it does not exist) or truncated (if it exists)."
        fd = await fs.open(abslog, "w+");
      } else {
        throw err;
      }
    }

    return await fd.write(data, 0, data.length, offset);
  } finally {
    if (fd !== undefined) {
      fd.close();
    }
  }
}

export function addLogSync(app: express.Application) {
  // Fetch sync status of a number of log files
  app.post(
    "/logsync",
    async (
      req: express.Request,
      res: express.Response,
      next: express.NextFunction
    ) => {
      try {
        const logFile = req.body.logFile;
        const size = await getLogSize(logFile);
        res.status(200).json({ logSize: size });
      } catch (err) {
        return next(err);
      }
    }
  );

  app.post(
    "/logwrite",
    async (
      req: express.Request,
      res: express.Response,
      next: express.NextFunction
    ) => {
      const log = req.body.logFile;
      if (typeof log !== "string" || log.length === 0) {
        return res.status(400).send("bad logFile");
      }
      const offset = req.body.offset;
      if (typeof offset !== "number" || offset < 0) {
        return res.status(400).send("bad offset");
      }

      try {
        const data = Buffer.from(req.body.data, "utf-8");

        const csize = await getLogSize(log);
        if (offset !== csize) {
          // offset should almost always be == size.  < is harmless, but means something unexpected is happening
          // so raise an error here out of caution
          return res.status(400).send("offset gap: ${offset) vs ${csize}");
        }

        const ret = await updateLog(log, data, offset);
        return res
          .status(200)
          .json({ bytesWritten: ret.bytesWritten, length: data.length });
      } catch (err) {
        return next(err);
      }
    }
  );
}
