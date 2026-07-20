"use strict";
const $ = (id) => document.getElementById(id);
const names = {
  overview: "Overview",
  evidence: "Evidence explorer",
  publish: "Guarded publish",
  activity: "Activity",
  settings: "Workspace settings",
  models: "Model lab",
  files: "Snapshot files",
  account: "Account & saved work",
};
const actionNames = {
  search: "Evidence collected",
  refresh: "Evidence refreshed",
  verify: "Absence verified",
  compose: "Receipts composed",
  search_missing: "Missing coverage searched",
  guarded_create: "Guarded publication",
  operation: "Operation recovered",
  receipt: "Receipt inspected",
};
let token = sessionStorage.getItem("nsf-session") || "";
if (/^#[a-f0-9]{64}$/.test(location.hash)) {
  token = location.hash.slice(1);
  sessionStorage.setItem("nsf-session", token);
  history.replaceState(null, "", location.pathname);
}
window.addEventListener("hashchange", () => {
  if (/^#[a-f0-9]{64}$/.test(location.hash)) {
    token = location.hash.slice(1);
    sessionStorage.setItem("nsf-session", token);
    history.replaceState(null, "", location.pathname);
    navigate("overview");
    work("Connecting your browser…", refresh);
  }
});
let state = { events: [], policies: {}, connected: false },
  busy = false,
  claim = null,
  prepared = null,
  selectedEvent = null,
  toastTimer,
  requestWorkspace = null;
