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

import * as Logging from "./logging";
import { postWithTimeout } from "./fetchWithTimeout";

const FAIL_INTERVAL_MS = 5 * 1000;
const SUCCESS_INTERVAL_MS = 60 * 5 * 1000;

export function pingControllerLoop(controller: URL, localPort: number) {
  const pingUrl = new URL("/ping", controller);

  async function doPing() {
    try {
      await postWithTimeout(pingUrl, { port: localPort });
      Logging.debug(`Controller ping of ${pingUrl} complete`);
      setTimeout(doPing, SUCCESS_INTERVAL_MS);
    } catch (err) {
      Logging.error("Controller ping failed: %s", err);
      setTimeout(doPing, FAIL_INTERVAL_MS);
    }
  }

  doPing();
}
