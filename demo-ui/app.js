const qs = (id) => document.getElementById(id);

const state = {
  accessToken: sessionStorage.getItem("demo_ui_access_token") || "",
};

const output = qs("output");
const authState = qs("authState");

function apiBase() {
  return qs("apiBaseUrl").value.trim().replace(/\/+$/, "");
}

function orgID() {
  return qs("orgId").value.trim();
}

function showJSON(value) {
  output.textContent = JSON.stringify(value, null, 2);
}

function showError(status, message, details = "") {
  showJSON({
    error: {
      status,
      message,
      details,
    },
  });
}

function setLoggedInState(email) {
  authState.textContent = `Logged in as ${email}`;
}

async function request(method, path, body = null) {
  const url = `${apiBase()}${path}`;
  const headers = { "Content-Type": "application/json" };
  if (state.accessToken) {
    headers.Authorization = `Bearer ${state.accessToken}`;
  }
  if (orgID()) {
    headers["X-Organization-ID"] = orgID();
  }

  const resp = await fetch(url, {
    method,
    headers,
    body: body ? JSON.stringify(body) : undefined,
  });

  const text = await resp.text();
  let parsed = null;
  if (text) {
    try {
      parsed = JSON.parse(text);
    } catch (_err) {
      parsed = { raw: text };
    }
  }

  if (!resp.ok) {
    const code = parsed?.code || "request_failed";
    const msg = parsed?.message || "Request failed";
    throw new Error(`HTTP ${resp.status} ${code}: ${msg}`);
  }

  return { status: resp.status, data: parsed };
}

async function withOutput(fn) {
  try {
    const result = await fn();
    showJSON(result);
  } catch (err) {
    showError(400, "API request failed", String(err.message || err));
  }
}

function initDefaults() {
  const fromUrl = new URLSearchParams(window.location.search).get("baseUrl");
  qs("apiBaseUrl").value = fromUrl || "http://localhost:8080";

  const today = new Date().toISOString().slice(0, 10);
  qs("invoiceFromDate").value = today;
  qs("invoiceToDate").value = today;
}

async function login() {
  const email = qs("email").value.trim();
  const password = qs("password").value;
  const { data } = await request("POST", "/auth/login", { email, password });
  state.accessToken = data.access_token;
  sessionStorage.setItem("demo_ui_access_token", state.accessToken);
  setLoggedInState(email);
  return { route: "POST /auth/login", response: data };
}

function bindUI() {
  qs("loginBtn").addEventListener("click", () => withOutput(login));
  qs("meBtn").addEventListener("click", () => withOutput(() => request("GET", "/v1/me")));
  qs("clientsBtn").addEventListener("click", () => withOutput(() => request("GET", "/v1/clients")));
  qs("projectsBtn").addEventListener("click", () => withOutput(() => request("GET", "/v1/projects")));
  qs("timeEntriesBtn").addEventListener("click", () => withOutput(() => request("GET", "/v1/time-entries")));

  qs("submitBtn").addEventListener("click", () => withOutput(() => {
    const id = qs("timeEntryId").value.trim();
    return request("POST", `/v1/time-entries/${id}/submit`, {});
  }));
  qs("approveBtn").addEventListener("click", () => withOutput(() => {
    const id = qs("timeEntryId").value.trim();
    return request("POST", `/v1/time-entries/${id}/approve`, {});
  }));
  qs("generateInvoiceBtn").addEventListener("click", () => withOutput(() => request("POST", "/v1/invoices/generate", {
    from_date: qs("invoiceFromDate").value,
    to_date: qs("invoiceToDate").value,
    currency: "USD",
  })));
  qs("sendInvoiceBtn").addEventListener("click", () => withOutput(() => {
    const id = qs("invoiceId").value.trim();
    return request("POST", `/v1/invoices/${id}/send`, {});
  }));
}

initDefaults();
bindUI();
if (state.accessToken) {
  authState.textContent = "Loaded token from session storage.";
}