const encoder = new TextEncoder();
function base64(text) {
  const bytes = encoder.encode(text);
  let binary = "";
  for (let i = 0; i < bytes.length; i += 8192)
    binary += String.fromCharCode(...bytes.subarray(i, i + 8192));
  return btoa(binary);
}
function decode64(value) {
  try {
    return new TextDecoder().decode(
      Uint8Array.from(atob(value), (c) => c.charCodeAt(0)),
    );
  } catch {
    return "(binary value)";
  }
}
function node(tag, cls, text) {
  const n = document.createElement(tag);
  if (cls) n.className = cls;
  if (text !== undefined) n.textContent = text;
  return n;
}
function icon(name) {
  const svg = document.createElementNS("http://www.w3.org/2000/svg", "svg");
  const use = document.createElementNS(svg.namespaceURI, "use");
  use.setAttribute("href", "#i-" + name);
  svg.setAttribute("aria-hidden", "true");
  svg.append(use);
  return svg;
}
function badge(text, outcome = text) {
  const n = node("span", "badge", text);
  n.dataset.state = outcome;
  return n;
}
function setBadge(id, text, outcome = text) {
  $(id).textContent = text;
  $(id).dataset.state = outcome;
}
function short(value) {
  return value ? value.slice(0, 12) : "—";
}
function toast(message, error = false) {
  clearTimeout(toastTimer);
  $("toast").textContent = message;
  $("toast").classList.toggle("error", error);
  $("toast").hidden = false;
  toastTimer = setTimeout(
    () => ($("toast").hidden = true),
    error ? 12000 : 5000,
  );
}
async function api(route, body = {}) {
  let res;
  try {
    res = await fetch("/api/" + route, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "X-NSF-Session": token,
        "X-NSF-Workspace": requestWorkspace || state.workspace_id || "",
      },
      body: JSON.stringify(body),
      signal: AbortSignal.timeout(65000),
    });
  } catch {
    throw new Error(
      "The app did not respond. If you were publishing, recover the operation from Activity before trying again.",
    );
  }
  const text = await res.text();
  if (res.status === 401) {
    showSessionGate();
    throw new Error(
      "This browser session is not connected. Use the Web launcher or paste its secure launch link.",
    );
  }
  let data;
  try {
    data = JSON.parse(text);
  } catch {
    throw new Error(
      res.status === 401
        ? "Session expired. Reopen NotSoFast from the executable."
        : text || "The request failed.",
    );
  }
  if (!res.ok || data.error) {
    const e = new Error(
      data.error?.message || data.error?.reason || "Request failed",
    );
    e.reason = data.error?.reason;
    throw e;
  }
  return data.result;
}
const call = (action, body) => api("call/" + action, body);
async function work(label, fn) {
  if (busy) {
    toast("One check is already running. Please let it finish.");
    return;
  }
  busy = true;
  requestWorkspace = state.connected ? state.workspace_id : null;
  const fields = [...document.querySelectorAll("input,textarea,select")].map(
    (n) => [n, n.disabled],
  );
  for (const [n] of fields) n.disabled = true;
  document.body.classList.add("working");
  $("busy-label").textContent = label;
  $("busy-bar").hidden = false;
  try {
    return await fn();
  } catch (e) {
    toast(e.message, true);
    $("error-message").textContent = e.message;
    $("error-banner").hidden = false;
  } finally {
    busy = false;
    requestWorkspace = null;
    for (const [n, disabled] of fields) n.disabled = disabled;
    document.body.classList.remove("working");
    $("busy-bar").hidden = true;
  }
}
function navigate(page) {
  if (!names[page]) page = "overview";
  for (const n of document.querySelectorAll(".page"))
    n.classList.toggle("active", n.id === "page-" + page);
  for (const n of document.querySelectorAll(".nav-item[data-page]")) {
    const active = n.dataset.page === page;
    n.classList.toggle("active", active);
    if (active) n.setAttribute("aria-current", "page");
    else n.removeAttribute("aria-current");
  }
  $("page-name").textContent = names[page];
  document.title = "NotSoFast — " + names[page];
  history.replaceState(null, "", "#" + page);
  if (page === "files" && state.connected && !busy)
    work("Loading tracked entries…", () => loadFiles(0));
  if (page === "account" && token && !busy)
    work("Loading your insights…", loadInsights);
  window.scrollTo({ top: 0, behavior: "instant" });
}
document.addEventListener("click", (e) => {
  const b = e.target.closest("[data-page]");
  if (b) {
    navigate(b.dataset.page);
  }
});
document.querySelector(".brand").addEventListener("click", (e) => {
  e.preventDefault();
  navigate("overview");
});
$("connect-top").onclick = () => navigate("settings");
function requireWorkspace() {
  if (!state.connected) {
    navigate("settings");
    throw new Error("Connect a repository or open the sample workspace first.");
  }
}
function receipts() {
  const unique = new Map();
  for (const e of state.events) {
    const r = e.result?.receipt;
    if (r) unique.set(r.id, r);
  }
  return [...unique.values()];
}
function matchingReceipts(q) {
  return receipts().filter(
    (r) =>
      r.snapshot === q.snapshot &&
      r.predicate.kind === q.predicate.kind &&
      r.predicate.version === q.predicate.version &&
      r.predicate.value === q.predicate.value,
  );
}
function eventOutcome(e) {
  return e.result?.outcome || (e.state === "DONE" ? "RECORDED" : e.state);
}
function eventLabel(e) {
  return (
    e.request?.path ||
    (e.request?.predicate?.value
      ? decode64(e.request.predicate.value)
      : short(e.request?.snapshot || e.result?.receipt?.snapshot)) ||
    "Workspace operation"
  );
}
function showEvent(e) {
  selectedEvent = e;
  $("detail-title").textContent = actionNames[e.action] || e.action;
  $("detail-body").textContent = JSON.stringify(e, null, 2);
  $("recover").hidden = e.action !== "guarded_create";
  $("detail-dialog").showModal();
  if ($("save-file"))
    $("save-file").hidden = !(
      e.action === "guarded_create" && e.result?.outcome === "PUBLISHED"
    );
}
function renderActivity(container, events) {
  container.replaceChildren();
  if (!events.length) {
    const empty = node("div", "empty-state");
    empty.append(
      node("h3", "", "A fresh start."),
      node("p", "", "Your searches and guarded actions will appear here."),
    );
    container.append(empty);
    return;
  }
  for (const e of events) {
    const row = node("button", "activity-row");
    row.type = "button";
    const symbol = node("span", "activity-icon");
    symbol.append(icon(e.action === "guarded_create" ? "shield" : "search"));
    const main = node("span", "activity-main");
    main.append(
      node("strong", "", actionNames[e.action] || e.action),
      node("small", "", eventLabel(e)),
    );
    const time = node(
      "span",
      "activity-time",
      new Date(e.time).toLocaleTimeString([], {
        hour: "2-digit",
        minute: "2-digit",
      }),
    );
    row.append(symbol, main, badge(eventOutcome(e)), time);
    row.onclick = () => showEvent(e);
    container.append(row);
  }
}
function rule(title, description, passed = true) {
  const r = node("div", "rule-row");
  r.append(icon(passed ? "check" : "shield"));
  const copy = node("div");
  copy.append(node("strong", "", title), node("p", "", description));
  r.append(copy);
  return r;
}
function update(next) {
  const changed =
    state.source !== next.source ||
    state.snapshot?.commit !== next.snapshot?.commit;
  state = next;
  if (next.profile) renderProfile(next.profile);
  if (next.cloud) renderCloud(next.cloud);
  state.events ||= [];
  state.policies ||= {};
  if (changed) {
    claim = null;
    resetDecision();
    invalidateReview();
  }
  $("workspace-name").textContent =
    state.repository_url?.replace("https://github.com/", "") ||
    state.source?.split(/[\\/]/).pop() ||
    "Your workspace";
  $("workspace-sub").textContent = state.connected
    ? "Managed branch · local"
    : "Local · private";
  $("connection").classList.toggle("connected", state.connected);
  $("connection").lastElementChild.textContent = state.connected
    ? "Workspace connected"
    : "Not connected";
  $("metric-entries").textContent = state.entry_count ?? "—";
  $("metric-receipts").textContent = receipts().length;
  $("metric-published").textContent = new Set(
    state.events
      .filter((e) => e.result?.outcome === "PUBLISHED")
      .map((e) => e.result.id),
  ).size;
  $("metric-policies").textContent = state.connected
    ? Object.keys(state.policies).length
    : "—";
  $("activity-count").textContent = state.events.length;
  $("art-snapshot").textContent = state.snapshot
    ? short(state.snapshot.commit)
    : "Connect to begin";
  for (const n of document.querySelectorAll(".current-snapshot"))
    n.textContent = state.snapshot
      ? "Snapshot " + short(state.snapshot.commit)
      : "Connect a repository to begin";
  $("repository-path").value = state.source || "";
  $("storage-path").textContent =
    state.state_root || "Connect a workspace to see its storage.";
  renderActivity($("recent-activity"), state.events.slice(0, 4));
  renderActivity($("full-activity"), state.events);
  $("settings-policies").replaceChildren();
  for (const [name, p] of Object.entries(state.policies)) {
    $("settings-policies").append(
      rule(
        name === "unique" ? "Unique filenames" : "Required marker",
        p.kind === "exact_basename"
          ? "A basename must be absent throughout the tracked snapshot."
          : "Files under " +
              p.destination_prefix +
              "/ require the absent marker “" +
              decode64(p.marker) +
              "”.",
        false,
      ),
    );
  }
  if (!state.connected)
    $("settings-policies").append(
      node(
        "p",
        "field-note",
        "Connect a workspace to activate its conventions.",
      ),
    );
  if (state.model?.model) {
    $("model-endpoint").value = state.model.endpoint;
    $("model-name").value = state.model.model;
    $("model-config-status").textContent =
      "Configured: " +
      state.model.model +
      (state.model_key_in_session
        ? " · Key held for this session."
        : " · No API key in this session.");
  }
  renderReceipts();
  $("session-gate").hidden = true;
  document.body.classList.remove("session-locked");
  renderWorkspaces();
  renderGitHub(state.github || {});
}
async function refresh() {
  const next = await api("status");
  const switched = requestWorkspace && next.workspace_id !== requestWorkspace;
  update(next);
  $("error-banner").hidden = true;
  if (switched)
    throw new Error(
      "Workspace changed in another window. Review the selected repository and start again.",
    );
}
async function connect(body) {
  const next = await api("connect", body);
  update(next);
  navigate("overview");
  toast("Workspace ready. Your source files are unchanged.");
}
$("connect-form").onsubmit = (e) => {
  e.preventDefault();
  work("Importing the committed snapshot…", () =>
    connect({ source: $("repository-path").value.trim() }),
  );
};
for (const id of ["try-demo", "settings-demo"])
  $(id).onclick = () =>
    work("Opening your sample workspace…", () => connect({ demo: true }));
