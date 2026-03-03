/**
 * ChatterChat — frontend app
 *
 * CONFIG is loaded from config.js (gitignored).
 * Copy config.js.example to config.js and fill in your values.
 */

// Synthetic JWT used in local dev mode.
const LOCAL_DEV_TOKEN = [
  btoa('{"alg":"none","typ":"JWT"}'),
  btoa(
    JSON.stringify({
      sub: "dev-sub-001",
      "cognito:username": "devuser",
      email: "dev@local.dev",
      exp: 9999999999,
    }),
  ),
  "localsig",
].join(".");

function apiBase() {
  return CONFIG.localDev ? CONFIG.localApiBase : CONFIG.apiBase;
}

function wsBase() {
  return CONFIG.localDev ? CONFIG.localWsBase : CONFIG.wsBase;
}

// ---------------------------------------------------------------------------
// Cognito auth (amazon-cognito-identity-js)
// ---------------------------------------------------------------------------

function initCognito() {
  const poolData = {
    UserPoolId: CONFIG.userPoolId,
    ClientId: CONFIG.clientId,
  };
  return new AmazonCognitoIdentity.CognitoUserPool(poolData);
}

function signIn(email, password) {
  return new Promise((resolve, reject) => {
    const userPool = initCognito();
    const authDetails = new AmazonCognitoIdentity.AuthenticationDetails({
      Username: email,
      Password: password,
    });
    const cognitoUser = new AmazonCognitoIdentity.CognitoUser({
      Username: email,
      Pool: userPool,
    });
    cognitoUser.authenticateUser(authDetails, {
      onSuccess(session) {
        const idToken = session.getIdToken().getJwtToken();
        sessionStorage.setItem("id_token", idToken);
        resolve(idToken);
      },
      onFailure(err) {
        reject(err);
      },
    });
  });
}

function register(email, password) {
  return new Promise((resolve, reject) => {
    const userPool = initCognito();
    userPool.signUp(email, password, [], null, (err, result) => {
      if (err) {
        reject(err);
        return;
      }
      resolve(result.user);
    });
  });
}

function verifyEmail(email, code) {
  return new Promise((resolve, reject) => {
    const userPool = initCognito();
    const cognitoUser = new AmazonCognitoIdentity.CognitoUser({
      Username: email,
      Pool: userPool,
    });
    cognitoUser.confirmRegistration(code, true, (err, result) => {
      if (err) {
        reject(err);
        return;
      }
      resolve(result);
    });
  });
}

function resendCode(email) {
  return new Promise((resolve, reject) => {
    const userPool = initCognito();
    const cognitoUser = new AmazonCognitoIdentity.CognitoUser({
      Username: email,
      Pool: userPool,
    });
    cognitoUser.resendConfirmationCode((err, result) => {
      if (err) {
        reject(err);
        return;
      }
      resolve(result);
    });
  });
}

// Kept for local dev compatibility (no longer used in prod).
async function exchangeCodeForTokens(code) {
  const verifier = sessionStorage.getItem("pkce_verifier");
  if (!verifier) throw new Error("Missing PKCE verifier");
  const resp = await fetch(`https://${CONFIG.cognitoDomain}/oauth2/token`, {
    method: "POST",
    headers: { "Content-Type": "application/x-www-form-urlencoded" },
    body: new URLSearchParams({
      grant_type: "authorization_code",
      client_id: CONFIG.clientId,
      code,
      redirect_uri: CONFIG.redirectUri,
      code_verifier: verifier,
    }),
  });
  if (!resp.ok) throw new Error(`Token exchange failed: ${resp.status}`);
  return resp.json();
}

// ---------------------------------------------------------------------------
// Auth helpers
// ---------------------------------------------------------------------------

function decodeJWT(token) {
  const [, payload] = token.split(".");
  return JSON.parse(atob(payload.replace(/-/g, "+").replace(/_/g, "/")));
}

function signOut() {
  if (!CONFIG.localDev) {
    const cognitoUser = initCognito().getCurrentUser();
    if (cognitoUser) cognitoUser.signOut(); // clears SDK's localStorage keys
  }
  sessionStorage.clear();
  window.location.href = "index.html";
}

