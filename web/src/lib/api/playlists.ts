import { request, ApiError } from "./client";
import type { Track } from "./tracks";

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

export async function getPlaylist(id: number): Promise<{ tag: string; playlist: Playlist }> {
    return request<{ tag: string; playlist: Playlist }>("GET", `/api/playlists/${id}`, null, { noAuth: true });
}

export async function createPlaylist(name: string, tag: string): Promise<Playlist> {
    return request<Playlist>("POST", "/api/playlists", { name, tag });
}

export async function updatePlaylist(id: number, updates: Partial<Playlist>): Promise<Playlist> {
    return request<Playlist>("PUT", `/api/playlists/${id}`, updates);
}

export async function deletePlaylist(id: number): Promise<void> {
    await request("DELETE", `/api/playlists/${id}`);
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
): Promise<{ track: Track; playlist: Playlist }> {
    const body: Record<string, unknown> = {};
    if (trackId !== undefined) body.trackId = trackId;
    if (checksum !== undefined) body.checksum = checksum;
    if (filePath !== undefined) body.filePath = filePath;
    if (index !== undefined) body.index = index;
    return request<{ track: Track; playlist: Playlist }>("POST", `/api/playlists/${playlistId}/tracks`, body);
}

export async function removeTrackFromPlaylist(playlistId: number, trackId: number): Promise<{ removed_track: Track; playlist: Playlist }> {
    return request<{ removed_track: Track; playlist: Playlist }>("DELETE", `/api/playlists/${playlistId}/tracks/${trackId}`);
}

export async function moveTrackInPlaylist(playlistId: number, from: number, to: number): Promise<Playlist> {
    return request<Playlist>("POST", `/api/playlists/${playlistId}/tracks/move`, {
        from,
        to,
    });
}

export async function shufflePlaylist(playlistId: number): Promise<Playlist> {
    return request<Playlist>("POST", `/api/playlists/${playlistId}/shuffle`);
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
        const err = data.error as { message?: string; code?: string } | string | undefined;
        let message = res.statusText;
        let code = "unknown";
        if (typeof err === "string") {
            message = err;
            code = "error";
        } else if (err && typeof err === "object") {
            message = err.message || res.statusText;
            code = err.code || "error";
        }
        throw new ApiError(message, res.status, code, data);
    }
    const blob = await res.blob();
    const disposition = res.headers.get("Content-Disposition") || "";
    const match = disposition.match(/filename="?([^"]+)"?/);
    const filename = match ? match[1] : `playlist_${id}.json`;
    return { blob, filename };
}

export async function importPlaylist(jsonString: string): Promise<{ message: string; playlist: Playlist }> {
    return request<{ message: string; playlist: Playlist }>("POST", "/api/playlists/import", null, {
        rawBody: jsonString,
    });
}
