import { request, buildQuery } from "./client";

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
    trackNum?: number;
    format?: string;
    duration: number;
    checksum: string;
    filePath: string;
    coverUrl?: string;
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

export async function getStatus(channel?: string): Promise<RadioStatus> {
    return request<RadioStatus>("GET", `/api/status${buildQuery({ channel })}`, null, { noAuth: true });
}

export async function getHealth(): Promise<unknown> {
    return request("GET", "/health", null, { noAuth: true });
}

// ---------------------------------------------------------------------------
// Queue
// ---------------------------------------------------------------------------

export async function getQueue(channel?: string): Promise<{ tracks: TrackInfo[] }> {
    return request<{ tracks: TrackInfo[] }>("GET", `/api/queue${buildQuery({ channel })}`, null, { noAuth: true });
}

// ---------------------------------------------------------------------------
// Skip controls (protected)
// ---------------------------------------------------------------------------

export async function skipNext(channel?: string): Promise<void> {
    await request("POST", `/api/skip/next${buildQuery({ channel })}`);
}

export async function skipPrev(channel?: string): Promise<void> {
    await request("POST", `/api/skip/prev${buildQuery({ channel })}`);
}

// ---------------------------------------------------------------------------
// Scheduler
// ---------------------------------------------------------------------------

export interface SchedulerStatus {
    running: boolean;
    last_tag: string;
    current_tag: string;
    time_tags: string[];
    library_tracks?: number;
    summary: Record<string, unknown>;
}

export async function getSchedulerStatus(channel?: string): Promise<SchedulerStatus> {
    return request<SchedulerStatus>("GET", `/api/scheduler/status${buildQuery({ channel })}`, null, { noAuth: true });
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