async function restoreSession() {
  return new Promise((resolve, reject) => {
    const userPool = initCognito();
    const cognitoUser = userPool.getCurrentUser();
    if (!cognitoUser) {
      reject(new Error("no session"));
      return;
    }
    cognitoUser.getSession((err, session) => {
      if (err || !session || !session.isValid()) {
        reject(err || new Error("invalid session"));
        return;
      }
      const token = session.getIdToken().getJwtToken();
      sessionStorage.setItem("id_token", token);
      resolve(token);
    });
  });
}

// ---------------------------------------------------------------------------
// REST helpers
// ---------------------------------------------------------------------------

function idToken() {
  return sessionStorage.getItem("id_token");
}

async function apiFetch(path, opts = {}) {
  const resp = await fetch(`${apiBase()}${path}`, {
    ...opts,
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${idToken()}`,
      ...(opts.headers || {}),
    },
  });
  if (!resp.ok) throw new Error(`API ${path} → ${resp.status}`);
  return resp.json();
}

// ---------------------------------------------------------------------------
// WebSocket
// ---------------------------------------------------------------------------

let ws = null;
let currentRoomId = null;

function connectWebSocket() {
  const token = idToken();
  if (!token) return;

  ws = new WebSocket(`${wsBase()}?token=${encodeURIComponent(token)}`);

  ws.addEventListener("open", () => console.log("WS connected"));
  ws.addEventListener("close", (e) => {
    console.warn("WS closed", e.code, e.reason);
    if (idToken()) setTimeout(connectWebSocket, 3000);
  });
  ws.addEventListener("error", (e) => console.error("WS error", e));
  ws.addEventListener("message", (e) => handleWSMessage(JSON.parse(e.data)));
}

function disconnectWebSocket() {
  if (ws) {
    ws.onclose = null; // suppress auto-reconnect
    ws.close();
    ws = null;
  }
}

function wsSend(payload) {
  if (ws && ws.readyState === WebSocket.OPEN) {
    ws.send(JSON.stringify(payload));
  }
}

function joinRoom(roomId) {
  currentRoomId = roomId;
  wsSend({ action: "joinRoom", room_id: roomId });
}

function sendMessage(body) {
  if (!currentRoomId || !body.trim()) return;
  wsSend({ action: "sendMessage", room_id: currentRoomId, body: body.trim() });
}

// ---------------------------------------------------------------------------
// Message handling
// ---------------------------------------------------------------------------

function handleWSMessage(msg) {
  switch (msg.type) {
    case "message":
      if (msg.room_id === currentRoomId) appendMessage(msg);
      break;
    case "joined":
      document.getElementById("current-room-name").textContent =
        `# ${msg.room_name}`;
      break;
    case "user_joined":
      if (msg.room_id === currentRoomId)
        appendSystemMsg(`${msg.username} joined`);
      break;
    case "user_left":
      if (msg.room_id === currentRoomId)
        appendSystemMsg(`${msg.username} left`);
      break;
    case "error":
      console.error("Server error:", msg.code, msg.message);
      break;
    case "pong":
      break;
    default:
      console.warn("Unknown WS message type:", msg.type);
  }
}

function appendMessage(msg) {
  const area = document.getElementById("message-area");
  area.querySelector(".placeholder")?.remove();

  const time = new Date(msg.created_at).toLocaleTimeString([], {
    hour: "2-digit",
    minute: "2-digit",
  });
  const el = document.createElement("div");
  el.className = "msg";
  el.innerHTML = `
    <div class="msg-content">
      <div class="msg-meta">
        <span class="msg-username">${escapeHTML(msg.username)}</span>
        <span class="msg-time">${time}</span>
      </div>
      <div class="msg-body">${escapeHTML(msg.body)}</div>
    </div>`;
  area.appendChild(el);
  area.scrollTop = area.scrollHeight;
}

function appendSystemMsg(text) {
  const area = document.getElementById("message-area");
  const el = document.createElement("div");
  el.className = "msg-system";
  el.textContent = text;
  area.appendChild(el);
  area.scrollTop = area.scrollHeight;
}

function escapeHTML(str) {
  return str
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#x27;");
}

// ---------------------------------------------------------------------------
// Room list
// ---------------------------------------------------------------------------

async function loadRooms() {
  const rooms = await apiFetch("/rooms");
  const list = document.getElementById("room-list");
  list.innerHTML = "";
  for (const room of rooms) {
    const li = document.createElement("li");
    li.textContent = `# ${room.name}`;
    li.dataset.roomId = room.id;
    li.addEventListener("click", () => selectRoom(room, li));
    list.appendChild(li);
  }
}

async function selectRoom(room, li) {
  document
    .querySelectorAll(".room-list li")
    .forEach((el) => el.classList.remove("active"));
  li.classList.add("active");

  document.getElementById("message-area").innerHTML = "";
  document.getElementById("message-form-wrapper").style.display = "flex";

  joinRoom(room.id);

  try {
    const msgs = await apiFetch(`/rooms/${room.id}/messages?limit=50`);
    for (const msg of msgs) appendMessage(msg);
  } catch (err) {
    console.error("Failed to load history:", err);
  }
}

// ---------------------------------------------------------------------------
// Display name modal
// ---------------------------------------------------------------------------

function showDisplayNameModal() {
  document.getElementById("username-modal").style.display = "flex";
  document.getElementById("display-name-input").focus();
}

function hideDisplayNameModal() {
  document.getElementById("username-modal").style.display = "none";
  document.getElementById("modal-error").textContent = "";
}

async function saveDisplayName() {
  const input = document.getElementById("display-name-input");
  const name = input.value.trim();
  const errorEl = document.getElementById("modal-error");

  if (name.length < 3 || name.length > 30) {
    errorEl.textContent = "Name must be 3–30 characters.";
    return;
  }

  const saveBtn = document.getElementById("display-name-save");
  saveBtn.disabled = true;
  saveBtn.textContent = "Saving…";
  errorEl.textContent = "";

  try {
    const user = await apiFetch("/users/me", {
      method: "PUT",
      body: JSON.stringify({ display_name: name }),
    });
    const effectiveName = user.display_name || user.email;
    document.getElementById("current-username").textContent = effectiveName;
    hideDisplayNameModal();

    // Reconnect WS so new messages use updated display name.
    disconnectWebSocket();
    connectWebSocket();
  } catch (err) {
    errorEl.textContent = "Failed to save. Please try again.";
    console.error("Save display name:", err);
  } finally {
    saveBtn.disabled = false;
    saveBtn.textContent = "Save";
  }
}

// ---------------------------------------------------------------------------
// Page init — Index
// ---------------------------------------------------------------------------

async function initIndexPage() {
  // Local dev: skip auth entirely.
  if (CONFIG.localDev) {
    sessionStorage.setItem("id_token", LOCAL_DEV_TOKEN);
    window.location.href = "chat.html";
    return;
  }

  if (sessionStorage.getItem("id_token")) {
    window.location.href = "chat.html";
    return;
  }

  try {
    await restoreSession();
    window.location.href = "chat.html";
    return;
  } catch (e) {
    // No valid session — show sign-in form as normal
  }

  // Tab switching
  document.querySelectorAll(".auth-tab-btn").forEach((btn) => {
    btn.addEventListener("click", () => {
      document
        .querySelectorAll(".auth-tab-btn")
        .forEach((b) => b.classList.remove("active"));
      btn.classList.add("active");
      const tab = btn.dataset.tab;
      document.getElementById("tab-signin").style.display =
        tab === "signin" ? "block" : "none";
      document.getElementById("tab-register").style.display =
        tab === "register" ? "block" : "none";
      document.getElementById("tab-verify").style.display = "none";
    });
  });

  // Sign In
  document.getElementById("signin-btn").addEventListener("click", async () => {
    const email = document.getElementById("signin-email").value.trim();
    const password = document.getElementById("signin-password").value;
    const errorEl = document.getElementById("signin-error");
    const btn = document.getElementById("signin-btn");

    errorEl.textContent = "";
    btn.disabled = true;
    btn.textContent = "Signing in…";
    try {
      await signIn(email, password);
      window.location.href = "chat.html";
    } catch (err) {
      errorEl.textContent = err.message || "Sign in failed.";
    } finally {
      btn.disabled = false;
      btn.textContent = "Sign In";
    }
  });

  // Register
  let pendingEmail = "";
  document
    .getElementById("register-btn")
    .addEventListener("click", async () => {
      const email = document.getElementById("register-email").value.trim();
      const password = document.getElementById("register-password").value;
      const errorEl = document.getElementById("register-error");
      const btn = document.getElementById("register-btn");

      errorEl.textContent = "";
      btn.disabled = true;
      btn.textContent = "Creating…";
      try {
        await register(email, password);
        pendingEmail = email;
        // Show verify panel
        document
          .querySelectorAll(".auth-tab-btn")
          .forEach((b) => b.classList.remove("active"));
        document.getElementById("tab-signin").style.display = "none";
        document.getElementById("tab-register").style.display = "none";
        document.getElementById("tab-verify").style.display = "block";
      } catch (err) {
        errorEl.textContent = err.message || "Registration failed.";
      } finally {
        btn.disabled = false;
        btn.textContent = "Create Account";
      }
    });

  // Verify
  document.getElementById("verify-btn").addEventListener("click", async () => {
    const code = document.getElementById("verify-code").value.trim();
    const errorEl = document.getElementById("verify-error");
    const btn = document.getElementById("verify-btn");

    errorEl.textContent = "";
    btn.disabled = true;
    btn.textContent = "Verifying…";
    try {
      await verifyEmail(pendingEmail, code);
      // Auto sign-in after verification
      const password = document.getElementById("register-password").value;
      await signIn(pendingEmail, password);
      window.location.href = "chat.html";
    } catch (err) {
      errorEl.textContent = err.message || "Verification failed.";
    } finally {
      btn.disabled = false;
      btn.textContent = "Verify";
    }
  });

  // Resend
  document.getElementById("resend-btn").addEventListener("click", async () => {
    const errorEl = document.getElementById("verify-error");
    try {
      await resendCode(pendingEmail);
      errorEl.style.color = "var(--muted)";
      errorEl.textContent = "Code resent.";
    } catch (err) {
      errorEl.style.color = "";
      errorEl.textContent = err.message || "Failed to resend.";
    }
  });

  // Allow Enter key in sign-in fields
  document
    .getElementById("signin-password")
    .addEventListener("keydown", (e) => {
      if (e.key === "Enter") document.getElementById("signin-btn").click();
    });
  document.getElementById("verify-code").addEventListener("keydown", (e) => {
    if (e.key === "Enter") document.getElementById("verify-btn").click();
  });
}

// ---------------------------------------------------------------------------
// Page init — Chat
// ---------------------------------------------------------------------------

async function initChatPage() {
  if (CONFIG.localDev) {
    if (!sessionStorage.getItem("id_token")) {
      sessionStorage.setItem("id_token", LOCAL_DEV_TOKEN);
    }
  }

  if (!idToken()) {
    try {
      await restoreSession();
    } catch (e) {
      window.location.href = "index.html";
      return;
    }
  }

  // Fetch user record from API (includes display_name).
  let user;
  try {
    user = await apiFetch("/users/me");
  } catch (err) {
    // Fallback: parse token claims for display
    console.error("Failed to fetch user:", err);
    const claims = decodeJWT(idToken());
    user = {
      email: claims.email || claims["cognito:username"] || "You",
      display_name: null,
    };
  }

  const effectiveName = user.display_name || user.email;
  document.getElementById("current-username").textContent = effectiveName;
  document.getElementById("sign-out-btn").addEventListener("click", signOut);

  // Edit username button
  document.getElementById("edit-username-btn").addEventListener("click", () => {
    document.getElementById("display-name-input").value =
      user.display_name || "";
    showDisplayNameModal();
  });

  // Modal save/cancel
  document
    .getElementById("display-name-save")
    .addEventListener("click", saveDisplayName);
  document
    .getElementById("display-name-input")
    .addEventListener("keydown", (e) => {
      if (e.key === "Enter") saveDisplayName();
      if (e.key === "Escape") hideDisplayNameModal();
    });
  document.getElementById("username-modal").addEventListener("click", (e) => {
    if (e.target === e.currentTarget) hideDisplayNameModal();
  });

  // Show modal if no display name set yet
  if (!user.display_name) {
    showDisplayNameModal();
  }

  connectWebSocket();
  await loadRooms();

  const input = document.getElementById("message-input");
  const sendBtn = document.getElementById("send-btn");

  sendBtn.addEventListener("click", () => {
    sendMessage(input.value);
    input.value = "";
  });
  input.addEventListener("keydown", (e) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      sendMessage(input.value);
      input.value = "";
    }
  });

  setInterval(() => wsSend({ action: "ping" }), 5 * 60 * 1000);
}

// ---------------------------------------------------------------------------
// Router
// ---------------------------------------------------------------------------

if (document.querySelector(".auth-page")) {
  initIndexPage();
} else if (document.querySelector(".chat-page")) {
  initChatPage();
}
