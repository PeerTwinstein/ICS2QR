const SPANS = [
  ["month", "Monat"],
  ["quarter", "Quartal"],
  ["halfyear", "Halbjahr"],
  ["year", "Jahr"],
];
const MONTHS = ["Januar", "Februar", "März", "April", "Mai", "Juni", "Juli", "August", "September", "Oktober", "November", "Dezember"];
const WEEKDAYS = ["Mo", "Di", "Mi", "Do", "Fr", "Sa", "So"];
const PRESETS = [10, 15, 20, 30, 45, 60, 90];
const MAX_DURATION = 366 * 24 * 60;

const state = {
  settings: { practiceName: "", address: "", services: [], comments: [], calendarSpan: "month", hasLogo: false },
  span: "month",
  spanChosen: false,
  anchor: new Date(),
  selectedDate: todayISO(),
  multiDay: false,
  keepMulti: false,
  endDateTouched: false,
  commentOn: {},
  appointments: [],
  editingId: null,
  ics: "",
};

const draft = { practiceName: "", address: "", services: [], comments: [], calendarSpan: "month" };

const $ = (id) => document.getElementById(id);

function todayISO() {
  return isoFromDate(new Date());
}

function isoFromDate(d) {
  const m = String(d.getMonth() + 1).padStart(2, "0");
  const day = String(d.getDate()).padStart(2, "0");
  return `${d.getFullYear()}-${m}-${day}`;
}

function parseISO(s) {
  const [y, m, d] = s.split("-").map(Number);
  return new Date(y, m - 1, d);
}

function el(tag, className, text) {
  const node = document.createElement(tag);
  if (className) node.className = className;
  if (text != null) node.textContent = text;
  return node;
}

function setStatus(text, kind) {
  const node = $("status");
  node.textContent = text || "";
  node.className = "status" + (kind ? " " + kind : "");
  node.hidden = !text;
}

function clockParts(timeStr, minutes) {
  const [hh, mm] = timeStr.split(":").map(Number);
  const total = hh * 60 + mm + minutes;
  const days = Math.floor(total / 1440);
  const mins = ((total % 1440) + 1440) % 1440;
  return { days, hour: Math.floor(mins / 60), minute: mins % 60 };
}

function startMoment(dateStr, timeStr) {
  const [y, m, d] = dateStr.split("-").map(Number);
  const [hh, mm] = timeStr.split(":").map(Number);
  return new Date(y, m - 1, d, hh, mm, 0, 0);
}

function addClock(dateStr, timeStr, minutes) {
  const [y, m, d] = dateStr.split("-").map(Number);
  const c = clockParts(timeStr, minutes);
  return new Date(y, m - 1, d + c.days, c.hour, c.minute, 0, 0);
}

function endMoment(dateStr, timeStr, minutes, endDateStr) {
  const c = clockParts(timeStr, minutes);
  if (endDateStr) {
    const [y, m, d] = endDateStr.split("-").map(Number);
    return new Date(y, m - 1, d, c.hour, c.minute, 0, 0);
  }
  const [y, m, d] = dateStr.split("-").map(Number);
  return new Date(y, m - 1, d + c.days, c.hour, c.minute, 0, 0);
}

function formatEnd(d) {
  const date = d.toLocaleDateString("de-DE", { weekday: "long", day: "numeric", month: "long", year: "numeric" });
  const hm = d.toLocaleTimeString("de-DE", { hour: "2-digit", minute: "2-digit" });
  return `${date}, ${hm} Uhr`;
}

function periodStart(anchor, span) {
  const y = anchor.getFullYear();
  const m = anchor.getMonth();
  if (span === "quarter") return new Date(y, Math.floor(m / 3) * 3, 1);
  if (span === "halfyear") return new Date(y, m < 6 ? 0 : 6, 1);
  if (span === "year") return new Date(y, 0, 1);
  return new Date(y, m, 1);
}

function monthsInSpan(span) {
  return { month: 1, quarter: 3, halfyear: 6, year: 12 }[span] || 1;
}

function periodTitle(anchor, span) {
  const start = periodStart(anchor, span);
  const y = start.getFullYear();
  const m = start.getMonth();
  if (span === "quarter") return `${m / 3 + 1}. Quartal ${y}`;
  if (span === "halfyear") return `${m < 6 ? "1" : "2"}. Halbjahr ${y}`;
  if (span === "year") return String(y);
  return `${MONTHS[m]} ${y}`;
}

