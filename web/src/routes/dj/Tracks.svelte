<script>
    import {
        listTracks,
        listOrphanedTracks,
        updateTrack,
        deleteTrack,
        scanTracks,
        reconcile,
    } from "../../lib/api.js";
    import { playlists, master, trackLibrary, toasts } from "../../lib/stores.js";
    import TrackList from "../../components/TrackList.svelte";
    import TrackUpload from "../../components/TrackUpload.svelte";

    // ---------------------------------------------------------------------------
    // Track library state
    // ---------------------------------------------------------------------------

    let allTracks = [];
    let orphanedTracks = [];
    let loadingTracks = false;
    let loadingOrphaned = false;
    let trackSearchQuery = "";
    let showUploadPanel = false;

    // Reconcile
    let reconciling = false;
    let reconcileResult = null;

    // Track editing
    let editingTrackId = null;
    let editTrackTitle = "";
    let editTrackArtist = "";
    let editTrackAlbum = "";
    let editTrackGenre = "";
    let editTrackYear = null;
    let editTrackNum = null;
    let savingTrack = false;
    let scanning = false;

    // Confirm delete
    let confirmDeleteTrackId = null;
    let confirmDeleteFromDisk = false;

    // ---------------------------------------------------------------------------
    // Data loading
    // ---------------------------------------------------------------------------

    async function loadAllTracks() {
        loadingTracks = true;
        try {
            const data = await listTracks();
            allTracks = data.tracks || [];
            trackLibrary.refresh();
        } catch (err) {
            toasts.error("Failed to load tracks: " + err.message);
        } finally {
            loadingTracks = false;
        }
    }

    async function loadOrphanedTracksData() {
        loadingOrphaned = true;
        try {
            const data = await listOrphanedTracks();
            orphanedTracks = data.tracks || [];
        } catch (err) {
            toasts.error("Failed to load orphaned tracks: " + err.message);
        } finally {
            loadingOrphaned = false;
        }
    }

    async function handleScanTracks() {
        scanning = true;
        try {
            const data = await scanTracks();
            toasts.success(
                `Scan complete: ${data.newly_added} new track(s) added. Library total: ${data.library_total}.`,
            );
            await loadAllTracks();
        } catch (err) {
            toasts.error("Scan failed: " + err.message);
        } finally {
            scanning = false;
        }
    }

    async function handleReconcile() {
        reconciling = true;
        reconcileResult = null;
        try {
            const data = await reconcile();
            reconcileResult = data;
            toasts.success(
                `Reconciled: ${data.removed_count} removed, ${data.orphaned_count} new files found.`,
            );
            await playlists.refresh();
            await master.refresh();
            await loadAllTracks();
            await loadOrphanedTracksData();
        } catch (err) {
            toasts.error("Reconcile failed: " + err.message);
        } finally {
            reconciling = false;
        }
    }

    // ---------------------------------------------------------------------------
    // Track editing
    // ---------------------------------------------------------------------------

    function startEditTrack(track) {
        editingTrackId = track.id;
        editTrackTitle = track.title || "";
        editTrackArtist = track.artist || "";
        editTrackAlbum = track.album || "";
        editTrackGenre = track.genre || "";
        editTrackYear = track.year || null;
        editTrackNum = track.trackNum || null;
    }

    function cancelEditTrack() {
        editingTrackId = null;
        savingTrack = false;
    }

    async function saveEditTrack() {
        if (!editingTrackId) return;
        savingTrack = true;
        try {
            const updates = {};
            updates.title = editTrackTitle;
            updates.artist = editTrackArtist;
            updates.album = editTrackAlbum;
            updates.genre = editTrackGenre;
            if (editTrackYear !== null && editTrackYear !== "")
                updates.year = parseInt(editTrackYear);
            if (editTrackNum !== null && editTrackNum !== "")
                updates.trackNum = parseInt(editTrackNum);
            await updateTrack(editingTrackId, updates);
            toasts.success("Track metadata updated!");
            editingTrackId = null;
            await loadAllTracks();
        } catch (err) {
            toasts.error("Failed to update track: " + err.message);
        } finally {
            savingTrack = false;
        }
    }

    async function handleDeleteTrack(id) {
        try {
            await deleteTrack(id, { deleteFromDisk: confirmDeleteFromDisk });
            const msg = confirmDeleteFromDisk
                ? "Track removed from library, all playlists, and deleted from disk."
                : "Track removed from library and all playlists.";
            toasts.success(msg);
            confirmDeleteTrackId = null;
            confirmDeleteFromDisk = false;
            await loadAllTracks();
            await playlists.refresh();
            await master.refresh();
        } catch (err) {
            toasts.error("Failed to delete track: " + err.message);
        }
    }

    // ---------------------------------------------------------------------------
    // Derived
    // ---------------------------------------------------------------------------

    $: filteredLibraryTracks = trackSearchQuery.trim()
        ? allTracks.filter(
              (t) =>
                  (t.title || "").toLowerCase().includes(trackSearchQuery.toLowerCase()) ||
                  (t.artist || "").toLowerCase().includes(trackSearchQuery.toLowerCase()) ||
                  (t.album || "").toLowerCase().includes(trackSearchQuery.toLowerCase()) ||
                  (t.genre || "").toLowerCase().includes(trackSearchQuery.toLowerCase()),
          )
        : allTracks;

    // Spinner SVG snippet helper as a string (reused in multiple buttons)
    const spinnerPath = `<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/>`;
