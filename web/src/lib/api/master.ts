import { request, buildQuery } from "./client";
import type { Playlist } from "./playlists";

// ---------------------------------------------------------------------------
// Time Slots
// ---------------------------------------------------------------------------

export interface TimeSlot {
    tag: string;
    label: string;
    startHour: number;
    endHour: number;
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
    time_slots: TimeSlot[];
}

export async function getMasterPlaylist(channel?: string): Promise<MasterPlaylist> {
    return request<MasterPlaylist>("GET", `/api/master${buildQuery({ channel })}`, null, { noAuth: true });
}

export async function assignPlaylistToTag(tag: string, playlistId: number, channel?: string): Promise<void> {
    await request("PUT", `/api/master/${tag}${buildQuery({ channel })}`, { playlistId });
}

export async function removePlaylistFromTag(tag: string, playlistId: number, channel?: string): Promise<void> {
    await request("DELETE", `/api/master/${tag}/${playlistId}${buildQuery({ channel })}`);
}

// ---------------------------------------------------------------------------
// Time Slots (configurable schedule)
// ---------------------------------------------------------------------------

export async function getTimeSlots(channel?: string): Promise<{ timeSlots: TimeSlot[] }> {
    return request<{ timeSlots: TimeSlot[] }>("GET", `/api/timeslots${buildQuery({ channel })}`, null, { noAuth: true });
}

export async function setTimeSlots(
    timeSlots: TimeSlot[],
    channel?: string,
): Promise<{ message: string; timeSlots: TimeSlot[] }> {
    return request<{ message: string; timeSlots: TimeSlot[] }>(
        "PUT",
        `/api/timeslots${buildQuery({ channel })}`,
        { timeSlots },
    );
}