function shiftAnchor(anchor, span, dir) {
  const d = new Date(anchor.getFullYear(), anchor.getMonth(), 1);
  if (span === "quarter") d.setMonth(d.getMonth() + 3 * dir);
  else if (span === "halfyear") d.setMonth(d.getMonth() + 6 * dir);
  else if (span === "year") d.setFullYear(d.getFullYear() + dir);
  else d.setMonth(d.getMonth() + dir);
  return d;
}

function showView(view) {
  $("view-termine").hidden = view !== "termine";
  $("view-admin").hidden = view !== "verwaltung";
  $("tab-termine").setAttribute("aria-pressed", view === "termine" ? "true" : "false");
  $("tab-verwaltung").setAttribute("aria-pressed", view === "verwaltung" ? "true" : "false");
  $("title").textContent = view === "verwaltung" ? "Verwaltung" : (state.settings.practiceName || "Termine");
}

function updateLogo() {
  const header = $("logo");
  const preview = $("logo-preview");
  const remove = $("logo-remove");
  if (state.settings.hasLogo) {
    const src = "/logo?v=" + Date.now();
    header.hidden = false;
    header.src = src;
    preview.hidden = false;
    preview.src = src;
    remove.hidden = false;
  } else {
    header.hidden = true;
    header.removeAttribute("src");
    preview.hidden = true;
    preview.removeAttribute("src");
    remove.hidden = true;
  }
}

function applySettings(data) {
  state.settings = {
    practiceName: data.practiceName || "",
    address: data.address || "",
    services: data.services || [],
    comments: data.comments || [],
    calendarSpan: data.calendarSpan || "month",
    hasLogo: Boolean(data.hasLogo),
  };
  if (!state.spanChosen) state.span = state.settings.calendarSpan;
  draft.practiceName = state.settings.practiceName;
  draft.address = state.settings.address;
  draft.services = state.settings.services.slice();
  draft.comments = state.settings.comments.slice();
  draft.calendarSpan = state.settings.calendarSpan;
  $("practice-name").value = draft.practiceName;
  $("practice-address").value = draft.address;
  $("span-default").value = draft.calendarSpan;
  $("title").textContent = $("view-admin").hidden ? (state.settings.practiceName || "Termine") : "Verwaltung";
  updateLogo();
  renderServiceList();
  renderComments();
  renderAdminLists();
  syncSpanButtons();
  renderChosen();
  renderCalendar();
}

async function loadSettings() {
  const res = await fetch("/api/settings");
  if (!res.ok) throw new Error(await errorText(res));
  applySettings(await res.json());
}

function renderServiceList() {
  const list = $("service-list");
  list.replaceChildren();
  state.settings.services.forEach((name) => {
    const opt = document.createElement("option");
    opt.value = name;
    list.append(opt);
  });
}

function renderComments() {
  const box = $("comments");
  box.replaceChildren();
  const extras = Object.keys(state.commentOn).filter((c) => state.commentOn[c] && !state.settings.comments.includes(c));
  const all = state.settings.comments.concat(extras);
  if (!all.length) {
    box.append(el("p", "hint", "Kommentare können Sie in der Verwaltung anlegen. Freitext geht immer."));
    return;
  }
  all.forEach((c) => {
    const label = el("label", "check");
    const input = document.createElement("input");
    input.type = "checkbox";
    input.checked = Boolean(state.commentOn[c]);
    input.addEventListener("change", () => {
      state.commentOn[c] = input.checked;
    });
    label.append(input, document.createTextNode(c));
    box.append(label);
  });
}

function syncSpanButtons() {
  document.querySelectorAll(".span-btn").forEach((b) => {
    b.setAttribute("aria-pressed", b.dataset.span === state.span ? "true" : "false");
  });
}

function renderChosen() {
  const d = parseISO(state.selectedDate);
  $("chosen-day").textContent = "Gewählter Tag: " + d.toLocaleDateString("de-DE", {
    weekday: "long", day: "numeric", month: "long", year: "numeric",
  });
}

