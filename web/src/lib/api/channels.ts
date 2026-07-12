import { request } from "./client";
import type { Track } from "./tracks";
import type { TimeSlot } from "./master";

// ---------------------------------------------------------------------------
// Channel types
// ---------------------------------------------------------------------------

export interface ChannelInfo {
    id: string;
    slug: string;
    name: string;
    description?: string;
    sortOrder: number;
    enabled: boolean;
    bitrate?: string;
}

export interface ChannelDetail extends ChannelInfo {
    activeTag: string;
    activePlaylistId?: number;
    totalTracks: number;
    schedulerRunning: boolean;
}

export interface ChannelStatus {
    stationName: string;
    currentTrack: string;
    currentTrackInfo?: Track;
    activeClients: number;
    maxClients: number;
    activeTag: string;
    activePlaylist?: string;
    activePlaylistId?: number;
    schedulerRunning: boolean;
    timezone: string;
    serverTime: string;
}

export interface CreateChannelBody {
    name: string;
    slug: string;
    description?: string;
    bitrate?: string;
    sortOrder?: number;
}

export interface UpdateChannelBody {
    name?: string;
    description?: string;
    bitrate?: string;
    sortOrder?: number;
    enabled?: boolean;
}

// ---------------------------------------------------------------------------
// Channel CRUD
// ---------------------------------------------------------------------------

export interface ChannelListResponse {
    channels: ChannelInfo[];
}

export async function listChannels(): Promise<ChannelListResponse> {
    return request<ChannelListResponse>("GET", "/api/channels", null, { noAuth: true });
}

export async function getChannel(slug: string): Promise<ChannelDetail> {
    return request<ChannelDetail>("GET", `/api/channels/${slug}`, null, { noAuth: true });
}

export async function createChannel(body: CreateChannelBody): Promise<ChannelInfo> {
    return request<ChannelInfo>("POST", "/api/channels", body);
}

export async function updateChannel(slug: string, body: UpdateChannelBody): Promise<ChannelInfo> {
    return request<ChannelInfo>("PUT", `/api/channels/${slug}`, body);
}

export async function deleteChannel(slug: string): Promise<void> {
    await request("DELETE", `/api/channels/${slug}`);
}

// ---------------------------------------------------------------------------
// Channel runtime (status, queue, skip controls)
// ---------------------------------------------------------------------------

export async function getChannelStatus(slug: string): Promise<ChannelStatus> {
    return request<ChannelStatus>("GET", `/api/channels/${slug}/status`, null, { noAuth: true });
}

export async function getChannelQueue(slug: string): Promise<{ tracks: Track[] }> {
    return request<{ tracks: Track[] }>("GET", `/api/channels/${slug}/queue`, null, { noAuth: true });
}

export async function skipNextChannel(slug: string): Promise<void> {
    await request("POST", `/api/channels/${slug}/skip/next`);
}

export async function skipPrevChannel(slug: string): Promise<void> {
    await request("POST", `/api/channels/${slug}/skip/prev`);
}

// ---------------------------------------------------------------------------
// Channel master (time-slot → playlist) mapping
// ---------------------------------------------------------------------------

export async function assignPlaylistToChannelTag(
    slug: string,
    tag: string,
    playlistId: number,
): Promise<void> {
    await request("PUT", `/api/channels/${slug}/master/${tag}`, { playlistId });
}

export async function removePlaylistFromChannelTag(
    slug: string,
    tag: string,
    playlistId: number,
): Promise<void> {
    await request("DELETE", `/api/channels/${slug}/master/${tag}/${playlistId}`);
}

// ---------------------------------------------------------------------------
// Channel time slots
// ---------------------------------------------------------------------------

export async function setChannelTimeSlots(
    slug: string,
    slots: TimeSlot[],
): Promise<{ message: string; timeSlots: TimeSlot[] }> {
    return request<{ message: string; timeSlots: TimeSlot[] }>(
        "PUT",
        `/api/channels/${slug}/timeslots`,
        { timeSlots: slots },
    );
}