$("browse").onclick = () =>
  work("Choose a repository folder…", async () => {
    const result = await api("pick");
    if (result.path) $("repository-path").value = result.path;
  });
function resetDecision() {
  setBadge("decision-badge", "Not checked");
  $("coverage-percent").textContent = "—";
  $("ring-value").setAttribute("stroke-dashoffset", "414.69");
  $("decision-title").textContent = "The answer starts with a search.";
  $("decision-copy").textContent =
    "Search the full repository or start with one folder. We’ll show exactly what’s still missing.";
  for (const id of ["coverage-checked", "coverage-missing", "coverage-found"])
    $(id).textContent = "0";
  $("witnesses").replaceChildren();
  $("fill-gaps").disabled = true;
  $("compose").disabled = true;
  renderReceipts();
}
function renderReceipts() {
  const rs = claim ? matchingReceipts(claim) : [];
  $("receipt-total").textContent =
    rs.length + " receipt" + (rs.length === 1 ? "" : "s");
  const list = $("receipt-list");
  list.replaceChildren();
  if (!rs.length) {
    list.append(
      node(
        "p",
        "field-note",
        "Run a search to collect evidence for this claim.",
      ),
    );
    return;
  }
  for (const r of rs) {
    const row = node("div", "receipt-row"),
      main = node("div", "receipt-main");
    main.append(
      node("strong", "", short(r.id)),
      node(
        "p",
        "",
        (r.scope?.prefixes?.join(", ") || "Entire snapshot") +
          " · " +
          r.stats.evaluated +
          " evaluated · " +
          r.stats.reused +
          " reused",
      ),
    );
    const inspect = node("button", "button subtle", "Inspect");
    inspect.onclick = () =>
      showEvent(state.events.find((e) => e.result?.receipt?.id === r.id));
    row.append(
      icon("check"),
      main,
      badge(r.complete ? "Scan complete" : "Partial scan"),
      inspect,
    );
    list.append(row);
  }
}
function showDecision(d) {
  const pct =
    d.required === 0
      ? 100
      : Math.min(100, Math.round((d.evaluated / d.required) * 100));
  $("coverage-percent").textContent = pct + "%";
  $("ring-value").setAttribute(
    "stroke-dashoffset",
    String(414.69 * (1 - pct / 100)),
  );
  setBadge("decision-badge", d.outcome);
  $("coverage-checked").textContent = d.evaluated;
  $("coverage-missing").textContent = d.missing_count ?? d.missing?.length ?? 0;
  $("coverage-found").textContent = d.witness_count ?? d.witnesses?.length ?? 0;
  const messages = {
    SUPPORTED: [
      "Absence is supported.",
      "The full claimed domain has been checked, with no exact match. This evidence applies to this snapshot.",
    ],
    REFUTED: [
      "A match changes the answer.",
      "The value exists in the claimed domain. The absence claim is refuted, even if some coverage is incomplete.",
    ],
    UNKNOWN: [
      "There’s more to look at.",
      "The current evidence does not cover the full claim. Search the missing coverage before drawing a conclusion.",
    ],
  };
  const [title, copy] = messages[d.outcome] || [
    "Evidence needs attention.",
    d.reason,
  ];
  $("decision-title").textContent = title;
  $("decision-copy").textContent = copy;
  $("fill-gaps").disabled = d.outcome !== "UNKNOWN";
  $("compose").disabled = !claim || matchingReceipts(claim).length < 2;
  $("witnesses").replaceChildren();
  for (const path of d.witnesses || [])
    $("witnesses").append(node("li", "", path));
  renderReceipts();
}
async function verifyClaim() {
  if (!claim) throw new Error("The snapshot changed. Start a new search.");
  claim.receipts = matchingReceipts(claim).map((r) => r.id);
  const decision = await call("verify", claim);
  await refresh();
  showDecision(decision);
}
$("search-form").onsubmit = (e) => {
  e.preventDefault();
  work("Searching immutable entries…", async () => {
    requireWorkspace();
    const value = $("search-value").value;
    if (encoder.encode(value).length > 65536)
      throw new Error("Search value exceeds 64 KiB.");
    const predicate = {
      kind: $("predicate").value,
      value: base64(value),
      version: 1,
    };
    const prefix = $("search-scope").value.trim().replace(/\/$/, "");
    claim = {
      repository: "workspace",
      snapshot: state.snapshot.commit,
      predicate,
      scope: {},
      receipts: [],
    };
    await call("search", {
      repository: claim.repository,
      snapshot: claim.snapshot,
      predicate,
      scope: prefix ? { prefixes: [prefix] } : {},
    });
    await refresh();
    await verifyClaim();
  });
};
for (const id of ["predicate", "search-value", "search-scope"])
  $(id).addEventListener("input", () => {
    if (!busy) {
      claim = null;
      resetDecision();
    }
  });
$("fill-gaps").onclick = () =>
  work("Searching the uncovered entries…", async () => {
    if (!claim) return;
    await call("search_missing", claim);
    await refresh();
    await verifyClaim();
  });
$("compose").onclick = () =>
  work("Composing compatible evidence…", async () => {
    if (!claim) return;
    await call("compose", claim);
    await refresh();
    await verifyClaim();
    toast("Coverage combined into a new receipt.");
  });