function renderCalendar() {
  const root = $("calendar");
  root.className = "months " + state.span;
  root.replaceChildren();
  const start = periodStart(state.anchor, state.span);
  const count = monthsInSpan(state.span);
  for (let i = 0; i < count; i++) {
    const d = new Date(start.getFullYear(), start.getMonth() + i, 1);
    root.append(renderMonth(d.getFullYear(), d.getMonth(), state.span !== "month"));
  }
  $("cal-title").textContent = periodTitle(state.anchor, state.span);
}

function renderMonth(year, month, showTitle) {
  const wrap = el("section", "month");
  if (showTitle) wrap.append(el("h3", "month-title", `${MONTHS[month]} ${year}`));
  const grid = el("div", "month-grid");
  WEEKDAYS.forEach((name) => grid.append(el("span", "wd", name)));
  const lead = (new Date(year, month, 1).getDay() + 6) % 7;
  for (let i = 0; i < lead; i++) grid.append(el("span", "wd"));
  const days = new Date(year, month + 1, 0).getDate();
  for (let day = 1; day <= days; day++) {
    const date = new Date(year, month, day);
    const iso = isoFromDate(date);
    const marks = state.appointments.filter((a) => a.date === iso);
    const btn = el("button", "day");
    btn.type = "button";
    btn.append(el("span", "num", String(day)));
    if (iso === todayISO()) btn.classList.add("today");
    if (iso === state.selectedDate) btn.classList.add("selected");
    if (date.getDay() === 0 || date.getDay() === 6) btn.classList.add("weekend");
    if (marks.length) {
      btn.classList.add("marked");
      if (state.span === "month") {
        marks.slice(0, 2).forEach((a) => btn.append(el("span", "mark", `${a.time} ${a.summary}`)));
        if (marks.length > 2) btn.append(el("span", "mark", `+${marks.length - 2}`));
      } else {
        btn.append(el("span", "dot"));
      }
    }
    const spoken = date.toLocaleDateString("de-DE", { weekday: "long", day: "numeric", month: "long" });
    btn.setAttribute("aria-label", marks.length ? `${spoken}, ${marks.map((a) => `${a.time} ${a.summary}`).join(", ")}` : spoken);
    btn.title = marks.map((a) => `${a.time} ${a.summary}`).join("\n");
    btn.addEventListener("click", () => selectDate(iso));
    grid.append(btn);
  }
  wrap.append(grid);
  return wrap;
}

function selectDate(iso) {
  state.selectedDate = iso;
  renderChosen();
  renderCalendar();
  refreshSchedule();
}

function paintPresets() {
  const dur = Number($("duration").value);
  document.querySelectorAll(".preset").forEach((b) => {
    b.setAttribute("aria-pressed", Number(b.dataset.min) === dur ? "true" : "false");
  });
}

function refreshSchedule() {
  const dur = Number($("duration").value);
  const timeVal = $("time").value;
  const valid = Number.isInteger(dur) && dur >= 1 && Boolean(timeVal);
  let naturalISO = "";
  let crosses = false;
  if (valid) {
    naturalISO = isoFromDate(addClock(state.selectedDate, timeVal, dur));
    crosses = naturalISO !== state.selectedDate;
  }
  if (!state.keepMulti) {
    state.multiDay = crosses;
    if (crosses && !state.endDateTouched) $("enddate").value = naturalISO;
    if (!crosses) {
      state.endDateTouched = false;
      $("enddate").value = "";
    }
  } else if (state.multiDay && !$("enddate").value && naturalISO) {
    $("enddate").value = naturalISO;
  }
  if (!state.multiDay) $("enddate").value = "";
  $("multiday").checked = state.multiDay;
  $("enddate-wrap").hidden = !state.multiDay;
  paintPreview();
  paintPresets();
}

function paintPreview() {
  const node = $("end-preview");
  const dur = Number($("duration").value);
  const timeVal = $("time").value;
  if (!Number.isInteger(dur) || dur < 1 || !timeVal) {
    node.textContent = "";
    node.classList.remove("bad");
    return;
  }
  const endDate = state.multiDay ? $("enddate").value : "";
  const end = endMoment(state.selectedDate, timeVal, dur, endDate);
  const start = startMoment(state.selectedDate, timeVal);
  if (!(end > start)) {
    node.textContent = "Das Ende muss nach dem Beginn liegen.";
    node.classList.add("bad");
    return;
  }
  node.classList.remove("bad");
  node.textContent = "Ende: " + formatEnd(end);
}

