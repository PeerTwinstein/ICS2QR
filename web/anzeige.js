const nameEl = document.getElementById("name");
const logoEl = document.getElementById("logo");
const idleEl = document.getElementById("idle");
const frameEl = document.getElementById("frame");
const qrEl = document.getElementById("qr");
const captionEl = document.getElementById("caption");
const remainEl = document.getElementById("remain");
const fullBtn = document.getElementById("full");

let etag = "";
let expiresAt = 0;

async function loadBrand() {
  try {
    const res = await fetch("/api/settings");
    if (!res.ok) return;
    const data = await res.json();
    nameEl.textContent = data.practiceName || "Termine";
    if (data.hasLogo) {
      logoEl.hidden = false;
      logoEl.src = "/logo?v=" + Date.now();
    } else {
      logoEl.hidden = true;
      logoEl.removeAttribute("src");
    }
  } catch {
    /* The idle screen stays usable without the practice record. */
  }
}

function hideCode() {
  expiresAt = 0;
  qrEl.removeAttribute("src");
  frameEl.hidden = true;
  captionEl.hidden = true;
  idleEl.hidden = false;
}

function showCode(data) {
  if (!data || !data.active || !data.pngBase64) {
    hideCode();
    return;
  }
  if (data.practiceName) nameEl.textContent = data.practiceName;
  const src = "data:image/png;base64," + data.pngBase64;
  if (qrEl.src !== src) qrEl.src = src;
  expiresAt = data.expiresAt ? Date.parse(data.expiresAt) : 0;
  frameEl.hidden = false;
  captionEl.hidden = false;
  idleEl.hidden = true;
  paintHint();
}

function paintHint() {
  if (frameEl.hidden) return;
  if (!Number.isFinite(expiresAt) || expiresAt <= 0) {
    remainEl.textContent = "";
    return;
  }
  const sec = Math.max(0, Math.ceil((expiresAt - Date.now()) / 1000));
  const m = Math.floor(sec / 60);
  const s = String(sec % 60).padStart(2, "0");
  const text = "Der Code verschwindet in " + m + ":" + s + ".";
  if (remainEl.textContent !== text) remainEl.textContent = text;
}

async function poll() {
  try {
    const headers = etag ? { "If-None-Match": etag } : {};
    const res = await fetch("/api/display", { headers, cache: "no-store" });
    if (res.status === 304) return;
    if (!res.ok) return;
    etag = res.headers.get("ETag") || "";
    showCode(await res.json());
  } catch {
    /* Keep the last image if the server is briefly unreachable. */
  }
}

fullBtn.addEventListener("click", () => {
  if (!document.fullscreenElement) {
    document.documentElement.requestFullscreen().catch(() => {});
  } else {
    document.exitFullscreen().catch(() => {});
  }
});

document.addEventListener("fullscreenchange", () => {
  fullBtn.textContent = document.fullscreenElement ? "Vollbild beenden" : "Vollbild";
});

loadBrand();
poll();
setInterval(poll, 400);
setInterval(paintHint, 250);