function invalidateReview() {
  prepared = null;
  $("review-empty").hidden = false;
  $("review-result").hidden = true;
  $("publish-result").hidden = true;
  setBadge("review-badge", "Awaiting proposal");
  $("step-review").classList.remove("active");
  $("step-publish").classList.remove("active");
}
for (const id of ["file-path", "file-content"])
  $(id).addEventListener("input", invalidateReview);
$("prepare-form").onsubmit = (e) => {
  e.preventDefault();
  work("Checking the proposed file against every convention…", async () => {
    requireWorkspace();
    invalidateReview();
    const path = $("file-path").value.trim(),
      content = $("file-content").value,
      bytes = encoder.encode(content).length;
    if (bytes > 65536) throw new Error("File exceeds 64 KiB.");
    if (
      !path ||
      path.includes("\\") ||
      path.split("/").some((p) => !p || p === "." || p === "..")
    )
      throw new Error(
        "Use a relative path with forward slashes, without parent segments.",
      );
    await refresh();
    const snapshot = state.snapshot.commit,
      ids = [],
      rules = [];
    let supported = true;
    for (const [name, p] of Object.entries(state.policies)) {
      if (p.destination_prefix && !path.startsWith(p.destination_prefix + "/"))
        continue;
      if (p.kind === "literal_bytes" && !content.includes(decode64(p.marker)))
        throw new Error(
          "Files under " +
            p.destination_prefix +
            "/ must contain “" +
            decode64(p.marker) +
            "”.",
        );
      const predicate = {
        kind: p.kind,
        value:
          p.kind === "exact_basename"
            ? base64(path.split("/").pop())
            : p.marker,
        version: 1,
      };
      const r = await call("search", {
        repository: "workspace",
        snapshot,
        predicate,
        scope: p.scope || {},
      });
      ids.push(r.receipt.id);
      const d = await call("verify", {
        repository: "workspace",
        snapshot,
        predicate,
        scope: p.scope || {},
        receipts: [r.receipt.id],
      });
      rules.push({ name, decision: d });
      if (d.outcome !== "SUPPORTED") supported = false;
    }
    await refresh();
    $("review-empty").hidden = true;
    $("review-result").hidden = false;
    $("review-rules").replaceChildren();
    for (const r of rules) {
      const message =
        r.decision.outcome === "SUPPORTED"
          ? "Exact absence supported across the required scope."
          : r.decision.outcome + ": " + (r.decision.witnesses || []).join(", ");
      $("review-rules").append(
        rule(
          r.name === "unique" ? "Filename uniqueness" : "Marker absence",
          message,
          r.decision.outcome === "SUPPORTED",
        ),
      );
    }
    $("review-path").textContent = path;
    $("review-base").textContent = short(snapshot);
    $("review-bytes").textContent = bytes.toLocaleString() + " bytes";
    $("step-review").classList.add("active");
    setBadge(
      "review-badge",
      supported ? "Ready to publish" : "Check did not pass",
      supported ? "SUPPORTED" : "REFUTED",
    );
    $("publish-confirm").disabled = !supported;
    if (supported)
      prepared = {
        repository: "workspace",
        snapshot,
        operation: crypto.randomUUID(),
        policy: "unique",
        policy_version: state.policies.unique.version,
        path,
        content: base64(content),
        receipts: ids,
      };
  });
};
$("publish-confirm").onclick = () =>
  work("Publishing against the verified head…", async () => {
    if (!prepared) throw new Error("Check the file again before publishing.");
    const q = prepared;
    if (
      q.path !== $("file-path").value.trim() ||
      q.content !== base64($("file-content").value)
    ) {
      invalidateReview();
      throw new Error(
        "The proposal changed. Check it again before publishing.",
      );
    }
    prepared = null;
    $("publish-confirm").disabled = true;
    try {
      const result = await call("guarded_create", q);
      await refresh();
      $("review-empty").hidden = true;
      $("review-result").hidden = true;
      setBadge("review-badge", "Published", "PUBLISHED");
      $("step-review").classList.add("active");
      $("step-publish").classList.add("active");
      const panel = $("publish-result");
      panel.replaceChildren(
        node("h3", "", "Verified. Published. Recorded."),
        node(
          "p",
          "",
          "Your file was created on the managed branch. The source working tree is unchanged.",
        ),
        node("p", "", "Commit"),
        node("code", "", result.candidate),
        node("p", "", "Operation: " + result.id),
      );
      panel.hidden = false;
      toast("File published successfully.");
    } catch (e) {
      await refresh().catch(() => {});
      throw new Error(
        e.message +
          " Operation ID: " +
          q.operation +
          ". Inspect Activity to recover its status.",
      );
    }
  });
$("close-detail").onclick = () => $("detail-dialog").close();
$("recover").onclick = () =>
  work("Recovering the recorded operation…", async () => {
    const e = selectedEvent;
    const result = await call("operation", {
      repository: "workspace",
      id: e.request.operation,
    });
    $("detail-body").textContent = JSON.stringify(result, null, 2);
    await refresh();
    toast("Operation status recovered. No file was published again.");
  });
$("export").onclick = () => {
  const blob = new Blob(
    [
      JSON.stringify(
        {
          source: state.source,
          snapshot: state.snapshot,
          events: state.events,
        },
        null,
        2,
      ),
    ],
    { type: "application/json" },
  );
  const url = URL.createObjectURL(blob),
    a = node("a");
  a.href = url;
  a.download = "notsofast-activity.json";
  a.click();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
};
const motion = localStorage.getItem("nsf-reduced-motion") === "true";
$("reduce-motion").checked = motion;
document.body.classList.toggle("reduced-motion", motion);
$("reduce-motion").onchange = () => {
  document.body.classList.toggle("reduced-motion", $("reduce-motion").checked);
  localStorage.setItem(
    "nsf-reduced-motion",
    String($("reduce-motion").checked),
  );
};
function download(blob, name) {
  const url = URL.createObjectURL(blob),
    a = node("a");
  a.href = url;
  a.download = name;
  a.click();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}
