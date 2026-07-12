import { request } from "./client";
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

export async function getMasterPlaylist(): Promise<MasterPlaylist> {
    return request<MasterPlaylist>("GET", "/api/master", null, { noAuth: true });
}

export async function assignPlaylistToTag(tag: string, playlistId: number): Promise<void> {
    await request("PUT", `/api/master/${tag}`, { playlistId });
}

export async function removePlaylistFromTag(tag: string, playlistId: number): Promise<void> {
    await request("DELETE", `/api/master/${tag}/${playlistId}`);
}

// ---------------------------------------------------------------------------
// Time Slots (configurable schedule)
// ---------------------------------------------------------------------------

export async function getTimeSlots(): Promise<{ timeSlots: TimeSlot[] }> {
    return request<{ timeSlots: TimeSlot[] }>("GET", "/api/timeslots", null, { noAuth: true });
}

export async function setTimeSlots(timeSlots: TimeSlot[]): Promise<{ message: string; timeSlots: TimeSlot[] }> {
    return request<{ message: string; timeSlots: TimeSlot[] }>("PUT", "/api/timeslots", { timeSlots });
}
