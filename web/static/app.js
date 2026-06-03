const form = document.querySelector("#create-form");
const urlInput = document.querySelector("#url");
const message = document.querySelector("#form-message");
const result = document.querySelector("#result");
const shortURL = document.querySelector("#short-url");
const copyButton = document.querySelector("#copy-button");
const list = document.querySelector("#url-list");
const refreshButton = document.querySelector("#refresh-button");
const loadMoreButton = document.querySelector("#load-more-button");

let nextPageToken = "";

function cookie(name) {
  return document.cookie
    .split("; ")
    .find((row) => row.startsWith(`${name}=`))
    ?.split("=")[1] || "";
}

async function api(path, options = {}) {
  const headers = new Headers(options.headers || {});
  headers.set("Accept", "application/json");
  if (options.body) {
    headers.set("Content-Type", "application/json");
    headers.set("X-CSRF-Token", decodeURIComponent(cookie("url_shortener_csrf")));
  }
  const response = await fetch(path, { ...options, headers });
  if (!response.ok) {
    const text = await response.text();
    throw new Error(text || `Request failed: ${response.status}`);
  }
  if (response.status === 204) {
    return null;
  }
  return response.json();
}

async function loadURLs(reset = false) {
  if (reset) {
    nextPageToken = "";
    list.innerHTML = "";
  }
  const params = new URLSearchParams({ limit: "25" });
  if (nextPageToken) params.set("page_token", nextPageToken);
  const data = await api(`/api/v1/urls?${params.toString()}`);
  nextPageToken = data.next_page_token || "";
  loadMoreButton.classList.toggle("hidden", !nextPageToken);
  for (const item of data.urls || []) {
    list.appendChild(await renderURL(item));
  }
}

async function renderURL(item) {
  const card = document.createElement("article");
  card.className = "url-card";
  card.innerHTML = `
    <div class="url-card-header">
      <div class="url-main">
        <a class="short-link" href="${escapeHTML(item.short_url)}" target="_blank" rel="noreferrer">${escapeHTML(item.short_url)}</a>
        <p class="long-url">${escapeHTML(item.url)}</p>
      </div>
      <button class="secondary" type="button">Copy</button>
    </div>
    <div class="meta">
      <span>${escapeHTML(item.created_by?.email || item.created_by?.id || "unknown")}</span>
      <span>${new Date(item.created_at).toLocaleString()}</span>
      <span class="hit-total">Loading hits...</span>
    </div>
    <div class="chart" aria-label="Hourly hits chart"></div>
  `;
  card.querySelector("button").addEventListener("click", () => navigator.clipboard.writeText(item.short_url));
  const chart = card.querySelector(".chart");
  try {
    const hits = await api(`/api/v1/urls/${encodeURIComponent(item.code)}/hits`);
    const series = hits.series || [];
    card.querySelector(".hit-total").textContent = `${series.reduce((sum, point) => sum + Number(point.hits || 0), 0)} hits in 24h`;
    renderChart(chart, series);
  } catch {
    card.querySelector(".hit-total").textContent = "Hits unavailable";
    chart.textContent = "Chart unavailable";
  }
  return card;
}

function renderChart(target, series) {
  target.innerHTML = "";
  const max = Math.max(1, ...series.map((point) => Number(point.hits || 0)));
  for (const point of series.slice(-24)) {
    const bar = document.createElement("div");
    bar.className = "bar";
    bar.style.height = `${Math.max(2, (Number(point.hits || 0) / max) * 72)}px`;
    bar.title = `${new Date(point.hour_start).toLocaleString()}: ${point.hits} hits`;
    target.appendChild(bar);
  }
}

function escapeHTML(value) {
  return String(value).replace(/[&<>"']/g, (char) => ({
    "&": "&amp;",
    "<": "&lt;",
    ">": "&gt;",
    '"': "&quot;",
    "'": "&#39;",
  }[char]));
}

form.addEventListener("submit", async (event) => {
  event.preventDefault();
  message.textContent = "Creating...";
  try {
    const data = await api("/api/v1/urls", {
      method: "POST",
      body: JSON.stringify({ url: urlInput.value }),
    });
    shortURL.textContent = data.short_url;
    result.classList.remove("hidden");
    message.textContent = "";
    urlInput.value = "";
    await loadURLs(true);
  } catch (error) {
    message.textContent = error.message;
  }
});

copyButton.addEventListener("click", () => navigator.clipboard.writeText(shortURL.textContent));
refreshButton.addEventListener("click", () => loadURLs(true));
loadMoreButton.addEventListener("click", () => loadURLs(false));

loadURLs(true).catch((error) => {
  list.textContent = error.message;
});