const archiveButton = node(
  "button",
  "button subtle",
  "Download managed snapshot (.zip)",
);
archiveButton.id = "download-snapshot";
$("storage-path").after(archiveButton);
archiveButton.onclick = () =>
  work("Exporting the managed snapshot…", async () => {
    requireWorkspace();
    const response = await fetch("/api/export", {
      method: "POST",
      headers: {
        "X-NSF-Session": token,
        "X-NSF-Workspace": state.workspace_id,
      },
      signal: AbortSignal.timeout(65000),
    });
    if (!response.ok)
      throw new Error(
        "Snapshot export failed. Refresh the workspace and try again.",
      );
    download(
      await response.blob(),
      "notsofast-" + short(state.snapshot.commit) + ".zip",
    );
  });
const lookupForm = node("form", "lookup-form");
lookupForm.id = "lookup-form";
const lookupLabel = node("label", "", "Recover by operation ID");
lookupLabel.htmlFor = "operation-id";
const lookupInput = node("input");
lookupInput.id = "operation-id";
lookupInput.placeholder = "Paste a previous operation ID";
lookupInput.required = true;
lookupInput.maxLength = 256;
const lookupButton = node("button", "button subtle", "Recover status");
lookupButton.type = "submit";
lookupForm.append(lookupLabel, lookupInput, lookupButton);
$("full-activity").before(lookupForm);
lookupForm.onsubmit = (e) => {
  e.preventDefault();
  work("Recovering the recorded operation…", async () => {
    requireWorkspace();
    const result = await call("operation", {
      repository: "workspace",
      id: lookupInput.value.trim(),
    });
    showEvent({ action: "operation", result });
    await refresh();
  });
};
const saveFile = node("button", "button subtle", "Download this file");
saveFile.id = "save-file";
$("recover").before(saveFile);
saveFile.hidden = true;
saveFile.onclick = () => {
  const q = selectedEvent.request;
  download(
    new Blob([Uint8Array.from(atob(q.content), (c) => c.charCodeAt(0))], {
      type: "application/octet-stream",
    }),
    q.path.split("/").pop(),
  );
};
async function quit() {
  if (busy) {
    toast("Please let the current operation finish before quitting.");
    return;
  }
  await work("Closing the workspace…", async () => {
    await api("quit");
    toast("NotSoFast has closed. You can close this window.");
  });
}
let stopModels = false;
$("model-mode").onchange = () => {
  $("discover-models").hidden = $("model-mode").value !== "local";
  $("model-endpoint").value =
    $("model-mode").value === "local" ? "http://127.0.0.1:11434/v1" : "";
  $("model-endpoint").placeholder =
    $("model-mode").value === "local"
      ? "http://127.0.0.1:11434/v1"
      : "https://your-provider.example/v1";
  $("model-key").value = "";
};
$("discover-models").onclick = () =>
  work("Looking for local models…", async () => {
    const found = await api("model/discover");
    $("discovered-models").replaceChildren();
    if (!found.length) {
      $("discovered-models").append(
        node(
          "p",
          "field-note",
          "No local server found. Start Ollama or LM Studio with a tool-capable model, or choose a provider.",
        ),
      );
      return;
    }
    for (const item of found) {
      const b = node("button", "button subtle", item.model);
      b.type = "button";
      b.onclick = () => {
        $("model-endpoint").value = item.endpoint;
        $("model-name").value = item.model;
      };
      $("discovered-models").append(b);
    }
  });
$("model-form").onsubmit = (e) => {
  e.preventDefault();
  work("Saving the model connection…", async () => {
    await api("model/configure", {
      endpoint: $("model-endpoint").value.trim(),
      model: $("model-name").value.trim(),
      api_key: $("model-key").value,
    });
    $("model-key").value = "";
    await refresh();
    toast("Connection saved. No model request has been made yet.");
  });
};
$("stop-models").onclick = () => {
  stopModels = true;
  toast("Will stop after the current task finishes.");
};
$("run-models").onclick = () =>
  work("Running isolated model experiments…", async () => {
    stopModels = false;
    $("model-results").replaceChildren();
    $("stop-models").hidden = false;
    const modes =
      $("model-baseline").value === "ALL" ? "ABCDE" : $("model-baseline").value;
    try {
      for (const mode of modes) {
        for (const destination of ["database.yaml", "fresh.txt"]) {
          if (stopModels) return;
          $("busy-label").textContent =
            "Baseline " + mode + " · " + destination + " · Waiting for model…";
          const result = await api("model/trial", {
            baseline: mode,
            destination,
          });
          const row = node("div", "model-trial"),
            title = node("h3", "", mode + " · " + destination);
          const passed =
            !result.infrastructure_error &&
            (result.ground_truth_valid
              ? result.legitimate_completion
              : !result.incorrectly_admitted);
          title.append(
            badge(
              result.infrastructure_error
                ? "Incomplete"
                : passed
                  ? "Expected result"
                  : "Unexpected result",
              result.infrastructure_error
                ? "UNKNOWN"
                : passed
                  ? "SUPPORTED"
                  : "REFUTED",
            ),
          );
          row.append(
            title,
            node(
              "p",
              "",
              result.infrastructure_error ||
                (result.published ? "Published" : "No publication") +
                  " · " +
                  result.model_requests +
                  " model requests · " +
                  result.tool_calls +
                  " tool calls · " +
                  (result.duration_ms / 1000).toFixed(1) +
                  "s",
            ),
          );
          $("model-results").append(row);
        }
      }
    } finally {
      $("stop-models").hidden = true;
    }
  });
$("export-models").onclick = () =>
  work("Loading saved experiment results…", async () => {
    const results = await api("model/results");
    if (!results.length) {
      toast("No saved model experiments yet.");
      return;
    }
    download(
      new Blob([JSON.stringify(results, null, 2)], {
        type: "application/json",
      }),
      "notsofast-model-results.json",
    );
  });
