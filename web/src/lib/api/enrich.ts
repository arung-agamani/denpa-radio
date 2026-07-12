import { request } from "./client";
import type { Track } from "./tracks";

// ---------------------------------------------------------------------------
// Metadata enrichment
// ---------------------------------------------------------------------------

export interface EnrichResult {
    mbid?: string;
    releaseDate?: string;
    label?: string;
    artistName?: string;
    albumTitle?: string;
    type?: string;
    genres?: string[];
    styles?: string[];
    year?: number;
    artFetched?: boolean;
    artPath?: string;
}

/** Returns the URL for a track's cover art image, or empty string. */
export function getCoverUrl(track: Track): string {
    return track.coverUrl || '';
}

/** Enrich a single track's metadata from external sources. */
export async function enrichTrack(id: number): Promise<EnrichResult> {
    return request<EnrichResult>("POST", `/api/tracks/${id}/enrich`);
}

/** Batch enrich all tracks in the library. */
export async function enrichLibrary(): Promise<{ tracks_attempted: number; art_found: number }> {
    return request<{ tracks_attempted: number; art_found: number }>("POST", "/api/library/enrich");
}