function checkedComments() {
  const ordered = state.settings.comments.slice();
  Object.keys(state.commentOn).forEach((c) => {
    if (!ordered.includes(c)) ordered.push(c);
  });
  return ordered.filter((c) => state.commentOn[c]);
}

function collectAppointment() {
  const summary = $("summary").value.trim();
  if (!summary) {
    setStatus("Bitte eine Leistung angeben.", "error");
    $("summary").focus();
    return null;
  }
  const dur = Number($("duration").value);
  if (!Number.isInteger(dur) || dur < 1 || dur > MAX_DURATION) {
    setStatus("Bitte eine Dauer zwischen 1 Minute und 366 Tagen angeben.", "error");
    return null;
  }
  const timeVal = $("time").value;
  if (!timeVal) {
    setStatus("Bitte eine Uhrzeit angeben.", "error");
    return null;
  }
  let endDate = "";
  if (state.multiDay) {
    endDate = $("enddate").value;
    if (!endDate) {
      setStatus("Bitte das Enddatum angeben.", "error");
      return null;
    }
  }
  const start = startMoment(state.selectedDate, timeVal);
  const end = endMoment(state.selectedDate, timeVal, dur, endDate);
  if (!(end > start)) {
    setStatus("Das Ende muss nach dem Beginn liegen.", "error");
    return null;
  }
  return {
    summary,
    date: state.selectedDate,
    time: timeVal.slice(0, 5),
    durationMin: dur,
    endDate,
    comments: checkedComments(),
    note: $("note").value.trim(),
  };
}

function resetForm() {
  state.editingId = null;
  state.commentOn = {};
  state.multiDay = false;
  state.keepMulti = false;
  state.endDateTouched = false;
  $("summary").value = "";
  $("note").value = "";
  $("multiday").checked = false;
  $("enddate").value = "";
  $("form-heading").textContent = "Termin";
  $("save-appt").textContent = "Termin hinzufügen";
  $("cancel-edit").hidden = true;
  renderComments();
  refreshSchedule();
}

let qrEpoch = 0;
let qrHideTimer = 0;

function hideQR() {
  qrEpoch++;
  clearTimeout(qrHideTimer);
  qrHideTimer = 0;
  state.ics = "";
  $("download-ics").hidden = true;
  fetch("/api/display", { method: "DELETE" }).catch(() => {});
}

function armQRExpiry(expiresAt) {
  const epoch = ++qrEpoch;
  clearTimeout(qrHideTimer);
  const wait = Math.max(0, Date.parse(expiresAt) - Date.now());
  qrHideTimer = setTimeout(() => {
    qrHideTimer = 0;
    if (epoch !== qrEpoch || !state.ics) return;
    setStatus("Der QR-Code ist von der Patientenanzeige verschwunden.", "");
  }, Number.isFinite(wait) ? wait : 2 * 60 * 1000);
}

let patientWin = null;
const SCREEN_KEY = "ics2qr.displayScreen";
let screenDetails = null;
let screensPromise = null;

function screenId(s) {
  return [s.label || "", s.isPrimary ? "1" : "0", s.isInternal ? "1" : "0", s.width, s.height, s.left, s.top].join("|");
}

function storedScreenId() {
  try { return localStorage.getItem(SCREEN_KEY) || ""; } catch { return ""; }
}

function sideName(s, screens) {
  if (s.isPrimary) return "Hauptbildschirm";
  const primary = screens.find((item) => item.isPrimary);
  if (!primary) return "weiterer Bildschirm";
  const dx = (s.left + s.width / 2) - (primary.left + primary.width / 2);
  const dy = (s.top + s.height / 2) - (primary.top + primary.height / 2);
  if (Math.abs(dx) >= Math.abs(dy)) return dx < 0 ? "links" : "rechts";
  return dy < 0 ? "darüber" : "darunter";
}

function screenCaption(s, screens) {
  const name = (s.label || "").trim();
  const useful = name && !/^(\\|generic|default)\b/i.test(name) ? name : "";
  const size = s.width + "×" + s.height;
  return useful ? `${sideName(s, screens)} · ${useful} · ${size}` : `${sideName(s, screens)} · ${size}`;
}