$("quit").onclick = quit;
$("settings-quit").onclick = quit;
function showSessionGate() {
  $("session-gate").hidden = false;
  document.body.classList.add("session-locked");
}
$("session-form").onsubmit = (e) => {
  e.preventDefault();
  try {
    const value = $("session-link").value.trim();
    const link = new URL(value);
    if (
      link.protocol !== "http:" ||
      link.hostname !== "127.0.0.1" ||
      link.username ||
      link.password ||
      !link.port ||
      !/^#[a-f0-9]{64}$/.test(link.hash)
    )
      throw new Error("Use the complete local launch link from NotSoFast.");
    location.assign(link.href);
  } catch (e) {
    toast(e.message, true);
  }
};
$("open-browser").onclick = () =>
  work("Opening the authenticated website…", async () => {
    await api("open-browser");
    toast("Your workspace has opened in the default browser.");
  });
$("dismiss-error").onclick = () => ($("error-banner").hidden = true);
$("retry-status").onclick = () =>
  work("Reconnecting to your workspace…", refresh);
function renderWorkspaces() {
  const target = $("recent-workspaces");
  target.replaceChildren();
  for (const item of state.workspaces || []) {
    const b = node("button", "workspace-choice");
    b.type = "button";
    b.append(icon(item.repository_url ? "branch" : "folder"));
    const text = node("span");
    text.append(
      node(
        "strong",
        "",
        item.repository_url?.replace("https://github.com/", "") ||
          item.source.split(/[\\/]/).pop(),
      ),
      node("small", "", item.repository_url || item.source),
    );
    b.append(text);
    if (item.source === state.source) b.append(badge("Active"));
    b.onclick = () =>
      work("Opening workspace…", () => connect({ source: item.source }));
    target.append(b);
  }
  if (!target.childElementCount)
    target.append(
      node("p", "field-note", "Your connected repositories will appear here."),
    );
}
function connectionTab(tab) {
  const github = tab === "github";
  $("tab-github").classList.toggle("active", github);
  $("tab-local").classList.toggle("active", !github);
  $("tab-github").setAttribute("aria-pressed", String(github));
  $("tab-local").setAttribute("aria-pressed", String(!github));
  $("github-connect").hidden = !github;
  $("connect-form").hidden = github;
}
let authTimer = null;
function renderGitHub(g) {
  $("github-account-name").textContent = g.signed_in
    ? "Connected as " + g.login
    : "GitHub, on your terms.";
  $("github-account-description").textContent = g.signed_in
    ? "Public and authorized private repositories."
    : "Public repositories need only a URL.";
  $("github-sign-in").hidden = !!g.signed_in;
  $("github-sign-out").hidden = !g.signed_in;
  const pending = ["starting", "authorizing"].includes(g.stage);
  $("github-auth-box").hidden = !pending && g.stage !== "error";
  $("github-device-code").textContent = g.code || "Requesting code…";
  $("github-device-code").hidden = !pending;
  $("github-authorize").hidden = !g.code;
  $("github-auth-message").textContent = g.message || "";
  $("github-cancel").hidden = !pending;
  if (pending && !authTimer) authTimer = setTimeout(pollGitHub, 2000);
}
async function pollGitHub() {
  authTimer = null;
  try {
    renderGitHub(await api("github/auth-state"));
  } catch {
    if (!document.body.classList.contains("session-locked"))
      authTimer = setTimeout(pollGitHub, 3000);
  }
}
$("tab-local").onclick = () => connectionTab("local");
$("tab-github").onclick = () => {
  connectionTab("github");
  work("Checking your GitHub connection…", async () =>
    renderGitHub(await api("github/status")),
  );
};
$("github-sign-in").onclick = () =>
  work("Starting GitHub browser sign-in…", async () =>
    renderGitHub(await api("github/sign-in")),
  );
async function disconnectGitHub() {
  clearTimeout(authTimer);
  authTimer = null;
  renderGitHub(await api("github/sign-out"));
  toast("Disconnected from this app. Existing local workspaces are retained.");
}
$("github-sign-out").onclick = () =>
  work("Disconnecting GitHub…", disconnectGitHub);
$("github-cancel").onclick = () =>
  work("Cancelling sign-in…", disconnectGitHub);
$("github-form").onsubmit = (e) => {
  e.preventDefault();
  work("Importing the latest GitHub snapshot…", async () => {
    const next = await api("github/import", {
      url: $("github-url").value.trim(),
    });
    update(next);
    navigate("overview");
    toast("GitHub snapshot imported. No remote changes were made.");
  });
};
let filePage = null,
  preview = null;
async function loadFiles(offset) {
  requireWorkspace();
  const result = await api("files", {
    query: $("file-query").value.trim(),
    offset,
  });
  filePage = result;
  const target = $("file-list");
  target.replaceChildren();
  for (const entry of result.entries) {
    const b = node("button", "file-row");
    b.type = "button";
    const name = node("span", "file-name");
    name.append(
      icon(entry.type === "tree" ? "folder" : "branch"),
      node("span", "", entry.path),
    );
    b.append(name, badge(entry.type), node("code", "", short(entry.oid)));
    b.onclick = () => {
      if (entry.type === "tree") {
        $("file-query").value = entry.path + "/";
        work("Loading folder entries…", () => loadFiles(0));
        return;
      }
      work("Reading the immutable file…", async () => {
        preview = await api("file", {
          path: entry.path,
          snapshot: result.snapshot,
        });
        const bytes = Uint8Array.from(atob(preview.content), (c) =>
          c.charCodeAt(0),
        );
        $("preview-name").textContent = entry.path;
        $("preview-meta").textContent =
          entry.type +
          " · " +
          bytes.length.toLocaleString() +
          " bytes · " +
          short(result.snapshot);
        $("preview-content").textContent = bytes.includes(0)
          ? "Binary file. Download it to inspect its contents."
          : new TextDecoder().decode(bytes);
        $("preview-dialog").showModal();
      });
    };
    target.append(b);
  }
  if (!result.entries.length) {
    const empty = node("div", "empty-state");
    empty.append(
      node("h3", "", "No matching paths."),
      node(
        "p",
        "",
        "Try a shorter path fragment. This view includes only tracked entries.",
      ),
    );
    target.append(empty);
  }
  $("file-total").textContent = result.total
    ? `${result.offset + 1}–${result.offset + result.entries.length} of ${result.total} entries`
    : "0 entries";
  $("files-prev").disabled = offset === 0;
  $("files-next").disabled = !result.has_more;
}
$("file-search-form").onsubmit = (e) => {
  e.preventDefault();
  work("Finding tracked paths…", () => loadFiles(0));
};
$("files-prev").onclick = () =>
  work("Loading previous entries…", () =>
    loadFiles(Math.max(0, filePage.offset - 100)),
  );
