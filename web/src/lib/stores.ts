import { writable, derived } from "svelte/store";
import type { Readable, Writable } from "svelte/store";
import {
    getStatus,
    listTracks,
    listPlaylists,
    getMasterPlaylist,
    getSchedulerStatus,
} from "./api";
import type { RadioStatus, Track, Playlist, MasterPlaylist, SchedulerStatus } from "./api";

// ---------------------------------------------------------------------------
// Radio status (polled periodically)
// ---------------------------------------------------------------------------

interface StatusStore extends Readable<RadioStatus> {
    refresh(): Promise<void>;
    startPolling(ms?: number): void;
    stopPolling(): void;
}

function createStatusStore(): StatusStore {
    const { subscribe, set } = writable<RadioStatus>({
        station_name: "Denpa Radio",
        current_track: "none",
        current_track_info: null,
        total_tracks: 0,
        active_clients: 0,
        max_clients: 0,
        active_tag: "",
        active_playlist: "",
        active_playlist_id: null,
        scheduler_running: false,
        playlist_summary: {},
        library_tracks: 0,
        timezone: "UTC",
        server_time: "",
    });

    let interval: ReturnType<typeof setInterval> | null = null;

    async function refresh(): Promise<void> {
        try {
            const data = await getStatus();
            set(data);
        } catch (err) {
            console.warn("Failed to fetch status:", err);
        }
    }

    function startPolling(ms = 5000): void {
        stopPolling();
        refresh();
        interval = setInterval(refresh, ms);
    }

    function stopPolling(): void {
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
export const libraryTrackCount = derived(status, ($s) => $s.library_tracks);
export const timezone = derived(status, ($s) => $s.timezone || "UTC");
export const serverTime = derived(status, ($s) => $s.server_time || "");

// ---------------------------------------------------------------------------
// Track library (refreshed on demand)
// ---------------------------------------------------------------------------

interface TrackLibraryStore extends Readable<Track[]> {
    refresh(): Promise<void>;
    loading: Writable<boolean>;
    error: Writable<string | null>;
}

function createTrackLibraryStore(): TrackLibraryStore {
    const { subscribe, set } = writable<Track[]>([]);

    const loading = writable<boolean>(false);
    const error = writable<string | null>(null);

    async function refresh(): Promise<void> {
        loading.set(true);
        error.set(null);
        try {
            const data = await listTracks();
            set(data.tracks || []);
        } catch (err) {
            console.error("Failed to fetch track library:", err);
            error.set(err instanceof Error ? err.message : String(err));
        } finally {
            loading.set(false);
        }
    }

    return { subscribe, refresh, loading, error };
}

export const trackLibrary = createTrackLibraryStore();

// ---------------------------------------------------------------------------
// Playlists list (refreshed on demand)
// ---------------------------------------------------------------------------

interface PlaylistsStore extends Readable<Playlist[]> {
    refresh(): Promise<void>;
    loading: Writable<boolean>;
    error: Writable<string | null>;
}

function createPlaylistsStore(): PlaylistsStore {
    const { subscribe, set } = writable<Playlist[]>([]);

    const loading = writable<boolean>(false);
    const error = writable<string | null>(null);

    async function refresh(): Promise<void> {
        loading.set(true);
        error.set(null);
        try {
            const data = await listPlaylists();
            set(data.playlists || []);
        } catch (err) {
            console.error("Failed to fetch playlists:", err);
            error.set(err instanceof Error ? err.message : String(err));
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

interface MasterStore extends Readable<MasterPlaylist> {
    refresh(): Promise<void>;
    loading: Writable<boolean>;
}

function createMasterStore(): MasterStore {
    const { subscribe, set } = writable<MasterPlaylist>({
        active_tag: "",
        active_playlist_id: null,
        total_tracks: 0,
        tags: {
            morning: { playlists: [], count: 0 },
            afternoon: { playlists: [], count: 0 },
            evening: { playlists: [], count: 0 },
            night: { playlists: [], count: 0 },
        },
    });

    const loading = writable<boolean>(false);

    async function refresh(): Promise<void> {
        loading.set(true);
        try {
            const data = await getMasterPlaylist();
            set(data);
        } catch (err) {
            console.error("Failed to fetch master playlist:", err);
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

interface SchedulerStore extends Readable<SchedulerStatus> {
    refresh(): Promise<void>;
}

function createSchedulerStore(): SchedulerStore {
    const { subscribe, set } = writable<SchedulerStatus>({
        running: false,
        last_tag: "",
        current_tag: "",
        time_tags: [],
        summary: {},
    });

    async function refresh(): Promise<void> {
        try {
            const data = await getSchedulerStatus();
            set(data);
        } catch (err) {
            console.error("Failed to fetch scheduler status:", err);
        }
    }

    return { subscribe, refresh };
}

export const scheduler = createSchedulerStore();

// ---------------------------------------------------------------------------
// Toast / notification system
// ---------------------------------------------------------------------------

export type ToastType = "info" | "success" | "error" | "warning";

export interface Toast {
    id: number;
    message: string;
    type: ToastType;
}

interface ToastStore extends Readable<Toast[]> {
    add(message: string, type?: ToastType, duration?: number): number;
    remove(id: number): void;
    success(message: string, duration?: number): number;
    error(message: string, duration?: number): number;
    warning(message: string, duration?: number): number;
    info(message: string, duration?: number): number;
}

let toastId = 0;

function createToastStore(): ToastStore {
    const { subscribe, update } = writable<Toast[]>([]);

    function add(message: string, type: ToastType = "info", duration = 4000): number {
        const id = ++toastId;
        update((toasts) => [...toasts, { id, message, type }]);
        if (duration > 0) {
            setTimeout(() => remove(id), duration);
        }
        return id;
    }

    function remove(id: number): void {
        update((toasts) => toasts.filter((t) => t.id !== id));
    }

    function success(message: string, duration?: number): number {
        return add(message, "success", duration);
    }

    function error(message: string, duration?: number): number {
        return add(message, "error", duration);
    }

    function warning(message: string, duration?: number): number {
        return add(message, "warning", duration);
    }

    function info(message: string, duration?: number): number {
        return add(message, "info", duration);
    }

    return { subscribe, add, remove, success, error, warning, info };
}

export const toasts = createToastStore();

// ---------------------------------------------------------------------------
// UI state
// ---------------------------------------------------------------------------

// Tracks which DJ sidebar section is expanded / selected.
export const djActiveSection = writable<string>("dashboard");

// Whether the audio player is currently playing.
export const isPlaying = writable<boolean>(false);
