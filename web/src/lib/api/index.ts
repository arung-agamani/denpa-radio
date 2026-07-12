// ---------------------------------------------------------------------------
// Barrel file — re-exports the full API surface.
// Import from '$lib/api' (or './api') to access everything.
// ---------------------------------------------------------------------------

export { ApiError } from "./client";
export type { RequestOptions } from "./client";

export { login, verifyToken, logout, isLoggedIn, getUsername } from "./auth";
export type { LoginResponse } from "./auth";

export { getStatus, getHealth, getQueue, skipNext, skipPrev, getSchedulerStatus, getTimezone, setTimezone, reconcile } from "./radio";
export type { TrackInfo, RadioStatus, SchedulerStatus, ReconcileResult } from "./radio";

export { listTracks, getTrack, listOrphanedTracks, updateTrack, deleteTrack, scanTracks, searchTracks, batchUpdateTracks, batchUpdateCover, refreshTrackMetadata } from "./tracks";
export type { Track, TrackListResponse, DeleteTrackOptions, ScanResult, TrackFilter, BatchUpdatePayload, BatchUpdateResult, SearchResult, RefreshMetadataResult } from "./tracks";

export { listPlaylists, getPlaylist, createPlaylist, updatePlaylist, deletePlaylist, addTrackToPlaylist, removeTrackFromPlaylist, moveTrackInPlaylist, shufflePlaylist, exportPlaylist, importPlaylist } from "./playlists";
export type { Playlist, TrackItem, PlaylistListResponse, AddTrackOptions, ExportPlaylistResult } from "./playlists";

export { getMasterPlaylist, assignPlaylistToTag, removePlaylistFromTag, getTimeSlots, setTimeSlots } from "./master";
export type { TimeSlot, TagEntry, MasterPlaylist } from "./master";

export { uploadTrack } from "./upload";
export type { UploadMeta, UploadOptions, UploadResult } from "./upload";

export { getCoverUrl, enrichTrack, enrichLibrary } from "./enrich";
export type { EnrichResult } from "./enrich";
