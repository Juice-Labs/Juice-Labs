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

import * as influx from "@influxdata/influxdb-client";
import {spawn} from "child_process";
import { readFile } from "fs/promises";


function valueTimestamp(point: influx.Point, name: string, value: string) {
    point.timestamp(new Date(value));
}

function valueInt(point: influx.Point, name: string, value: string) {
    point.intField(name, parseInt(value, 10));
}

type MetricSubmit = (point: influx.Point, name: string, value: string) => void;

const QUERY : Record<string, [MetricSubmit, string]> = {
    "timestamp": [valueTimestamp, ""],
    "memory.used": [valueInt, "mb"],
    "utilization.gpu": [valueInt, "pct"],
    "utilization.memory": [valueInt, "pct"],
};
const QUERY_KEYS = Object.keys(QUERY);


function csvToPoint(line: string): influx.Point {
    const cols = line.split(",");
    if (cols.length !== QUERY_KEYS.length) {
        throw Error(`nvidia-smi unexpected results: ${line}`);
    }
    const point = new influx.Point("gpu");
    cols.forEach((value: string, idx) => {
        const name = QUERY_KEYS[idx];
        const [handler, units] = QUERY[name];
        const nameUnits = units ? `${name}_${units}` : name;
        handler(point, nameUnits, value);
    })
    return point;
}

export function getInfluxWrite(config: Config, hostname: string): influx.WriteApi {
    const bucket = 'agent'
    const client = new influx.InfluxDB({url: config.url, token: config.token})
    const writeApi = client.getWriteApi(config.org, bucket)

    writeApi.useDefaultTags({host: hostname});
    return writeApi;
}

export function reportStatus(writeApi: influx.WriteApi, status: Record<string, number>) {
    const point = new influx.Point("agent-status");
    for (const [key, value] of Object.entries(status)) {
        point.intField(key, value);
    }
    writeApi.writePoint(point);
    writeApi.flush();
}

export function startSMICollection(writeApi: influx.WriteApi) {

    const WAIT_SEC = "1";

    const q = "--query-gpu=" + QUERY_KEYS.join(",");

    const proc = spawn("nvidia-smi", ["--format=csv,noheader,nounits", "-l", WAIT_SEC, q]);

    proc.stdout.on("data", async (data: string) => {
        const splat = data.toString().split(/\r?\n/).filter(line => line.length > 0);
        for (const i of splat) {
            const point = csvToPoint(i);
            writeApi.writePoint(point);
        }
        writeApi.flush();
    });

    proc.on('exit', async (code) => {
        await writeApi.close();
        // TODO: what about non-failure exit?
        if (code) {
            throw Error(`nvidia-smi shutdown unexpectedly: ${code}`);
        }
    });
}
export class Config {
    constructor(public url: string, public org: string, public token: string) {}
}

export async function readConfig(fname: string) : Promise<Config> {
    const cfg = await readFile(fname);
    const cfgJson = JSON.parse(cfg.toString());

    function g(key: string): string {
        const v = cfgJson[key];
        if (typeof v !== 'string') {
            throw Error("missing value for ${key} in ${fname}");
        }
        return v;
    }

    const url = g("url");
    const org = g("org");
    const token = g("token");
    return new Config(url, org, token);
}