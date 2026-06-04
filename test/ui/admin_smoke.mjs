import { spawn } from "node:child_process";
import { existsSync } from "node:fs";
import { setTimeout as sleep } from "node:timers/promises";

const baseURL = process.env.UI_BASE_URL || "http://localhost:18080";
const chromeBin = resolveChromeBin();
const debugPort = process.env.CHROME_DEBUG_PORT || "9223";
const userDataDir = process.env.CHROME_USER_DATA_DIR || "/tmp/url-shortener-chrome-smoke";

const chrome = spawn(chromeBin, [
  "--headless=new",
  "--disable-gpu",
  "--no-sandbox",
  `--user-data-dir=${userDataDir}`,
  `--remote-debugging-port=${debugPort}`,
  "about:blank",
], {
  stdio: ["ignore", "pipe", "pipe"],
});

chrome.stderr.on("data", (chunk) => {
  if (process.env.CHROME_DEBUG) process.stderr.write(chunk);
});

chrome.on("error", (err) => {
  console.error(`failed to start Chrome from ${chromeBin}: ${err.message}`);
});

try {
  const wsURL = await waitForWebSocketURL();
  const cdp = await connectCDP(wsURL);
  await cdp.send("Page.enable");
  await cdp.send("Runtime.enable");
  await cdp.send("Page.navigate", { url: baseURL });
  await waitForLoad(cdp);

  await evalInPage(cdp, `
    (() => {
      if (!document.querySelector("#create-form")) throw new Error("create form missing");
      if (!document.querySelector("#url-list")) throw new Error("url list missing");
      return true;
    })()
  `);

  const targetURL = `https://example.com/ui-smoke-${Date.now()}`;
  const shortURL = await evalInPage(cdp, `
    (async () => {
      const input = document.querySelector("#url");
      input.value = ${JSON.stringify(targetURL)};
      input.dispatchEvent(new Event("input", { bubbles: true }));
      document.querySelector("#create-form").requestSubmit();

      const deadline = Date.now() + 10000;
      while (Date.now() < deadline) {
        const value = document.querySelector("#short-url")?.textContent?.trim();
        if (value) return value;
        await new Promise((resolve) => setTimeout(resolve, 100));
      }
      throw new Error("short URL did not render");
    })()
  `);

  await waitFor(async () => {
    const response = await fetch(shortURL, { redirect: "manual" });
    return response.status === 302 && response.headers.get("location") === targetURL;
  }, "redirect did not return expected 302");

  const code = shortURL.split("/").pop();
  const hitCount = await evalInPage(cdp, `
    (async () => {
      const code = ${JSON.stringify(code)};
      const deadline = Date.now() + 10000;
      while (Date.now() < deadline) {
        const response = await fetch("/api/v1/urls/" + encodeURIComponent(code) + "/hits");
        const data = await response.json();
        const total = (data.series || []).reduce((sum, point) => sum + Number(point.hits || 0), 0);
        if (total > 0) return total;
        await new Promise((resolve) => setTimeout(resolve, 200));
      }
      throw new Error("hourly hits did not increment");
    })()
  `);

  const listCheck = await evalInPage(cdp, `
    (async () => {
      const deadline = Date.now() + 10000;
      while (Date.now() < deadline) {
        const card = [...document.querySelectorAll(".url-card")].find((el) => el.textContent.includes(${JSON.stringify(shortURL)}));
        if (card && card.querySelectorAll(".chart .bar").length > 0) {
          return {
            hasCreator: card.textContent.includes("dev@example.com"),
            hasBars: card.querySelectorAll(".chart .bar").length,
          };
        }
        await new Promise((resolve) => setTimeout(resolve, 200));
      }
      throw new Error("created URL did not render in list with chart");
    })()
  `);

  if (!listCheck.hasCreator) throw new Error("creator was not rendered");
  if (listCheck.hasBars < 1) throw new Error("chart bars were not rendered");

  console.log(JSON.stringify({
    ok: true,
    short_url: shortURL,
    target_url: targetURL,
    hit_count: hitCount,
    chart_bars: listCheck.hasBars,
  }, null, 2));
} finally {
  chrome.kill("SIGTERM");
}

async function waitForWebSocketURL() {
  const endpoint = `http://127.0.0.1:${debugPort}/json/list`;
  for (let i = 0; i < 300; i++) {
    try {
      const response = await fetch(endpoint);
      if (response.ok) {
        const targets = await response.json();
        const page = targets.find((target) => target.type === "page" && target.webSocketDebuggerUrl);
        if (page) return page.webSocketDebuggerUrl;
      }
    } catch {
      // Chrome is still starting.
    }
    await sleep(100);
  }
  throw new Error("Chrome DevTools endpoint did not start");
}

function resolveChromeBin() {
  if (process.env.CHROME_BIN) return process.env.CHROME_BIN;
  for (const candidate of [
    "/usr/bin/google-chrome",
    "/usr/bin/google-chrome-stable",
    "/usr/bin/chromium-browser",
    "/usr/bin/chromium",
    "google-chrome",
    "google-chrome-stable",
    "chromium-browser",
    "chromium",
  ]) {
    if (candidate.startsWith("/") && !existsSync(candidate)) continue;
    return candidate;
  }
  return "google-chrome";
}

function connectCDP(wsURL) {
  const ws = new WebSocket(wsURL);
  let id = 0;
  const pending = new Map();
  const events = new EventTarget();

  ws.addEventListener("message", (event) => {
    const msg = JSON.parse(event.data);
    if (msg.id && pending.has(msg.id)) {
      const { resolve, reject } = pending.get(msg.id);
      pending.delete(msg.id);
      if (msg.error) reject(new Error(msg.error.message));
      else resolve(msg.result || {});
      return;
    }
    if (msg.method) events.dispatchEvent(new MessageEvent(msg.method, { data: msg.params || {} }));
  });

  return new Promise((resolve, reject) => {
    ws.addEventListener("open", () => {
      resolve({
        send(method, params = {}) {
          const requestID = ++id;
          ws.send(JSON.stringify({ id: requestID, method, params }));
          return new Promise((resolveRequest, rejectRequest) => {
            pending.set(requestID, { resolve: resolveRequest, reject: rejectRequest });
          });
        },
        once(method) {
          return new Promise((resolveEvent) => {
            events.addEventListener(method, (event) => resolveEvent(event.data), { once: true });
          });
        },
      });
    }, { once: true });
    ws.addEventListener("error", reject, { once: true });
  });
}

async function waitForLoad(cdp) {
  await cdp.once("Page.loadEventFired");
}

async function evalInPage(cdp, expression) {
  const result = await cdp.send("Runtime.evaluate", {
    expression,
    awaitPromise: true,
    returnByValue: true,
  });
  if (result.exceptionDetails) {
    throw new Error(result.exceptionDetails.text || "page evaluation failed");
  }
  return result.result.value;
}

async function waitFor(check, errorMessage) {
  const deadline = Date.now() + 10000;
  while (Date.now() < deadline) {
    if (await check()) return;
    await sleep(200);
  }
  throw new Error(errorMessage);
}
