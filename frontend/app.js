/**
 * ChatterChat — frontend app
 *
 * CONFIG is loaded from config.js (gitignored).
 * Copy config.js.example to config.js and fill in your values.
 */

// Synthetic JWT used in local dev mode (backend ignores it; frontend uses it for UI display).
const LOCAL_DEV_TOKEN = [
  btoa('{"alg":"none","typ":"JWT"}'),
  btoa(JSON.stringify({ sub: 'dev-sub-001', 'cognito:username': 'devuser', email: 'dev@local.dev', exp: 9999999999 })),
  'localsig'
].join('.');

function apiBase() {
  return CONFIG.localDev ? CONFIG.localApiBase : CONFIG.apiBase;
}

function wsBase() {
  return CONFIG.localDev ? CONFIG.localWsBase : CONFIG.wsBase;
}

// ---------------------------------------------------------------------------
// PKCE helpers
// ---------------------------------------------------------------------------

function base64urlEncode(buffer) {
  return btoa(String.fromCharCode(...new Uint8Array(buffer)))
    .replace(/\+/g, '-').replace(/\//g, '_').replace(/=/g, '');
}

async function generatePKCE() {
  const verifier = base64urlEncode(crypto.getRandomValues(new Uint8Array(64)));
  const encoded = new TextEncoder().encode(verifier);
  const hash = await crypto.subtle.digest('SHA-256', encoded);
  const challenge = base64urlEncode(hash);
  return { verifier, challenge };
}

// ---------------------------------------------------------------------------
// Auth flow
// ---------------------------------------------------------------------------

async function startLogin() {
  const { verifier, challenge } = await generatePKCE();
  sessionStorage.setItem('pkce_verifier', verifier);

  const params = new URLSearchParams({
    response_type: 'code',
    client_id: CONFIG.clientId,
    redirect_uri: CONFIG.redirectUri,
    scope: 'openid email profile',
    code_challenge: challenge,
    code_challenge_method: 'S256',
  });

  window.location.href = `https://${CONFIG.cognitoDomain}/oauth2/authorize?${params}`;
}

async function exchangeCodeForTokens(code) {
  const verifier = sessionStorage.getItem('pkce_verifier');
  if (!verifier) throw new Error('Missing PKCE verifier');

  const resp = await fetch(`https://${CONFIG.cognitoDomain}/oauth2/token`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
    body: new URLSearchParams({
      grant_type: 'authorization_code',
      client_id: CONFIG.clientId,
      code,
      redirect_uri: CONFIG.redirectUri,
      code_verifier: verifier,
    }),
  });

  if (!resp.ok) throw new Error(`Token exchange failed: ${resp.status}`);
  return resp.json(); // { id_token, access_token, refresh_token, ... }
}

function decodeJWT(token) {
  const [, payload] = token.split('.');
  return JSON.parse(atob(payload.replace(/-/g, '+').replace(/_/g, '/')));
}

function signOut() {
  sessionStorage.clear();
  window.location.href = 'index.html';
}

// ---------------------------------------------------------------------------
// REST helpers
// ---------------------------------------------------------------------------

function idToken() { return sessionStorage.getItem('id_token'); }