function chosenScreen() {
  if (!screenDetails || !screenDetails.screens.length) return null;
  const screens = screenDetails.screens;
  const stored = storedScreenId();
  const found = screens.find((s) => screenId(s) === stored);
  if (found) return found;
  const current = screenDetails.currentScreen;
  return screens.find((s) => s !== current) || current || null;
}

function setAskOption(denied) {
  const sel = $("screen-pick");
  sel.replaceChildren();
  const opt = document.createElement("option");
  opt.value = "";
  opt.textContent = denied ? "Monitorwahl blockiert" : "Monitor freigeben";
  sel.append(opt);
  sel.disabled = denied;
}

function fillScreenSelect() {
  if (!screenDetails) return;
  const sel = $("screen-pick");
  const screens = [...screenDetails.screens];
  sel.disabled = false;
  sel.replaceChildren();
  const seen = new Map();
  screens.forEach((s) => {
    const opt = document.createElement("option");
    opt.value = screenId(s);
    let caption = screenCaption(s, screens);
    const count = (seen.get(caption) || 0) + 1;
    seen.set(caption, count);
    if (count > 1) caption += " (" + count + ")";
    opt.textContent = caption;
    sel.append(opt);
  });
  const pick = chosenScreen();
  if (pick) sel.value = screenId(pick);
}

function ensureScreens() {
  if (screenDetails) return Promise.resolve(screenDetails);
  if (!("getScreenDetails" in window)) return Promise.resolve(null);
  if (!screensPromise) {
    screensPromise = window.getScreenDetails().then((details) => {
      screenDetails = details;
      details.addEventListener("screenschange", fillScreenSelect);
      fillScreenSelect();
      return details;
    }).catch((err) => {
      screensPromise = null;
      setAskOption(true);
      throw err;
    });
  }
  return screensPromise;
}

async function prepareScreens() {
  if (!("getScreenDetails" in window)) {
    $("open-display").title = "Dieser Browser kann den Monitor nicht wählen. Das Fenster lässt sich auf den anderen Bildschirm ziehen. Chrome und Edge können den Monitor festlegen.";
    return;
  }
  $("screen-pick-label").hidden = false;
  setAskOption(false);
  try {
    const perm = await navigator.permissions.query({ name: "window-management" });
    if (perm.state === "granted") await ensureScreens();
    if (perm.state === "denied") setAskOption(true);
    perm.onchange = () => {
      if (perm.state === "granted") void ensureScreens().catch(() => {});
      if (perm.state === "denied") setAskOption(true);
    };
  } catch {
    /* The click on the list asks for the monitors. */
  }
}

function popupFeatures(s) {
  if (!s) return "popup,width=640,height=900";
  const width = Math.max(640, s.availWidth || s.width);
  const height = Math.max(700, s.availHeight || s.height);
  return `popup,left=${s.availLeft},top=${s.availTop},width=${width},height=${height}`;
}

function placePatientWindow(win, s) {
  if (!win || !s) return;
  try {
    win.moveTo(s.availLeft, s.availTop);
    win.resizeTo(s.availWidth || s.width, s.availHeight || s.height);
  } catch {
    /* The browser keeps the window where it is. */
  }
}

function openPatientDisplay() {
  const s = chosenScreen();
  if (!(patientWin && !patientWin.closed)) {
    patientWin = window.open("/anzeige.html", "ics2qr-anzeige", popupFeatures(s));
  }
  if (patientWin) {
    placePatientWindow(patientWin, s);
    try { patientWin.focus(); } catch { /* The window can stay in the background. */ }
  }
  if (!screenDetails && "getScreenDetails" in window) {
    ensureScreens().then(() => {
      if (patientWin && !patientWin.closed) placePatientWindow(patientWin, chosenScreen());
    }).catch(() => {});
  }
  return Boolean(patientWin);
}

function touchList() {
  const had = Boolean(state.ics);
  hideQR();
  renderList();
  renderCalendar();
  if (had) setStatus("Die Termine haben sich geändert. Der QR-Code muss neu erzeugt werden.", "");
}

function sortedAppointments() {
  return state.appointments.slice().sort((a, b) => (a.date + a.time).localeCompare(b.date + b.time));
}

