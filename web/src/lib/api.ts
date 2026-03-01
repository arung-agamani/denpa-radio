const BASE = "";

function getToken(): string | null {
    return localStorage.getItem("dj_token");
}

function authHeaders(): Record<string, string> {
    const token = getToken();
    const headers: Record<string, string> = { "Content-Type": "application/json" };
    if (token) {
        headers["Authorization"] = `Bearer ${token}`;
    }
    return headers;
}

interface RequestOptions {
    noAuth?: boolean;
    raw?: boolean;
    rawBody?: string;
}

async function request<T = unknown>(
    method: string,
    path: string,
    body: unknown = null,
    options: RequestOptions = {},
): Promise<T> {
    const config: RequestInit & { headers: Record<string, string> } = {
        method,
        headers: options.noAuth
            ? { "Content-Type": "application/json" }
            : authHeaders(),
    };

    if (body !== null) {
        if (
            body instanceof Blob ||
            body instanceof ArrayBuffer ||
            typeof body === "string"
        ) {
            config.body = body as BodyInit;
        } else {
            config.body = JSON.stringify(body);
        }
    }

    if (options.rawBody) {
        config.body = options.rawBody;
        if (!options.noAuth) {
            config.headers = { ...authHeaders() };
        }
        delete config.headers["Content-Type"];
    }

    const res = await fetch(`${BASE}${path}`, config);

    if (res.status === 401) {
        const event = new CustomEvent("auth:unauthorized");
        window.dispatchEvent(event);
    }

    if (options.raw) {
        return res as unknown as T;
    }

    const data = await res.json();

    if (!res.ok) {
        throw new ApiError(data.error || res.statusText, res.status, data);
    }

    return data as T;
}

export class ApiError extends Error {
    status: number;
    data: unknown;

    constructor(message: string, status: number, data: unknown) {
        super(message);
        this.name = "ApiError";
        this.status = status;
        this.data = data;
    }
}

// ---------------------------------------------------------------------------
// Auth
// ---------------------------------------------------------------------------

export interface LoginResponse {
    token: string;
    username: string;
}

export async function login(username: string, password: string): Promise<LoginResponse> {
    const data = await request<LoginResponse>(
        "POST",
        "/api/auth/login",
        { username, password },
        { noAuth: true },
    );
    if (data.token) {
        localStorage.setItem("dj_token", data.token);
        localStorage.setItem("dj_username", data.username);
    }
    return data;
}

export async function verifyToken(): Promise<unknown> {
    return request("GET", "/api/auth/verify");
}

export function logout(): void {
    localStorage.removeItem("dj_token");
    localStorage.removeItem("dj_username");
}

export function isLoggedIn(): boolean {
    return !!getToken();
}

export function getUsername(): string {
    return localStorage.getItem("dj_username") || "";
}

// ---------------------------------------------------------------------------
// Public status
// ---------------------------------------------------------------------------

export interface TrackInfo {
    id: number;
    title: string;
    artist: string;
    album: string;
    genre: string;
    year?: number;
    format?: string;
    duration: number;
    checksum: string;
    file_path: string;
}

export interface RadioStatus {
    station_name: string;
    current_track: string;
    current_track_info: TrackInfo | null;
    total_tracks: number;
    active_clients: number;
    max_clients: number;
    active_tag: string;
    active_playlist: string;
    active_playlist_id: number | null;
    scheduler_running: boolean;
    playlist_summary: Record<string, number>;
    library_tracks: number;
    timezone: string;
    server_time: string;
}

export async function getStatus(): Promise<RadioStatus> {
    return request<RadioStatus>("GET", "/api/status", null, { noAuth: true });
}

export async function getHealth(): Promise<unknown> {
    return request("GET", "/health", null, { noAuth: true });
}

// ---------------------------------------------------------------------------
// Tracks
// ---------------------------------------------------------------------------

export interface Track {
    id: number;
    title: string;
    artist: string;
    album: string;
    genre: string;
    year?: number;
    track_num?: number;
    format?: string;
    duration: number;
    checksum: string;
    file_path: string;
    size?: number;
}

export interface TrackListResponse {
    tracks: Track[];
}

export async function listTracks(): Promise<TrackListResponse> {
    return request<TrackListResponse>("GET", "/api/tracks", null, { noAuth: true });
}

export async function getTrack(id: number): Promise<Track> {
    return request<Track>(`GET`, `/api/tracks/${id}`, null, { noAuth: true });
}

export async function listOrphanedTracks(): Promise<TrackListResponse> {
    return request<TrackListResponse>("GET", "/api/tracks/orphaned");
}

export async function updateTrack(id: number, updates: Partial<Track>): Promise<Track> {
    return request<Track>("PUT", `/api/tracks/${id}`, updates);
}

