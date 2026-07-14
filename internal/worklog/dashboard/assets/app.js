const state = {
  data: null,
  month: new Date(new Date().getFullYear(), new Date().getMonth(), 1),
  project: "",
  status: "",
};

const el = (id) => document.getElementById(id);
let receiptTrigger = null;

function colorIndex(value) {
  let hash = 0;
  for (const char of value) hash = ((hash << 5) - hash + char.charCodeAt(0)) | 0;
  return Math.abs(hash) % 6;
}

function localDay(value) {
  const date = new Date(value);
  return new Date(date.getFullYear(), date.getMonth(), date.getDate());
}

function dayKey(date) {
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, "0")}-${String(date.getDate()).padStart(2, "0")}`;
}

function addDays(date, days) {
  const copy = new Date(date);
  copy.setDate(copy.getDate() + days);
  return copy;
}

function duration(seconds) {
  if (!seconds) return "0m";
  const minutes = Math.max(1, Math.round(seconds / 60));
  if (minutes < 60) return `${minutes}m`;
  const hours = Math.floor(minutes / 60);
  const remainder = minutes % 60;
  if (hours < 24) return remainder ? `${hours}h ${remainder}m` : `${hours}h`;
  const days = Math.floor(hours / 24);
  const remainingHours = hours % 24;
  return remainingHours ? `${days}d ${remainingHours}h` : `${days}d`;
}

function filteredFeatures() {
  if (!state.data) return [];
  return state.data.features.filter((item) => {
    const feature = item.feature;
    return (!state.project || feature.project === state.project) && (!state.status || feature.status === state.status);
  });
}

function renderSummary() {
  const summary = state.data.summary;
  el("activeCount").textContent = summary.active_count + summary.blocked_count + summary.verified_count;
  el("shippedCount").textContent = summary.shipped_count;
  el("sessionCount").textContent = summary.total_sessions;
  el("scopeCount").textContent = summary.total_scope_added;
  el("updatedAt").textContent = `UPDATED ${new Date(state.data.generated_at).toLocaleString([], { dateStyle: "medium", timeStyle: "short" })}`;
}

function renderProjectOptions() {
  const projects = [...new Set(state.data.features.map((item) => item.feature.project))].sort();
  const select = el("projectFilter");
  const current = select.value;
  select.innerHTML = '<option value="">All projects</option>';
  for (const project of projects) {
    const option = document.createElement("option");
    option.value = project;
    option.textContent = project;
    select.append(option);
  }
  select.value = projects.includes(current) ? current : "";
}

function featuresForDay(date, features) {
  const key = dayKey(date);
  return features.filter((item) => {
    const start = localDay(item.feature.created_at);
    const end = localDay(item.end_at);
    return dayKey(start) <= key && dayKey(end) >= key;
  });
}

function featureButton(item, date) {
  const button = document.createElement("button");
  const startsToday = dayKey(localDay(item.feature.created_at)) === dayKey(date);
  button.type = "button";
  button.className = `feature-chip color-${colorIndex(item.feature.project)}${startsToday ? "" : " continuing"}`;
  const name = document.createElement("span");
  const elapsed = document.createElement("span");
  name.className = "feature-name";
  elapsed.className = "feature-duration";
  name.textContent = startsToday ? item.feature.title : `- ${item.feature.title}`;
  elapsed.textContent = duration(item.cycle_seconds);
  button.append(name, elapsed);
  button.title = `${item.feature.title} - ${duration(item.cycle_seconds)}`;
  button.setAttribute("aria-label", `${item.feature.title}, ${item.feature.status}, cycle ${duration(item.cycle_seconds)}`);
  button.addEventListener("click", () => openReceipt(item));
  return button;
}

function renderCalendar() {
  const features = filteredFeatures();
  const grid = el("calendarGrid");
  const agenda = el("agenda");
  grid.replaceChildren();
  agenda.replaceChildren();
  el("monthLabel").textContent = state.month.toLocaleString([], { month: "long", year: "numeric" });

  const first = new Date(state.month);
  const mondayOffset = (first.getDay() + 6) % 7;
  const gridStart = addDays(first, -mondayOffset);
  const today = localDay(new Date());
  let visibleCount = 0;

  for (let index = 0; index < 42; index++) {
    const date = addDays(gridStart, index);
    const items = featuresForDay(date, features);
    if (date.getMonth() === state.month.getMonth()) visibleCount += items.length;
    const day = document.createElement("article");
    day.className = "day";
    if (date.getMonth() !== state.month.getMonth()) day.classList.add("outside");
    if (dayKey(date) === dayKey(today)) day.classList.add("today");
    day.setAttribute("aria-label", date.toLocaleDateString([], { weekday: "long", month: "long", day: "numeric" }));
    const number = document.createElement("span");
    number.className = "day-number";
    number.textContent = date.getDate();
    const list = document.createElement("div");
    list.className = "day-features";
    for (const item of items.slice(0, 3)) list.append(featureButton(item, date));
    if (items.length > 3) {
      const more = document.createElement("span");
      more.className = "more-count";
      more.textContent = `+${items.length - 3} more`;
      list.append(more);
    }
    day.append(number, list);
    grid.append(day);
  }

  const daysInMonth = new Date(state.month.getFullYear(), state.month.getMonth() + 1, 0).getDate();
  for (let dayNumber = 1; dayNumber <= daysInMonth; dayNumber++) {
    const date = new Date(state.month.getFullYear(), state.month.getMonth(), dayNumber);
    const items = featuresForDay(date, features);
    if (!items.length) continue;
    const row = document.createElement("article");
    row.className = "agenda-day";
    const dateLabel = document.createElement("div");
    dateLabel.className = "agenda-date";
    dateLabel.innerHTML = `${date.toLocaleString([], { weekday: "short" })}<strong>${dayNumber}</strong>`;
    const list = document.createElement("div");
    list.className = "agenda-items";
    for (const item of items) list.append(featureButton(item, date));
    row.append(dateLabel, list);
    agenda.append(row);
  }
  el("emptyState").hidden = visibleCount > 0;
}

function metricRow(label, value) {
  const dt = document.createElement("dt");
  const dd = document.createElement("dd");
  dt.textContent = label;
  dd.textContent = value;
  return [dt, dd];
}

function sessionModels(sessions) {
  const names = [...new Set(sessions.map((session) => session.model_name).filter(Boolean))];
  return names.length ? names.join(", ") : "-";
}

function openReceipt(item) {
  receiptTrigger = document.activeElement;
  el("receiptId").textContent = `${item.feature.id} / ${item.feature.project}`;
  el("receiptTitle").textContent = item.feature.title;
  const metrics = el("receiptMetrics");
  metrics.replaceChildren(
    ...metricRow("STATUS", item.feature.status.toUpperCase()),
    ...metricRow("CYCLE", duration(item.cycle_seconds)),
    ...metricRow("BLOCKED", duration(item.blocked_seconds)),
    ...metricRow("VERIFY LAG", duration(item.verification_lag_seconds)),
    ...metricRow("SESSIONS", String(item.session_count)),
    ...metricRow("MODELS", sessionModels(item.sessions)),
    ...metricRow("SIZE / TYPE", `${item.feature.size || "-"} / ${item.feature.type || "-"}`),
    ...metricRow("BUDGET", duration(item.feature.budget_seconds)),
    ...metricRow("SCOPE + / LATER", `${item.scope_added} / ${item.scope_deferred}`),
  );

  const events = el("receiptEvents");
  events.replaceChildren();
  for (const event of item.events) {
    const li = document.createElement("li");
    const time = document.createElement("time");
    const kind = document.createElement("b");
    const note = document.createElement("span");
    time.textContent = new Date(event.created_at).toLocaleDateString([], { month: "short", day: "numeric" });
    kind.textContent = event.kind;
    note.textContent = event.note || "-";
    li.append(time, kind, note);
    events.append(li);
  }

  const scope = el("receiptScope");
  scope.replaceChildren();
  if (!item.scope.length) {
    const li = document.createElement("li");
    li.textContent = "No scope changes recorded.";
    scope.append(li);
  } else {
    for (const entry of item.scope) {
      const li = document.createElement("li");
      const decision = document.createElement("b");
      const text = document.createElement("span");
      decision.textContent = entry.decision;
      text.textContent = entry.text;
      li.append(decision, text);
      scope.append(li);
    }
  }

  const receipt = el("receipt");
  const scrim = el("scrim");
  receipt.inert = false;
  document.querySelector("header").inert = true;
  document.querySelector("main").inert = true;
  scrim.hidden = false;
  requestAnimationFrame(() => scrim.classList.add("visible"));
  receipt.classList.add("open");
  receipt.setAttribute("aria-hidden", "false");
  document.body.style.overflow = "hidden";
  el("closeReceipt").focus();
}

function closeReceipt() {
  const receipt = el("receipt");
  if (!receipt.classList.contains("open")) return;
  const scrim = el("scrim");
  receipt.classList.remove("open");
  receipt.setAttribute("aria-hidden", "true");
  receipt.inert = true;
  document.querySelector("header").inert = false;
  document.querySelector("main").inert = false;
  scrim.classList.remove("visible");
  document.body.style.overflow = "";
  setTimeout(() => { scrim.hidden = true; }, 180);
  if (receiptTrigger && document.contains(receiptTrigger)) receiptTrigger.focus();
}

async function load() {
  const refresh = el("refresh");
  refresh.disabled = true;
  try {
    const response = await fetch("/api/dashboard", { cache: "no-store" });
    if (!response.ok) throw new Error(`HTTP ${response.status}`);
    state.data = await response.json();
    renderSummary();
    renderProjectOptions();
    renderCalendar();
  } catch (error) {
    el("updatedAt").textContent = `READ FAILED / ${error.message}`;
  } finally {
    refresh.disabled = false;
  }
}

el("prevMonth").addEventListener("click", () => { state.month.setMonth(state.month.getMonth() - 1); renderCalendar(); });
el("nextMonth").addEventListener("click", () => { state.month.setMonth(state.month.getMonth() + 1); renderCalendar(); });
el("today").addEventListener("click", () => { const now = new Date(); state.month = new Date(now.getFullYear(), now.getMonth(), 1); renderCalendar(); });
el("projectFilter").addEventListener("change", (event) => { state.project = event.target.value; renderCalendar(); });
el("statusFilter").addEventListener("change", (event) => { state.status = event.target.value; renderCalendar(); });
el("refresh").addEventListener("click", load);
el("closeReceipt").addEventListener("click", closeReceipt);
el("scrim").addEventListener("click", closeReceipt);
document.addEventListener("keydown", (event) => { if (event.key === "Escape") closeReceipt(); });

load();