</script>

<h1 class="text-2xl font-bold text-gray-900 dark:text-white">📚 Track Library</h1>
<p class="text-sm text-gray-500 dark:text-gray-400 -mt-4">
    Central library of all known tracks. Edit metadata here and changes propagate to all playlists.
</p>

<!-- Actions bar -->
<div class="flex flex-wrap gap-3">
    <button
        type="button"
        class="px-4 py-2 text-sm font-medium rounded-lg border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors flex items-center gap-2"
        on:click={loadAllTracks}
        disabled={loadingTracks}
    >
        {#if loadingTracks}
            <svg class="animate-spin w-4 h-4" fill="none" viewBox="0 0 24 24">
                <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/>
                <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/>
            </svg>
        {/if}
        📚 Load Library
    </button>
    <button
        type="button"
        class="px-4 py-2 text-sm font-medium rounded-lg border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors flex items-center gap-2"
        on:click={handleScanTracks}
        disabled={scanning}
    >
        {#if scanning}
            <svg class="animate-spin w-4 h-4" fill="none" viewBox="0 0 24 24">
                <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/>
                <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/>
            </svg>
        {/if}
        🔍 Scan Music Directory
    </button>
    <button
        type="button"
        class="px-4 py-2 text-sm font-medium rounded-lg border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors flex items-center gap-2"
        on:click={loadOrphanedTracksData}
        disabled={loadingOrphaned}
    >
        {#if loadingOrphaned}
            <svg class="animate-spin w-4 h-4" fill="none" viewBox="0 0 24 24">
                <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/>
                <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/>
            </svg>
        {/if}
        🔎 Find Orphaned
    </button>
    <button
        type="button"
        class="px-4 py-2 text-sm font-medium rounded-lg border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors flex items-center gap-2"
        on:click={handleReconcile}
        disabled={reconciling}
    >
        {#if reconciling}
            <svg class="animate-spin w-4 h-4" fill="none" viewBox="0 0 24 24">
                <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/>
                <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/>
            </svg>
        {/if}
        🔄 Reconcile
    </button>
    <button
        type="button"
        class="px-4 py-2 text-sm font-medium rounded-lg border transition-colors flex items-center gap-2
            {showUploadPanel
            ? 'border-primary-500 bg-primary-50 dark:bg-primary-900/20 text-primary-700 dark:text-primary-400'
            : 'border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700'}"
        on:click={() => (showUploadPanel = !showUploadPanel)}
    >
        <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" d="M3 16.5v2.25A2.25 2.25 0 0 0 5.25 21h13.5A2.25 2.25 0 0 0 21 18.75V16.5m-13.5-9L12 3m0 0 4.5 4.5M12 3v13.5" />
        </svg>
        Upload Files
    </button>
</div>

<!-- Upload panel -->
{#if showUploadPanel}
    <div class="bg-white dark:bg-gray-800 rounded-xl border border-primary-200 dark:border-primary-800 overflow-hidden">
        <div class="flex items-center justify-between px-5 py-4 border-b border-primary-100 dark:border-primary-800 bg-primary-50 dark:bg-primary-900/20">
            <h3 class="text-base font-semibold text-primary-800 dark:text-primary-300 flex items-center gap-2">
                <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M3 16.5v2.25A2.25 2.25 0 0 0 5.25 21h13.5A2.25 2.25 0 0 0 21 18.75V16.5m-13.5-9L12 3m0 0 4.5 4.5M12 3v13.5" />
                </svg>
                Upload Audio Files
            </h3>
            <button
                type="button"
                class="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 transition-colors"
                on:click={() => (showUploadPanel = false)}
                aria-label="Close upload panel"
            >
                <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M6 18 18 6M6 6l12 12" />
                </svg>
            </button>
        </div>
        <div class="p-5">
            <TrackUpload
                on:uploaded={(e) => {
                    toasts.success(`Uploaded: ${e.detail.track.title || e.detail.track.filePath}`);
                    loadAllTracks();
                }}
            />
        </div>
    </div>
{/if}

{#if reconcileResult}
    <div class="p-3 rounded-lg bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800 text-sm">
        <p class="text-green-800 dark:text-green-300">
            ✅ Reconcile: {reconcileResult.removed_count} removed,
            {reconcileResult.orphaned_count} new. Total: {reconcileResult.total_tracks}.
        </p>
    </div>
{/if}

<!-- Orphaned tracks section -->
{#if orphanedTracks.length > 0}
    <div class="bg-white dark:bg-gray-800 rounded-xl border border-amber-200 dark:border-amber-800 overflow-hidden">
        <div class="px-5 py-4 border-b border-amber-200 dark:border-amber-800 bg-amber-50 dark:bg-amber-900/20">
            <h3 class="text-base font-semibold text-amber-800 dark:text-amber-300 flex items-center gap-2">
                <span>⚠️</span>
                Orphaned Tracks
                <span class="text-xs font-normal text-amber-600 dark:text-amber-400">
                    ({orphanedTracks.length} file{orphanedTracks.length !== 1 ? "s" : ""} on disk not in library)
                </span>
            </h3>
        </div>
        <TrackList
            tracks={orphanedTracks}
            editable={false}
            showIndex={true}
            compact={true}
            emptyMessage="No orphaned tracks found."
        />
    </div>
{/if}

<!-- Track library section -->
<div class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 overflow-hidden">
    <div class="px-5 py-4 border-b border-gray-200 dark:border-gray-700 flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3">
        <h3 class="text-base font-semibold text-gray-900 dark:text-white">
            📚 Track Library ({allTracks.length})
        </h3>
        <input
            type="text"
            placeholder="Search tracks..."
            bind:value={trackSearchQuery}
            class="w-full sm:w-64 px-3 py-1.5 text-sm rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-primary-500 focus:border-primary-500"
        />
    </div>

    {#if allTracks.length === 0 && !loadingTracks}
        <div class="p-12 text-center text-gray-400 dark:text-gray-500">
            <span class="text-4xl block mb-3">📚</span>
            <p class="text-sm font-medium">Track library is empty.</p>
            <p class="text-xs mt-1">
                Click "Load Library" to fetch all tracks, or "Scan Music Directory" to discover files.
            </p>
        </div>
    {:else if loadingTracks}
        <div class="flex items-center justify-center py-12">
            <div class="animate-spin rounded-full h-8 w-8 border-b-2 border-primary-500"></div>
        </div>
    {:else}
        <div class="overflow-x-auto">
            <table class="w-full text-left">
                <thead>
                    <tr class="border-b border-gray-200 dark:border-gray-700 text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-gray-400">
                        <th class="px-3 py-2 w-16 text-center">ID</th>
                        <th class="px-3 py-2">Title</th>
                        <th class="px-3 py-2 hidden sm:table-cell">Artist</th>
                        <th class="px-3 py-2 hidden md:table-cell">Album</th>
                        <th class="px-3 py-2 hidden lg:table-cell w-20 text-center">Format</th>
                        <th class="px-3 py-2 w-32 text-center">Actions</th>
                    </tr>
                </thead>
                <tbody>
                    {#each filteredLibraryTracks.slice(0, 200) as track (track.id || track.checksum)}
                        {#if editingTrackId === track.id}
                            <!-- Inline edit row -->
                            <tr class="border-b border-gray-100 dark:border-gray-800 bg-primary-50 dark:bg-primary-900/20">
                                <td class="px-3 py-2 text-center">
                                    <span class="text-xs font-mono text-gray-400">{track.id}</span>
                                </td>
                                <td class="px-3 py-2">
                                    <input
                                        type="text"
                                        bind:value={editTrackTitle}
                                        placeholder="Title"
                                        class="w-full px-2 py-1 text-sm rounded border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
                                    />
                                </td>
                                <td class="px-3 py-2 hidden sm:table-cell">
                                    <input
                                        type="text"
                                        bind:value={editTrackArtist}
                                        placeholder="Artist"
                                        class="w-full px-2 py-1 text-sm rounded border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
                                    />
                                </td>
                                <td class="px-3 py-2 hidden md:table-cell">
                                    <input
                                        type="text"
                                        bind:value={editTrackAlbum}
                                        placeholder="Album"
                                        class="w-full px-2 py-1 text-sm rounded border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
                                    />
                                </td>
                                <td class="px-3 py-2 hidden lg:table-cell">
                                    <input
                                        type="text"
                                        bind:value={editTrackGenre}
                                        placeholder="Genre"
                                        class="w-full px-2 py-1 text-sm rounded border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
                                    />
                                </td>
                                <td class="px-3 py-2 text-center">
                                    <div class="flex items-center justify-center gap-1">
                                        <button
                                            type="button"
                                            on:click={saveEditTrack}
                                            disabled={savingTrack}
                                            class="px-2 py-1 text-xs font-medium rounded bg-primary-500 text-white hover:bg-primary-600 disabled:opacity-50"
                                        >
                                            {savingTrack ? "..." : "Save"}
                                        </button>
                                        <button
                                            type="button"
                                            on:click={cancelEditTrack}
                                            class="px-2 py-1 text-xs font-medium rounded border border-gray-300 dark:border-gray-600 text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700"
                                        >
                                            Cancel
                                        </button>
                                    </div>
                                </td>
                            </tr>
                        {:else}
                            <!-- Normal display row -->
                            <tr class="border-b border-gray-100 dark:border-gray-800 hover:bg-gray-50 dark:hover:bg-gray-800/50 group">
                                <td class="px-3 py-2 text-center">
                                    <span class="text-xs font-mono text-gray-400 dark:text-gray-500">{track.id}</span>
                                </td>
                                <td class="px-3 py-2">
                                    <p class="text-sm font-medium text-gray-900 dark:text-white truncate" title={track.title}>
                                        {track.title || "Untitled"}
                                    </p>
                                    {#if track.artist}
                                        <p class="text-xs text-gray-500 dark:text-gray-400 truncate sm:hidden mt-0.5">
                                            {track.artist}
                                        </p>
                                    {/if}
                                </td>
                                <td class="px-3 py-2 hidden sm:table-cell">
                                    <span class="text-sm text-gray-600 dark:text-gray-300 truncate block max-w-[200px]">
                                        {track.artist || "—"}
                                    </span>
                                </td>
                                <td class="px-3 py-2 hidden md:table-cell">
                                    <span class="text-sm text-gray-500 dark:text-gray-400 truncate block max-w-[200px]">
                                        {track.album || "—"}
                                    </span>
                                </td>
                                <td class="px-3 py-2 hidden lg:table-cell text-center">
                                    {#if track.format}
                                        <span
                                            class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium uppercase
                                            {track.format === 'flac'
                                                ? 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300'
                                                : track.format === 'mp3'
                                                  ? 'bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300'
                                                  : 'bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-300'}"
                                        >
                                            {track.format}
                                        </span>
                                    {/if}
                                </td>
                                <td class="px-3 py-2 text-center">
                                    <div class="flex items-center justify-center gap-1 opacity-0 group-hover:opacity-100 focus-within:opacity-100 transition-opacity">
                                        <button
                                            type="button"
                                            on:click|stopPropagation={() => startEditTrack(track)}
                                            class="p-1 rounded hover:bg-gray-200 dark:hover:bg-gray-600 text-gray-400 hover:text-gray-700 dark:hover:text-gray-200 transition-colors"
                                            title="Edit metadata"
                                        >
                                            <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
                                                <path stroke-linecap="round" stroke-linejoin="round" d="m16.862 4.487 1.687-1.688a1.875 1.875 0 1 1 2.652 2.652L10.582 16.07a4.5 4.5 0 0 1-1.897 1.13L6 18l.8-2.685a4.5 4.5 0 0 1 1.13-1.897l8.932-8.931Zm0 0L19.5 7.125M18 14v4.75A2.25 2.25 0 0 1 15.75 21H5.25A2.25 2.25 0 0 1 3 18.75V8.25A2.25 2.25 0 0 1 5.25 6H10" />
                                            </svg>
                                        </button>
                                        <button
                                            type="button"
                                            on:click|stopPropagation={() => { confirmDeleteTrackId = track.id; }}
                                            class="p-1 rounded hover:bg-red-100 dark:hover:bg-red-900/40 text-gray-400 hover:text-red-600 dark:hover:text-red-400 transition-colors"
                                            title="Remove from library"
                                        >
                                            <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
                                                <path stroke-linecap="round" stroke-linejoin="round" d="m14.74 9-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 0 1-2.244 2.077H8.084a2.25 2.25 0 0 1-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 0 0-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 0 1 3.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 0 0-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 0 0-7.5 0" />
                                            </svg>
                                        </button>
                                    </div>
                                </td>
                            </tr>
                        {/if}
                    {/each}
                </tbody>
            </table>
        </div>
        {#if filteredLibraryTracks.length > 200}
            <div class="px-4 py-3 text-center text-xs text-gray-400 dark:text-gray-500 border-t border-gray-100 dark:border-gray-800">
                Showing 200 of {filteredLibraryTracks.length} tracks. Use search to narrow results.
            </div>
        {/if}
        <div class="px-3 py-2 text-xs text-gray-400 dark:text-gray-500 border-t border-gray-100 dark:border-gray-800">
            {filteredLibraryTracks.length} track{filteredLibraryTracks.length !== 1 ? "s" : ""} in library
        </div>
    {/if}
</div>

<!-- Confirm delete track modal -->
{#if confirmDeleteTrackId}
    <div
        class="fixed inset-0 z-50 flex items-center justify-center p-4"
        on:click|self={() => { confirmDeleteTrackId = null; confirmDeleteFromDisk = false; }}
        role="dialog"
        aria-modal="true"
    >
        <div class="absolute inset-0 bg-black/50" />
        <div class="relative bg-white dark:bg-gray-800 rounded-2xl shadow-xl max-w-sm w-full p-6">
            <div class="text-center">
                <div class="mx-auto flex items-center justify-center h-12 w-12 rounded-full bg-red-100 dark:bg-red-900/40 mb-4">
                    <svg class="h-6 w-6 text-red-600 dark:text-red-400" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
                        <path stroke-linecap="round" stroke-linejoin="round" d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126ZM12 15.75h.007v.008H12v-.008Z" />
                    </svg>
                </div>
                <h3 class="text-lg font-semibold text-gray-900 dark:text-white mb-2">Delete Track from Library?</h3>
                <p class="text-sm text-gray-500 dark:text-gray-400 mb-4">
                    This will remove the track from the library <strong>and all playlists</strong> that contain it.
                </p>

                <label class="flex items-center gap-2 justify-center mb-6 cursor-pointer select-none">
                    <input
                        type="checkbox"
                        bind:checked={confirmDeleteFromDisk}
                        class="w-4 h-4 rounded border-gray-300 dark:border-gray-600 text-red-600 focus:ring-red-500"
                    />
                    <span class="text-sm text-gray-600 dark:text-gray-400">Also delete file from disk</span>
                </label>

                {#if confirmDeleteFromDisk}
                    <p class="text-xs text-red-500 dark:text-red-400 mb-4 -mt-3">
                        Warning: the audio file will be permanently removed from the server.
                    </p>
                {/if}

                <div class="flex gap-3 justify-center">
                    <button
                        type="button"
                        class="px-4 py-2 text-sm font-medium rounded-lg border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700"
                        on:click={() => { confirmDeleteTrackId = null; confirmDeleteFromDisk = false; }}
                    >
                        Cancel
                    </button>
                    <button
                        type="button"
                        class="px-4 py-2 text-sm font-medium rounded-lg bg-red-600 text-white hover:bg-red-700 transition-colors"
                        on:click={() => handleDeleteTrack(confirmDeleteTrackId)}
                    >
                        {confirmDeleteFromDisk ? "Delete Forever" : "Delete"}
                    </button>
                </div>
            </div>
        </div>
    </div>
{/if}