function renderList() {
  const ul = $("appt-list");
  ul.replaceChildren();
  const items = sortedAppointments();
  $("empty-list").hidden = items.length > 0;
  $("make-qr").disabled = items.length === 0;
  items.forEach((ap) => {
    const li = document.createElement("li");
    const text = el("div");
    const end = endMoment(ap.date, ap.time, ap.durationMin, ap.endDate);
    const sameDay = isoFromDate(end) === ap.date;
    const when = sameDay
      ? `${parseISO(ap.date).toLocaleDateString("de-DE", { weekday: "short", day: "numeric", month: "short", year: "numeric" })} ${ap.time}–${end.toLocaleTimeString("de-DE", { hour: "2-digit", minute: "2-digit" })}`
      : `${ap.date.split("-").reverse().join(".")} ${ap.time} – ${isoFromDate(end).split("-").reverse().join(".")} ${end.toLocaleTimeString("de-DE", { hour: "2-digit", minute: "2-digit" })}`;
    text.append(el("strong", "", `${when} · ${ap.summary}`));
    const extra = [...ap.comments, ap.note].filter(Boolean).join(" · ");
    if (extra) text.append(el("small", "", extra));
    const actions = el("div", "item-actions");
    const edit = el("button", "ghost tiny", "Bearbeiten");
    edit.type = "button";
    edit.addEventListener("click", () => editAppointment(ap));
    const remove = el("button", "ghost tiny", "Entfernen");
    remove.type = "button";
    remove.addEventListener("click", () => {
      state.appointments = state.appointments.filter((item) => item.id !== ap.id);
      if (state.editingId === ap.id) resetForm();
      touchList();
    });
    actions.append(edit, remove);
    li.append(text, actions);
    ul.append(li);
  });
}

function editAppointment(ap) {
  state.editingId = ap.id;
  state.selectedDate = ap.date;
  state.anchor = parseISO(ap.date);
  $("summary").value = ap.summary;
  $("time").value = ap.time;
  $("duration").value = String(ap.durationMin);
  $("note").value = ap.note || "";
  state.commentOn = {};
  ap.comments.forEach((c) => { state.commentOn[c] = true; });
  state.keepMulti = Boolean(ap.endDate);
  state.multiDay = Boolean(ap.endDate);
  state.endDateTouched = Boolean(ap.endDate);
  $("enddate").value = ap.endDate || "";
  $("form-heading").textContent = "Termin bearbeiten";
  $("save-appt").textContent = "Änderung übernehmen";
  $("cancel-edit").hidden = false;
  renderComments();
  renderChosen();
  renderCalendar();
  refreshSchedule();
  $("summary").focus();
}

async function makeQR() {
  if (state.editingId) {
    setStatus("Bitte die Änderung zuerst übernehmen oder abbrechen.", "error");
    return;
  }
  const pending = $("summary").value.trim();
  const appointments = sortedAppointments().map((ap) => ({
    summary: ap.summary,
    date: ap.date,
    time: ap.time,
    durationMin: ap.durationMin,
    endDate: ap.endDate,
    comments: ap.comments,
    note: ap.note,
  }));
  $("make-qr").disabled = true;
  const displayOpen = openPatientDisplay();
  try {
    const res = await fetch("/api/qr", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ appointments }),
    });
    if (!res.ok) throw new Error(await errorText(res));
    const data = await res.json();
    state.ics = data.ics;
    $("download-ics").hidden = false;
    if (data.expiresAt) armQRExpiry(data.expiresAt);
    let message = "Der QR-Code liegt auf der Patientenanzeige und verschwindet nach 2 Minuten.";
    if (!displayOpen) message += " Das Fenster dazu öffnet der Knopf „Patientenanzeige“.";
    if (pending) message += " Die noch nicht übernommene Leistung ist nicht enthalten.";
    setStatus(message, displayOpen && !pending ? "ok" : "");
  } catch (err) {
    setStatus(err.message || "Der QR-Code konnte nicht erzeugt werden.", "error");
  } finally {
    $("make-qr").disabled = state.appointments.length === 0;
  }
}

function downloadICS() {
  if (!state.ics) return;
  const blob = new Blob([state.ics], { type: "text/calendar" });
  const link = document.createElement("a");
  link.href = URL.createObjectURL(blob);
  link.download = "termine.ics";
  document.body.append(link);
  link.click();
  link.remove();
  setTimeout(() => URL.revokeObjectURL(link.href), 1000);
}