export interface DeleteTrackOptions {
    deleteFromDisk?: boolean;
}

export async function deleteTrack(id: number, { deleteFromDisk = false }: DeleteTrackOptions = {}): Promise<unknown> {
    const qs = deleteFromDisk ? "?deleteFromDisk=true" : "";
    return request("DELETE", `/api/tracks/${id}${qs}`);
}

export interface ScanResult {
    newly_added: number;
    library_total: number;
}

export async function scanTracks(): Promise<ScanResult> {
    return request<ScanResult>("POST", "/api/tracks/scan");
}

export async function searchTracks(query: string): Promise<TrackListResponse> {
    return request<TrackListResponse>(
        "GET",
        `/api/tracks/search?q=${encodeURIComponent(query)}`,
        null,
        { noAuth: true },
    );
}

// ---------------------------------------------------------------------------
// Playlists
// ---------------------------------------------------------------------------

export interface Playlist {
    id: number;
    name: string;
    tag: string;
    tracks: Track[];
    trackCount?: number;
    currentTrackChecksum?: string;
}

/** Subset alias for components that only need basic track fields. Compatible with Track. */
export type TrackItem = Track;

export interface PlaylistListResponse {
    playlists: Playlist[];
}

export async function listPlaylists(): Promise<PlaylistListResponse> {
    return request<PlaylistListResponse>("GET", "/api/playlists", null, { noAuth: true });
}

export async function getPlaylist(id: number): Promise<Playlist> {
    return request<Playlist>("GET", `/api/playlists/${id}`, null, { noAuth: true });
}

export async function createPlaylist(name: string, tag: string): Promise<Playlist> {
    return request<Playlist>("POST", "/api/playlists", { name, tag });
}

export async function updatePlaylist(id: number, updates: Partial<Playlist>): Promise<Playlist> {
    return request<Playlist>("PUT", `/api/playlists/${id}`, updates);
}

export async function deletePlaylist(id: number): Promise<unknown> {
    return request("DELETE", `/api/playlists/${id}`);
}

// ---------------------------------------------------------------------------
// Playlist track manipulation
// ---------------------------------------------------------------------------

export interface AddTrackOptions {
    trackId?: number;
    checksum?: string;
    filePath?: string;
    index?: number;
}

export async function addTrackToPlaylist(
    playlistId: number,
    { trackId, checksum, filePath, index }: AddTrackOptions = {},
): Promise<unknown> {
    const body: Record<string, unknown> = {};
    if (trackId !== undefined) body.trackId = trackId;
    if (checksum !== undefined) body.checksum = checksum;
    if (filePath !== undefined) body.filePath = filePath;
    if (index !== undefined) body.index = index;
    return request("POST", `/api/playlists/${playlistId}/tracks`, body);
}

export async function removeTrackFromPlaylist(playlistId: number, trackId: number): Promise<unknown> {
    return request("DELETE", `/api/playlists/${playlistId}/tracks/${trackId}`);
}

export async function moveTrackInPlaylist(playlistId: number, from: number, to: number): Promise<unknown> {
    return request("POST", `/api/playlists/${playlistId}/tracks/move`, {
        from,
        to,
    });
}

export async function shufflePlaylist(playlistId: number): Promise<unknown> {
    return request("POST", `/api/playlists/${playlistId}/shuffle`);
}

// ---------------------------------------------------------------------------
// Playlist import / export
// ---------------------------------------------------------------------------

export interface ExportPlaylistResult {
    blob: Blob;
    filename: string;
}

export async function exportPlaylist(id: number): Promise<ExportPlaylistResult> {
    const res = await request<Response>("GET", `/api/playlists/${id}/export`, null, {
        raw: true,
    });
    if (!res.ok) {
        const data = await res.json();
        throw new ApiError(data.error || res.statusText, res.status, data);
    }
    const blob = await res.blob();
    const disposition = res.headers.get("Content-Disposition") || "";
    const match = disposition.match(/filename="?([^"]+)"?/);
    const filename = match ? match[1] : `playlist_${id}.json`;
    return { blob, filename };
}

export async function importPlaylist(jsonString: string): Promise<unknown> {
    return request("POST", "/api/playlists/import", null, {
        rawBody: jsonString,
    });
}

// ---------------------------------------------------------------------------
// Master playlist
// ---------------------------------------------------------------------------

export interface TagEntry {
    playlists: Playlist[];
    count: number;
}

export interface MasterPlaylist {
    active_tag: string;
    active_playlist_id: number | null;
    total_tracks: number;
    tags: Record<string, TagEntry>;
}

export async function getMasterPlaylist(): Promise<MasterPlaylist> {
    return request<MasterPlaylist>("GET", "/api/master", null, { noAuth: true });
}

