import { writable, derived } from 'svelte/store';
import { getStatus, listPlaylists, getMasterPlaylist, getSchedulerStatus } from './api.js';

// ---------------------------------------------------------------------------
// Radio status (polled periodically)
// ---------------------------------------------------------------------------

function createStatusStore() {
  const { subscribe, set } = writable({
    station_name: 'Denpa Radio',
    current_track: 'none',
    current_track_path: '',
    current_track_info: null,
    total_tracks: 0,
    active_clients: 0,
    max_clients: 0,
    active_tag: '',
    active_playlist: '',
    active_playlist_id: null,
    scheduler_running: false,
    playlist_summary: {},
  });

  let interval = null;

  async function refresh() {
    try {
      const data = await getStatus();
      set(data);
    } catch (err) {
      console.warn('Failed to fetch status:', err);
    }
  }

  function startPolling(ms = 5000) {
    stopPolling();
    refresh();
    interval = setInterval(refresh, ms);
  }

  function stopPolling() {
    if (interval) {
      clearInterval(interval);
      interval = null;
    }
  }

  return { subscribe, refresh, startPolling, stopPolling };
}

export const status = createStatusStore();

// Convenience derived stores from status.
export const stationName = derived(status, ($s) => $s.station_name);
export const currentTrack = derived(status, ($s) => $s.current_track);
export const currentTrackInfo = derived(status, ($s) => $s.current_track_info);
export const activeClients = derived(status, ($s) => $s.active_clients);
export const activeTag = derived(status, ($s) => $s.active_tag);
export const activePlaylist = derived(status, ($s) => $s.active_playlist);

// ---------------------------------------------------------------------------
// Playlists list (refreshed on demand)
// ---------------------------------------------------------------------------

function createPlaylistsStore() {
  const { subscribe, set } = writable([]);

  let loading = writable(false);
  let error = writable(null);

  async function refresh() {
    loading.set(true);
    error.set(null);
    try {
      const data = await listPlaylists();
      set(data.playlists || []);
    } catch (err) {
      console.error('Failed to fetch playlists:', err);
      error.set(err.message);
    } finally {
      loading.set(false);
    }
  }

  return { subscribe, refresh, loading, error };
}

export const playlists = createPlaylistsStore();

// ---------------------------------------------------------------------------
// Master playlist (refreshed on demand)
// ---------------------------------------------------------------------------

function createMasterStore() {
  const { subscribe, set } = writable({
    active_tag: '',
    active_playlist_id: null,
    total_tracks: 0,
    tags: {
      morning: { playlists: [], count: 0 },
      afternoon: { playlists: [], count: 0 },
      evening: { playlists: [], count: 0 },
      night: { playlists: [], count: 0 },
    },
  });

  let loading = writable(false);

  async function refresh() {
    loading.set(true);
    try {
      const data = await getMasterPlaylist();
      set(data);
    } catch (err) {
      console.error('Failed to fetch master playlist:', err);
    } finally {
      loading.set(false);
    }
  }

  return { subscribe, refresh, loading };
}

export const master = createMasterStore();

// ---------------------------------------------------------------------------
// Scheduler status (refreshed on demand)
// ---------------------------------------------------------------------------

function createSchedulerStore() {
  const { subscribe, set } = writable({
    running: false,
    last_tag: '',
    current_tag: '',
    time_tags: [],
    summary: {},
  });

  async function refresh() {
    try {
      const data = await getSchedulerStatus();
      set(data);
    } catch (err) {
      console.error('Failed to fetch scheduler status:', err);
    }
  }

  return { subscribe, refresh };
}

export const scheduler = createSchedulerStore();

// ---------------------------------------------------------------------------
// Toast / notification system
// ---------------------------------------------------------------------------

let toastId = 0;

function createToastStore() {
  const { subscribe, update } = writable([]);

  function add(message, type = 'info', duration = 4000) {
    const id = ++toastId;
    update((toasts) => [...toasts, { id, message, type }]);
    if (duration > 0) {
      setTimeout(() => remove(id), duration);
    }
    return id;
  }

  function remove(id) {
    update((toasts) => toasts.filter((t) => t.id !== id));
  }

  function success(message, duration) {
    return add(message, 'success', duration);
  }

  function error(message, duration) {
    return add(message, 'error', duration);
  }

  function warning(message, duration) {
    return add(message, 'warning', duration);
  }

  function info(message, duration) {
    return add(message, 'info', duration);
  }

  return { subscribe, add, remove, success, error, warning, info };
}

export const toasts = createToastStore();

// ---------------------------------------------------------------------------
// UI state
// ---------------------------------------------------------------------------

// Tracks which DJ sidebar section is expanded / selected.
export const djActiveSection = writable('dashboard');

// Whether the audio player is currently playing.
export const isPlaying = writable(false);
