const nameEl = document.getElementById("name");
const logoEl = document.getElementById("logo");
const idleEl = document.getElementById("idle");
const frameEl = document.getElementById("frame");
const qrEl = document.getElementById("qr");
const hintEl = document.getElementById("hint");
const fullBtn = document.getElementById("full");

let etag = "";

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

function showCode(data) {
  if (!data || !data.active || !data.pngBase64) {
    qrEl.removeAttribute("src");
    frameEl.hidden = true;
    hintEl.hidden = true;
    idleEl.hidden = false;
    return;
  }
  if (data.practiceName) nameEl.textContent = data.practiceName;
  const src = "data:image/png;base64," + data.pngBase64;
  if (qrEl.src !== src) qrEl.src = src;
  frameEl.hidden = false;
  hintEl.hidden = false;
  idleEl.hidden = true;
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