function renderAdminLists() {
  renderEditable($("service-items"), draft.services, (index) => {
    draft.services.splice(index, 1);
    renderAdminLists();
  });
  renderEditable($("comment-items"), draft.comments, (index) => {
    draft.comments.splice(index, 1);
    renderAdminLists();
  });
}

function renderEditable(ul, items, onRemove) {
  ul.replaceChildren();
  if (!items.length) {
    ul.append(el("li", "hint", "Noch keine Einträge."));
    return;
  }
  items.forEach((item, index) => {
    const li = document.createElement("li");
    li.append(el("span", "", item));
    const btn = el("button", "ghost tiny", "Entfernen");
    btn.type = "button";
    btn.addEventListener("click", () => onRemove(index));
    li.append(btn);
    ul.append(li);
  });
}

function addDraftItem(listName, input) {
  const value = input.value.trim();
  if (!value) return;
  if (draft[listName].some((item) => item.toLowerCase() === value.toLowerCase())) {
    setStatus("Dieser Eintrag ist schon vorhanden.", "error");
    return;
  }
  draft[listName].push(value);
  draft[listName].sort((a, b) => a.localeCompare(b, "de"));
  input.value = "";
  renderAdminLists();
  setStatus("");
}

async function saveSettings(event) {
  event.preventDefault();
  draft.practiceName = $("practice-name").value.trim();
  draft.address = $("practice-address").value.trim();
  draft.calendarSpan = $("span-default").value;
  try {
    const res = await fetch("/api/settings", {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        practiceName: draft.practiceName,
        address: draft.address,
        services: draft.services,
        comments: draft.comments,
        calendarSpan: draft.calendarSpan,
      }),
    });
    if (!res.ok) throw new Error(await errorText(res));
    applySettings(await res.json());
    renderCalendar();
    setStatus("Die Verwaltung ist gespeichert.", "ok");
  } catch (err) {
    setStatus(err.message || "Die Einstellungen konnten nicht gespeichert werden.", "error");
  }
}

async function uploadLogo(file) {
  if (!file) return;
  const body = new FormData();
  body.append("logo", file);
  try {
    const res = await fetch("/api/logo", { method: "POST", body });
    if (!res.ok) throw new Error(await errorText(res));
    state.settings.hasLogo = true;
    updateLogo();
    setStatus("Das Logo ist gespeichert.", "ok");
  } catch (err) {
    setStatus(err.message || "Das Logo konnte nicht gespeichert werden.", "error");
  } finally {
    $("logo-file").value = "";
  }
}

async function removeLogo() {
  try {
    const res = await fetch("/api/logo", { method: "DELETE" });
    if (!res.ok) throw new Error(await errorText(res));
    state.settings.hasLogo = false;
    updateLogo();
    setStatus("Das Logo ist entfernt.", "ok");
  } catch (err) {
    setStatus(err.message || "Das Logo konnte nicht entfernt werden.", "error");
  }
}

async function errorText(res) {
  try {
    const data = await res.json();
    return data.error || "Die Aktion ist fehlgeschlagen.";
  } catch {
    return "Die Aktion ist fehlgeschlagen.";
  }
}