$("files-next").onclick = () =>
  work("Loading more entries…", () => loadFiles(filePage.offset + 100));
$("preview-close").onclick = () => $("preview-dialog").close();
$("preview-download").onclick = () => {
  if (preview)
    download(
      new Blob([
        Uint8Array.from(atob(preview.content), (c) => c.charCodeAt(0)),
      ]),
      preview.path.split("/").pop(),
    );
};
function renderCommands() {
  const query = $("command-query").value.toLowerCase(),
    list = $("command-list");
  list.replaceChildren();
  for (const [page, name] of Object.entries(names)) {
    if (!name.toLowerCase().includes(query)) continue;
    const b = node("button", "command-item", name);
    b.type = "button";
    b.append(node("span", "", "↗"));
    b.onclick = () => {
      $("command-dialog").close();
      navigate(page);
    };
    list.append(b);
  }
}
function openCommands() {
  renderCommands();
  $("command-dialog").showModal();
  $("command-query").focus();
}
$("command-open").onclick = openCommands;
$("command-close").onclick = () => $("command-dialog").close();
$("command-query").oninput = renderCommands;
$("command-query").onkeydown = (e) => {
  if (e.key === "Enter") {
    $("command-list").querySelector("button")?.click();
  }
};
document.addEventListener("keydown", (e) => {
  if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "k") {
    e.preventDefault();
    openCommands();
  }
});

let profile = {
    name: "Local operator",
    mode: "system",
    scheme: "lime",
    motion: false,
    draft_path: "",
    draft: "",
    bookmarks: [],
  },
  profileLoaded = false,
  draftTimer,
  cloudTimer,
  cloudPending = null;
const systemDark = matchMedia("(prefers-color-scheme: dark)");
function applyAppearance(p) {
  document.documentElement.dataset.mode =
    p.mode === "system" ? (systemDark.matches ? "dark" : "light") : p.mode;
  document.documentElement.dataset.scheme = p.scheme;
  document.body.classList.toggle("reduced-motion", !!p.motion);
}
systemDark.addEventListener("change", () => applyAppearance(profile));
function renderProfile(p) {
  profile = p;
  applyAppearance(p);
  $("profile-label").textContent = p.name || "Local operator";
  $("profile-name").value = p.name || "";
  $("theme-mode").value = p.mode;
  $("reduce-motion").checked = p.motion;
  $("draft-summary").textContent = p.draft_path
    ? "Saved locally: " + p.draft_path
    : "No saved draft yet.";
  if (!profileLoaded) {
    profileLoaded = true;
    if (p.draft_path) {
      $("file-path").value = p.draft_path;
      $("file-content").value = p.draft;
    }
  }
  renderBookmarks();
}
async function saveProfile(p) {
  const saved = await api("profile", p);
  renderProfile(saved);
  state.profile = saved;
  return saved;
}
$("profile-form").onsubmit = (e) => {
  e.preventDefault();
  work("Saving your local profile…", async () => {
    await saveProfile({ ...profile, name: $("profile-name").value.trim() });
    toast("Profile saved on this computer.");
  });
};
async function saveDraft() {
  await saveProfile({
    ...profile,
    draft_path: $("file-path").value,
    draft: $("file-content").value,
  });
}
$("save-draft").onclick = () => work("Saving your draft…", saveDraft);
$("resume-draft").onclick = () => {
  if (!profile.draft_path) {
    toast("Prepare a file in Guarded publish to start a draft.");
    return;
  }
  $("file-path").value = profile.draft_path;
  $("file-content").value = profile.draft;
  invalidateReview();
  navigate("publish");
};
for (const id of ["file-path", "file-content"]) {
  $(id).addEventListener("input", () => {
    clearTimeout(draftTimer);
    draftTimer = setTimeout(async function persist() {
      if (busy) {
        draftTimer = setTimeout(persist, 1200);
        return;
      }
      try {
        await saveDraft();
      } catch {
        $("draft-summary").textContent =
          "Draft not saved. Use Save current draft to retry.";
      }
    }, 900);
  });
}
function renderBookmarks() {
  const list = $("bookmarks");
  list.replaceChildren();
  for (const url of profile.bookmarks || []) {
    const b = node(
      "button",
      "workspace-choice",
      url.replace("https://github.com/", ""),
    );
    b.type = "button";
    b.onclick = () => {
      navigate("settings");
      connectionTab("github");
      $("github-url").value = url;
      toast("Repository URL ready. Choose Import from GitHub to continue.");
    };
    list.append(b);
  }
}
$("bookmark-current").onclick = () =>
  work("Saving repository bookmark…", async () => {
    if (!state.repository_url)
      throw Error("Connect a GitHub repository first.");
    await saveProfile({
      ...profile,
      bookmarks: [
        ...new Set([...(profile.bookmarks || []), state.repository_url]),
      ],
    });
    toast("Repository bookmarked.");
  });
