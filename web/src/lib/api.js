const BASE = '';

function getToken() {
  return localStorage.getItem('dj_token');
}

function authHeaders() {
  const token = getToken();
  const headers = { 'Content-Type': 'application/json' };
  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }
  return headers;
}

async function request(method, path, body = null, options = {}) {
  const config = {
    method,
    headers: options.noAuth ? { 'Content-Type': 'application/json' } : authHeaders(),
  };

  if (body !== null) {
    if (body instanceof Blob || body instanceof ArrayBuffer || typeof body === 'string') {
      config.body = body;
    } else {
      config.body = JSON.stringify(body);
    }
  }

  if (options.rawBody) {
    config.body = options.rawBody;
    if (!options.noAuth) {
      config.headers = { ...authHeaders() };
    }
    delete config.headers['Content-Type'];
  }

  const res = await fetch(`${BASE}${path}`, config);

  if (res.status === 401) {
    const event = new CustomEvent('auth:unauthorized');
    window.dispatchEvent(event);
  }

  if (options.raw) {
    return res;
  }

  const data = await res.json();

  if (!res.ok) {
    throw new ApiError(data.error || res.statusText, res.status, data);
  }

  return data;
}

export class ApiError extends Error {
  constructor(message, status, data) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.data = data;
  }
}

// ---------------------------------------------------------------------------
// Auth
// ---------------------------------------------------------------------------

export async function login(username, password) {
  const data = await request('POST', '/api/auth/login', { username, password }, { noAuth: true });
  if (data.token) {
    localStorage.setItem('dj_token', data.token);
    localStorage.setItem('dj_username', data.username);
  }
  return data;
}

export async function verifyToken() {
  return request('GET', '/api/auth/verify');
}

export function logout() {
  localStorage.removeItem('dj_token');
  localStorage.removeItem('dj_username');
}

export function isLoggedIn() {
  return !!getToken();
}

export function getUsername() {
  return localStorage.getItem('dj_username') || '';
}

// ---------------------------------------------------------------------------
// Public status
// ---------------------------------------------------------------------------

export async function getStatus() {
  return request('GET', '/api/status', null, { noAuth: true });
}

export async function getHealth() {
  return request('GET', '/health', null, { noAuth: true });
}

// ---------------------------------------------------------------------------
// Tracks
// ---------------------------------------------------------------------------

export async function listTracks() {
  return request('GET', '/api/tracks', null, { noAuth: true });
}

export async function getTrack(id) {
  return request('GET', `/api/tracks/${id}`, null, { noAuth: true });
}

export async function listOrphanedTracks() {
  return request('GET', '/api/tracks/orphaned');
}

// ---------------------------------------------------------------------------
// Playlists
// ---------------------------------------------------------------------------

export async function listPlaylists() {
  return request('GET', '/api/playlists', null, { noAuth: true });
}

export async function getPlaylist(id) {
  return request('GET', `/api/playlists/${id}`, null, { noAuth: true });
}

export async function createPlaylist(name, tag) {
  return request('POST', '/api/playlists', { name, tag });
}

export async function updatePlaylist(id, updates) {
  return request('PUT', `/api/playlists/${id}`, updates);
}

export async function deletePlaylist(id) {
  return request('DELETE', `/api/playlists/${id}`);
}

// ---------------------------------------------------------------------------
// Playlist track manipulation
// ---------------------------------------------------------------------------

export async function addTrackToPlaylist(playlistId, { trackId, checksum, filePath, index } = {}) {
  const body = {};
  if (trackId !== undefined) body.trackId = trackId;
  if (checksum !== undefined) body.checksum = checksum;
  if (filePath !== undefined) body.filePath = filePath;
  if (index !== undefined) body.index = index;
  return request('POST', `/api/playlists/${playlistId}/tracks`, body);
}

export async function removeTrackFromPlaylist(playlistId, trackId) {
  return request('DELETE', `/api/playlists/${playlistId}/tracks/${trackId}`);
}

export async function moveTrackInPlaylist(playlistId, from, to) {
  return request('POST', `/api/playlists/${playlistId}/tracks/move`, { from, to });
}

export async function shufflePlaylist(playlistId) {
  return request('POST', `/api/playlists/${playlistId}/shuffle`);
}

// ---------------------------------------------------------------------------
// Playlist import / export
// ---------------------------------------------------------------------------

export async function exportPlaylist(id) {
  const res = await request('GET', `/api/playlists/${id}/export`, null, { raw: true });
  if (!res.ok) {
    const data = await res.json();
    throw new ApiError(data.error || res.statusText, res.status, data);
  }
  const blob = await res.blob();
  const disposition = res.headers.get('Content-Disposition') || '';
  const match = disposition.match(/filename="?([^"]+)"?/);
  const filename = match ? match[1] : `playlist_${id}.json`;
  return { blob, filename };
}

export async function importPlaylist(jsonString) {
  return request('POST', '/api/playlists/import', null, { rawBody: jsonString });
}

// ---------------------------------------------------------------------------
// Master playlist
// ---------------------------------------------------------------------------

export async function getMasterPlaylist() {
  return request('GET', '/api/master', null, { noAuth: true });
}

export async function assignPlaylistToTag(tag, playlistId) {
  return request('PUT', `/api/master/${tag}`, { playlistId });
}

export async function removePlaylistFromTag(tag, playlistId) {
  return request('DELETE', `/api/master/${tag}/${playlistId}`);
}

// ---------------------------------------------------------------------------
// Scheduler
// ---------------------------------------------------------------------------

export async function getSchedulerStatus() {
  return request('GET', '/api/scheduler/status', null, { noAuth: true });
}

// ---------------------------------------------------------------------------
// Reconcile / hot-reload
// ---------------------------------------------------------------------------

export async function reconcile() {
  return request('POST', '/api/reconcile');
}