async function apiFetch(path, opts = {}) {
  const resp = await fetch(`${apiBase()}${path}`, {
    ...opts,
    headers: {
      'Content-Type': 'application/json',
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

  ws.addEventListener('open', () => console.log('WS connected'));
  ws.addEventListener('close', (e) => {
    console.warn('WS closed', e.code, e.reason);
    // Reconnect after 3 s unless user signed out.
    if (idToken()) setTimeout(connectWebSocket, 3000);
  });
  ws.addEventListener('error', (e) => console.error('WS error', e));
  ws.addEventListener('message', (e) => handleWSMessage(JSON.parse(e.data)));
}

function wsSend(payload) {
  if (ws && ws.readyState === WebSocket.OPEN) {
    ws.send(JSON.stringify(payload));
  }
}

function joinRoom(roomId) {
  currentRoomId = roomId;
  wsSend({ action: 'joinRoom', room_id: roomId });
}

function sendMessage(body) {
  if (!currentRoomId || !body.trim()) return;
  wsSend({ action: 'sendMessage', room_id: currentRoomId, body: body.trim() });
}

// ---------------------------------------------------------------------------
// Message handling
// ---------------------------------------------------------------------------

function handleWSMessage(msg) {
  switch (msg.type) {
    case 'message':
      if (msg.room_id === currentRoomId) appendMessage(msg);
      break;
    case 'joined':
      document.getElementById('current-room-name').textContent = `# ${msg.room_name}`;
      break;
    case 'user_joined':
      if (msg.room_id === currentRoomId) appendSystemMsg(`${msg.username} joined`);
      break;
    case 'user_left':
      if (msg.room_id === currentRoomId) appendSystemMsg(`${msg.username} left`);
      break;
    case 'error':
      console.error('Server error:', msg.code, msg.message);
      break;
    case 'pong':
      break;
    default:
      console.warn('Unknown WS message type:', msg.type);
  }
}

function appendMessage(msg) {
  const area = document.getElementById('message-area');
  // Remove placeholder if present.
  area.querySelector('.placeholder')?.remove();

  const time = new Date(msg.created_at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
  const el = document.createElement('div');
  el.className = 'msg';
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
  const area = document.getElementById('message-area');
  const el = document.createElement('div');
  el.className = 'msg-system';
  el.textContent = text;
  area.appendChild(el);
  area.scrollTop = area.scrollHeight;
}

function escapeHTML(str) {
  return str.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;').replace(/'/g, '&#x27;');
}

// ---------------------------------------------------------------------------
// Room list
// ---------------------------------------------------------------------------

async function loadRooms() {
  const rooms = await apiFetch('/rooms');
  const list = document.getElementById('room-list');
  list.innerHTML = '';
  for (const room of rooms) {
    const li = document.createElement('li');
    li.textContent = `# ${room.name}`;
    li.dataset.roomId = room.id;
    li.addEventListener('click', () => selectRoom(room, li));
    list.appendChild(li);
  }
}

async function selectRoom(room, li) {
  document.querySelectorAll('.room-list li').forEach(el => el.classList.remove('active'));
  li.classList.add('active');

  document.getElementById('message-area').innerHTML = '';
  document.getElementById('message-form-wrapper').style.display = 'flex';

  joinRoom(room.id);

  // Load message history.
  try {
    const msgs = await apiFetch(`/rooms/${room.id}/messages?limit=50`);
    for (const msg of msgs) appendMessage(msg);
  } catch (err) {
    console.error('Failed to load history:', err);
  }
}

// ---------------------------------------------------------------------------
// Page init
// ---------------------------------------------------------------------------

async function initIndexPage() {
  // Local dev: skip PKCE entirely and go straight to chat.
  if (CONFIG.localDev) {
    sessionStorage.setItem('id_token', LOCAL_DEV_TOKEN);
    window.location.href = 'chat.html';
    return;
  }

  // If already authenticated, go straight to chat.
  if (sessionStorage.getItem('id_token')) {
    window.location.href = 'chat.html';
    return;
  }
  document.getElementById('sign-in-btn').addEventListener('click', startLogin);
}

async function initChatPage() {
  // Local dev: ensure synthetic token is set and skip the Cognito code exchange.
  if (CONFIG.localDev) {
    if (!sessionStorage.getItem('id_token')) {
      sessionStorage.setItem('id_token', LOCAL_DEV_TOKEN);
    }
  }

  // Handle redirect back from Cognito.
  const params = new URLSearchParams(window.location.search);
  const code = params.get('code');
  if (!CONFIG.localDev && code) {
    history.replaceState({}, '', window.location.pathname);
    try {
      const tokens = await exchangeCodeForTokens(code);
      sessionStorage.setItem('id_token', tokens.id_token);
      sessionStorage.removeItem('pkce_verifier');
    } catch (err) {
      console.error('Token exchange failed:', err);
      window.location.href = 'index.html';
      return;
    }
  }

  if (!idToken()) {
    window.location.href = 'index.html';
    return;
  }

  const claims = decodeJWT(idToken());
  const username = claims['cognito:username'] || claims.email || 'You';
  document.getElementById('current-username').textContent = username;
  document.getElementById('sign-out-btn').addEventListener('click', signOut);

  connectWebSocket();
  await loadRooms();

  // Send message on Enter / button click.
  const input = document.getElementById('message-input');
  const sendBtn = document.getElementById('send-btn');

  sendBtn.addEventListener('click', () => {
    sendMessage(input.value);
    input.value = '';
  });
  input.addEventListener('keydown', (e) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      sendMessage(input.value);
      input.value = '';
    }
  });

  // Keepalive ping every 5 minutes.
  setInterval(() => wsSend({ action: 'ping' }), 5 * 60 * 1000);
}

// Route to the right init function based on current page.
if (document.querySelector('.auth-page')) {
  initIndexPage();
} else if (document.querySelector('.chat-page')) {
  initChatPage();
}