async function loadInsights() {
  const result = await api("insights"),
    target = $("account-insights");
  target.replaceChildren();
  for (const [key, label] of [
    ["workspaces", "Saved workspaces"],
    ["actions", "Recorded actions"],
    ["published", "Published operations"],
    ["model_trials", "Model trials"],
  ]) {
    const card = node("div", "metric");
    card.append(
      node("span", "metric-label", label),
      node("strong", "", String(result[key])),
    );
    target.append(card);
  }
}
let selectedScheme = "lime";
const schemes = [
  ["lime", "Citrus", 82],
  ["ocean", "Tidal", 205],
  ["violet", "Iris", 265],
  ["rose", "Bloom", 335],
  ["amber", "Ember", 38],
];
function drawSchemes() {
  const list = $("scheme-options");
  list.replaceChildren();
  for (const [id, label, hue] of schemes) {
    const b = node("button", "scheme-choice");
    b.type = "button";
    b.setAttribute("aria-pressed", String(selectedScheme === id));
    const dot = node("i");
    dot.style.setProperty("--swatch", hue);
    b.append(dot, node("span", "", label));
    b.onclick = () => {
      selectedScheme = id;
      applyAppearance({ ...profile, mode: $("theme-mode").value, scheme: id });
      drawSchemes();
    };
    list.append(b);
  }
}
$("appearance-open").onclick = () => {
  selectedScheme = profile.scheme;
  $("theme-mode").value = profile.mode;
  drawSchemes();
  $("appearance-dialog").showModal();
};
$("theme-mode").onchange = () =>
  applyAppearance({
    ...profile,
    mode: $("theme-mode").value,
    scheme: selectedScheme,
  });
$("appearance-close").onclick = () => $("appearance-dialog").close();
$("appearance-dialog").addEventListener("close", () =>
  applyAppearance(profile),
);
$("appearance-save").onclick = () =>
  work("Saving your appearance…", async () => {
    await saveProfile({
      ...profile,
      mode: $("theme-mode").value,
      scheme: selectedScheme,
    });
    $("appearance-dialog").close();
    toast("Appearance saved.");
  });
$("reduce-motion").onchange = () =>
  work("Saving animation preference…", () =>
    saveProfile({ ...profile, motion: $("reduce-motion").checked }),
  );
function renderCloud(c) {
  $("cloud-url").value = c.url || "";
  $("account-state").textContent = c.signed_in
    ? "Cloud connected · local work saved"
    : "Saved on this computer";
  $("cloud-description").textContent = c.signed_in
    ? "Signed in as " + c.email
    : c.pending
      ? "Complete sign-in in your browser. This window will reconnect."
      : c.configured
        ? "Cloud is configured. Sign in to save or restore portable work."
        : "Optional cloud sync needs your Supabase project configuration.";
  $("cloud-github").disabled = !c.configured || c.pending;
  $("cloud-google").disabled = !c.configured || c.pending;
  $("cloud-github").hidden = c.signed_in;
  $("cloud-google").hidden = c.signed_in;
  $("cloud-signout").hidden = !c.signed_in;
  $("cloud-save").disabled = !c.signed_in;
  $("cloud-list").disabled = !c.signed_in;
  if (c.pending && !cloudTimer) cloudTimer = setTimeout(pollCloud, 2000);
}
async function pollCloud() {
  cloudTimer = null;
  try {
    renderCloud(await api("cloud/status"));
  } catch {
    if (!document.body.classList.contains("session-locked"))
      cloudTimer = setTimeout(pollCloud, 3000);
  }
}
$("cloud-form").onsubmit = (e) => {
  e.preventDefault();
  work("Saving cloud configuration…", async () => {
    renderCloud(
      await api("cloud/configure", {
        url: $("cloud-url").value.trim(),
        key: $("cloud-key").value.trim(),
      }),
    );
    $("cloud-key").value = "";
    $("cloud-setup").open = false;
    toast(
      "Connection saved. Enable your provider and apply the database schema before signing in.",
    );
  });
};
for (const provider of ["github", "google"])
  $("cloud-" + provider).onclick = () =>
    work("Opening secure sign-in…", async () =>
      renderCloud(await api("cloud/sign-in", { provider })),
    );
$("cloud-signout").onclick = () =>
  work("Signing out…", async () => {
    renderCloud(await api("cloud/sign-out"));
    $("cloud-snapshots").replaceChildren();
    toast("Signed out locally. Your local work remains saved.");
  });
function renderSaves(saves) {
  const target = $("cloud-snapshots");
  target.replaceChildren();
  if (!saves.length)
    target.append(
      node(
        "p",
        "field-note",
        "No cloud saves yet. Your local work is ready to save.",
      ),
    );
  for (const snapshot of saves) {
    const b = node(
      "button",
      "cloud-snapshot",
      snapshot.payload.name || "Saved profile",
    );
    b.append(
      node(
        "small",
        "",
        new Date(snapshot.created_at).toLocaleString() +
          " · " +
          (snapshot.payload.bookmarks || []).length +
          " bookmarks",
      ),
    );
    b.onclick = () => {
      cloudPending = snapshot.payload;
      $("restore-preview").textContent = JSON.stringify(
        snapshot.payload,
        null,
        2,
      );
      $("cloud-restore-dialog").showModal();
    };
    target.append(b);
  }
}
$("cloud-save").onclick = () =>
  work("Saving portable work to your account…", async () => {
    clearTimeout(draftTimer);
    await saveDraft();
    renderSaves(await api("cloud/save"));
    toast("Cloud snapshot saved.");
  });
$("cloud-list").onclick = () =>
  work("Finding saved work…", async () => renderSaves(await api("cloud/list")));
$("restore-close").onclick = () => $("cloud-restore-dialog").close();
$("restore-confirm").onclick = () =>
  work("Restoring your chosen snapshot…", async () => {
    if (!cloudPending) return;
    clearTimeout(draftTimer);
    await saveProfile(cloudPending);
    $("file-path").value = profile.draft_path;
    $("file-content").value = profile.draft;
    invalidateReview();
    $("cloud-restore-dialog").close();
    toast("Portable work restored. Open a bookmarked repository to continue.");
  });
navigate(location.hash.slice(1));
if (token) work("Opening your local workspace…", refresh);
else showSessionGate();