function bind() {
  $("tab-termine").addEventListener("click", () => showView("termine"));
  $("tab-verwaltung").addEventListener("click", () => showView("verwaltung"));
  SPANS.forEach(([id, label]) => {
    const btn = el("button", "span-btn", label);
    btn.type = "button";
    btn.dataset.span = id;
    btn.addEventListener("click", () => {
      state.span = id;
      state.spanChosen = true;
      syncSpanButtons();
      renderCalendar();
    });
    $("span-switch").append(btn);
  });
  $("cal-prev").addEventListener("click", () => {
    state.anchor = shiftAnchor(state.anchor, state.span, -1);
    renderCalendar();
  });
  $("cal-next").addEventListener("click", () => {
    state.anchor = shiftAnchor(state.anchor, state.span, 1);
    renderCalendar();
  });
  $("cal-today").addEventListener("click", () => {
    state.anchor = new Date();
    selectDate(todayISO());
  });
  PRESETS.forEach((min) => {
    const btn = el("button", "preset", min + " min");
    btn.type = "button";
    btn.dataset.min = String(min);
    btn.addEventListener("click", () => {
      $("duration").value = String(min);
      refreshSchedule();
    });
    $("presets").append(btn);
  });
  ["time", "duration"].forEach((id) => $(id).addEventListener("input", refreshSchedule));
  $("multiday").addEventListener("change", () => {
    state.keepMulti = true;
    state.multiDay = $("multiday").checked;
    if (!state.multiDay) state.endDateTouched = false;
    refreshSchedule();
  });
  $("enddate").addEventListener("input", () => {
    state.endDateTouched = true;
    state.keepMulti = true;
    state.multiDay = true;
    $("multiday").checked = true;
    $("enddate-wrap").hidden = false;
    paintPreview();
  });
  $("appt-form").addEventListener("submit", (event) => {
    event.preventDefault();
    const ap = collectAppointment();
    if (!ap) return;
    if (state.editingId) {
      ap.id = state.editingId;
      const index = state.appointments.findIndex((item) => item.id === state.editingId);
      if (index >= 0) state.appointments[index] = ap;
    } else {
      ap.id = crypto.randomUUID();
      state.appointments.push(ap);
    }
    resetForm();
    touchList();
    $("summary").focus();
  });
  $("cancel-edit").addEventListener("click", () => {
    resetForm();
    renderCalendar();
  });
  $("make-qr").addEventListener("click", makeQR);
  $("screen-pick").addEventListener("pointerdown", (event) => {
    if (screenDetails || $("screen-pick").disabled) return;
    event.preventDefault();
    ensureScreens().catch(() => {});
  });
  $("screen-pick").addEventListener("change", () => {
    const id = $("screen-pick").value;
    if (!id) return;
    try { localStorage.setItem(SCREEN_KEY, id); } catch { /* The choice lasts for this tab only. */ }
    if (patientWin && !patientWin.closed) {
      placePatientWindow(patientWin, chosenScreen());
      try { patientWin.focus(); } catch { /* The window can stay in the background. */ }
    }
  });
  $("open-display").addEventListener("click", () => {
    if (!openPatientDisplay()) {
      setStatus("Die Patientenanzeige konnte nicht geöffnet werden. Der Browser blockiert das Fenster.", "error");
    }
  });
  $("download-ics").addEventListener("click", downloadICS);
  let clearTimer = 0;
  $("clear-all").addEventListener("click", () => {
    const btn = $("clear-all");
    const dirty = state.appointments.length || $("summary").value.trim() || $("note").value.trim() || state.ics || state.editingId;
    if (!dirty) {
      resetForm();
      renderCalendar();
      return;
    }
    if (btn.dataset.armed !== "1") {
      btn.dataset.armed = "1";
      btn.textContent = "Wirklich löschen?";
      clearTimer = setTimeout(() => {
        btn.dataset.armed = "";
        btn.textContent = "Eingabe löschen";
      }, 4000);
      return;
    }
    clearTimeout(clearTimer);
    btn.dataset.armed = "";
    btn.textContent = "Eingabe löschen";
    state.appointments = [];
    hideQR();
    resetForm();
    renderList();
    renderCalendar();
    setStatus("Die Eingabe ist geleert.", "ok");
  });
  $("service-add").addEventListener("click", () => addDraftItem("services", $("service-input")));
  $("comment-add").addEventListener("click", () => addDraftItem("comments", $("comment-input")));
  $("service-input").addEventListener("keydown", (event) => {
    if (event.key === "Enter") {
      event.preventDefault();
      addDraftItem("services", $("service-input"));
    }
  });
  $("comment-input").addEventListener("keydown", (event) => {
    if (event.key === "Enter") {
      event.preventDefault();
      addDraftItem("comments", $("comment-input"));
    }
  });
  $("view-admin").addEventListener("submit", saveSettings);
  $("logo-file").addEventListener("change", () => uploadLogo($("logo-file").files[0]));
  $("logo-remove").addEventListener("click", removeLogo);
}

bind();
prepareScreens();
renderChosen();
refreshSchedule();
renderCalendar();
renderList();
renderAdminLists();
loadSettings().catch(() => setStatus("Die Einstellungen konnten nicht geladen werden.", "error"));