export async function assignPlaylistToTag(tag: string, playlistId: number): Promise<unknown> {
    return request("PUT", `/api/master/${tag}`, { playlistId });
}

export async function removePlaylistFromTag(tag: string, playlistId: number): Promise<unknown> {
    return request("DELETE", `/api/master/${tag}/${playlistId}`);
}

// ---------------------------------------------------------------------------
// Queue
// ---------------------------------------------------------------------------

export async function getQueue(): Promise<TrackListResponse> {
    return request<TrackListResponse>("GET", "/api/queue", null, { noAuth: true });
}

// ---------------------------------------------------------------------------
// Skip controls (protected)
// ---------------------------------------------------------------------------

export async function skipNext(): Promise<unknown> {
    return request("POST", "/api/skip/next");
}

export async function skipPrev(): Promise<unknown> {
    return request("POST", "/api/skip/prev");
}

// ---------------------------------------------------------------------------
// Scheduler
// ---------------------------------------------------------------------------

export interface SchedulerStatus {
    running: boolean;
    last_tag: string;
    current_tag: string;
    time_tags: string[];
    summary: Record<string, unknown>;
}

export async function getSchedulerStatus(): Promise<SchedulerStatus> {
    return request<SchedulerStatus>("GET", "/api/scheduler/status", null, { noAuth: true });
}

// ---------------------------------------------------------------------------
// Timezone
// ---------------------------------------------------------------------------

export async function getTimezone(): Promise<{ timezone: string }> {
    return request<{ timezone: string }>("GET", "/api/timezone", null, { noAuth: true });
}

export async function setTimezone(timezone: string): Promise<{ timezone: string }> {
    return request<{ timezone: string }>("PUT", "/api/timezone", { timezone });
}

// ---------------------------------------------------------------------------
// Reconcile / hot-reload
// ---------------------------------------------------------------------------

export interface ReconcileResult {
    removed_count: number;
    orphaned_count: number;
    total_tracks?: number;
}

export async function reconcile(): Promise<ReconcileResult> {
    return request<ReconcileResult>("POST", "/api/reconcile");
}

// ---------------------------------------------------------------------------
// Track upload
// ---------------------------------------------------------------------------

export interface UploadMeta {
    title?: string;
    artist?: string;
    album?: string;
    genre?: string;
}

export interface UploadOptions {
    onProgress?: (percent: number) => void;
    meta?: UploadMeta;
    optimize?: boolean;
}

export interface UploadResult {
    status: string;
    added: boolean;
    track: Track;
}

export function uploadTrack(
    file: File,
    { onProgress, meta, optimize = true }: UploadOptions = {},
): Promise<UploadResult> {
    return new Promise((resolve, reject) => {
        const token = getToken();
        const form = new FormData();
        form.append("file", file);

        // Append the optimize flag (defaults to true).
        form.append("optimize", optimize ? "true" : "false");

        // Append any provided metadata fields so the server can override
        // the embedded audio tags and derive the on-disk filename from the title.
        if (meta?.title?.trim())  form.append("title",  meta.title.trim());
        if (meta?.artist?.trim()) form.append("artist", meta.artist.trim());
        if (meta?.album?.trim())  form.append("album",  meta.album.trim());
        if (meta?.genre?.trim())  form.append("genre",  meta.genre.trim());

        const xhr = new XMLHttpRequest();

        xhr.upload.addEventListener("progress", (e) => {
            if (e.lengthComputable && onProgress) {
                onProgress(Math.round((e.loaded / e.total) * 100));
            }
        });

        xhr.addEventListener("load", () => {
            let data: { error?: string | { message?: string } } & Record<string, unknown>;
            try {
                data = JSON.parse(xhr.responseText);
            } catch {
                reject(new ApiError("Invalid server response", xhr.status, null));
                return;
            }
            if (xhr.status === 401) {
                window.dispatchEvent(new CustomEvent("auth:unauthorized"));
            }
            if (xhr.status >= 200 && xhr.status < 300) {
                resolve(data as unknown as UploadResult);
            } else {
                const errField = data?.error;
                const msg =
                    typeof errField === "object"
                        ? errField?.message ?? xhr.statusText
                        : errField ?? xhr.statusText;
                reject(new ApiError(msg, xhr.status, data));
            }
        });

        xhr.addEventListener("error", () => {
            reject(new ApiError("Network error during upload", 0, null));
        });

        xhr.addEventListener("abort", () => {
            reject(new ApiError("Upload was cancelled", 0, null));
        });

        xhr.open("POST", "/api/tracks/upload");
        if (token) {
            xhr.setRequestHeader("Authorization", `Bearer ${token}`);
        }
        xhr.send(form);
    });
}
