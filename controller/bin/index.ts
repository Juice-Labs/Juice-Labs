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

import express from "express";
import morgan from "morgan";
import * as hoststore from "../src/hoststore_sqlite";
import * as pug from "pug";
import { addLogSync } from "../src/logsync";
import { getWithTimeout } from "../../agent/src/fetchWithTimeout";

function randomInteger(min : number, max : number) {
  return Math.floor(Math.random() * (max - min + 1)) + min;
}

const port = process.env.PORT || 8080;

const app = express();
app.use(morgan("combined"));
app.use(express.json());

app.get(
  "/connect",
  async (
    req: express.Request,
    res: express.Response,
    next: express.NextFunction
  ): Promise<void> => {
    try {
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
    } catch (err) {
      next(err);
    }
  }
);

app.post(
  "/ping",
  async (
    req: express.Request,
    res: express.Response,
    next: express.NextFunction
  ): Promise<void> => {
    try {
      // Strip local IP v6 prefix if it appears
      const host = req.ip.replace(/^(::ffff:)/, "");
      let port = req.body.port;
      req.body.ip = host;
      if (typeof port !== "number") {
        res.status(400).send("Missing port");
        return;
      }

      await hoststore.addHost(host, port, req.body.gpu_count, req.body);
      res.status(200).json({});
    } catch (err) {
      next(err);
    }
  }
);

app.get(
  "/statushr",
  async (
    req: express.Request,
    res: express.Response,
    next: express.NextFunction
  ) => {
    try {
      const statii = await hoststore.getHostState();
      res.status(200).send(pug.renderFile("view/status.pug", { statii }));
    } catch (err) {
      next(err);
    }
  }
);

app.get(
  "/status",
  async (
    req: express.Request,
    res: express.Response,
    next: express.NextFunction
  ) => {
    try {
      var hosts = [];
      const statii = await hoststore.getHostState();
      for(let i = 0; i < statii.length; ++i)
      {
        hosts.push(statii[i].agent.data);
      }
      res.status(200).json(hosts);
    } catch (err) {
      next(err);
    }
  }
);

addLogSync(app);

app.listen(port, () => {
  console.log(`Listening at http://localhost:${port}`);
});
