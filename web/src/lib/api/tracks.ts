import { request } from "./client";

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
    trackNum?: number;
    format?: string;
    duration: number;
    checksum: string;
    filePath: string;
    coverUrl?: string;
}

export interface TrackListResponse {
    tracks: Track[];
    total_tracks?: number;
    library_total?: number;
}

export async function listTracks(): Promise<TrackListResponse> {
    return request<TrackListResponse>("GET", "/api/tracks", null, { noAuth: true });
}

export async function getTrack(id: number): Promise<Track> {
    return request<Track>("GET", `/api/tracks/${id}`, null, { noAuth: true });
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

export async function deleteTrack(id: number, { deleteFromDisk = false }: DeleteTrackOptions = {}): Promise<void> {
    const qs = deleteFromDisk ? "?deleteFromDisk=true" : "";
    await request("DELETE", `/api/tracks/${id}${qs}`);
}

export interface ScanResult {
    newly_added: number;
    library_total: number;
}

export async function scanTracks(): Promise<ScanResult> {
    return request<ScanResult>("POST", "/api/tracks/scan");
}

export interface SearchResult {
    tracks: Track[];
    query?: string;
    total_tracks?: number;
}

export async function searchTracks(query: string): Promise<SearchResult> {
    return request<SearchResult>(
        "GET",
        `/api/tracks/search?q=${encodeURIComponent(query)}`,
        null,
        { noAuth: true },
    );
}

// ---------------------------------------------------------------------------
// Batch operations
// ---------------------------------------------------------------------------

export interface TrackFilter {
    album?: string;
    artist?: string;
    genre?: string;
}

export interface BatchUpdatePayload {
    filter: TrackFilter;
    updates: Partial<Track>;
}

export interface BatchUpdateResult {
    status: string;
    updated: number;
}

export async function batchUpdateTracks(payload: BatchUpdatePayload): Promise<BatchUpdateResult> {
    return request<BatchUpdateResult>("POST", "/api/library/batch-update", payload);
}

export async function batchUpdateCover(filter: TrackFilter, file: File): Promise<BatchUpdateResult> {
    const formData = new FormData();
    formData.append("cover", file);
    if (filter.album) formData.append("album", filter.album);
    if (filter.artist) formData.append("artist", filter.artist);
    if (filter.genre) formData.append("genre", filter.genre);
    return request<BatchUpdateResult>("POST", "/api/library/batch-cover", null, { formData });
}

// ---------------------------------------------------------------------------
// Metadata refresh / backfill
// ---------------------------------------------------------------------------

export interface RefreshMetadataResult {
    total: number;
    probed: number;
    updated: number;
    failed: number;
}

export async function refreshTrackMetadata(): Promise<RefreshMetadataResult> {
    return request<RefreshMetadataResult>("POST", "/api/tracks/refresh-metadata");
}
