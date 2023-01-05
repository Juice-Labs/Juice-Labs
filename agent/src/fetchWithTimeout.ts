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

import fetch from "node-fetch";
import AbortController from "abort-controller";

const TIMEOUT_MS = 5000;

async function fetchWithTimeout(
  url: string | URL,
  method: string,
  body?: any
): Promise<any> {
  const abort = new AbortController();
  const timeout = setTimeout(() => {
    abort.abort();
  }, TIMEOUT_MS);

  let bodyInfo = {};
  if (body !== undefined) {
    bodyInfo = {
      body: JSON.stringify(body),
      headers: {
        "Content-Type": "application/json",
      },
    };
  }

  const res = await fetch(url.toString(), {
    method,
    signal: abort.signal,
    ...bodyInfo,
  });

  if (!res.ok) {
    throw Error(res.statusText);
  }

  const payload = await res.json();

  clearTimeout(timeout);

  return payload;
}

export async function getWithTimeout(url: string | URL): Promise<any> {
  return await fetchWithTimeout(url, "GET", undefined);
}

export async function postWithTimeout(
  url: string | URL,
  body: any
): Promise<any> {
  try
  {
    return await fetchWithTimeout(url, "POST", body);
  }
  catch(err)
  {
    // If we don't catch the AbortError all is lost.
  }
}
