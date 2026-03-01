<script>
    import {
        getPlaylist,
        createPlaylist,
        updatePlaylist,
        deletePlaylist,
        addTrackToPlaylist,
        removeTrackFromPlaylist,
        moveTrackInPlaylist,
        shufflePlaylist,
        exportPlaylist,
        listTracks,
        listOrphanedTracks,
    } from "../../lib/api.js";
    import { playlists, master, toasts } from "../../lib/stores.js";
    import { tagEmoji, tagLabel, tagColors } from "../../lib/tags.js";
    import TrackList from "../../components/TrackList.svelte";

    // ---------------------------------------------------------------------------
    // Shared data
    // ---------------------------------------------------------------------------

    let allTracks = [];
    let orphanedTracks = [];
    let loadingTracks = false;
    let loadingOrphaned = false;

    // Selected playlist for editing
    let selectedPlaylistId = null;
    let selectedPlaylist = null;
    let loadingPlaylist = false;

    // Create playlist form
    let newPlaylistName = "";
    let newPlaylistTag = "morning";
    let creatingPlaylist = false;

    // Edit playlist form
    let editingPlaylist = false;
    let editName = "";
    let editTag = "";

    // Add track modal
    let showAddTrackModal = false;
    let addTrackSearch = "";
    let addTrackSource = "existing"; // 'existing' | 'orphaned'

    // Confirm delete
    let confirmDeleteId = null;

    // ---------------------------------------------------------------------------
    // Data loading helpers
    // ---------------------------------------------------------------------------

    async function loadAllTracks() {
        loadingTracks = true;
        try {
            const data = await listTracks();
            allTracks = data.tracks || [];
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

    async function loadPlaylistDetail(id) {
        loadingPlaylist = true;
        selectedPlaylistId = id;
        try {
            const data = await getPlaylist(id);
            selectedPlaylist = data.playlist;
        } catch (err) {
            toasts.error("Failed to load playlist: " + err.message);
            selectedPlaylist = null;
        } finally {
            loadingPlaylist = false;
        }
    }

    // ---------------------------------------------------------------------------
    // Playlist CRUD
    // ---------------------------------------------------------------------------

    async function handleCreatePlaylist() {
        if (!newPlaylistName.trim()) {
            toasts.warning("Please enter a playlist name.");
            return;
        }
        creatingPlaylist = true;
        try {
            await createPlaylist(newPlaylistName.trim(), newPlaylistTag);
            toasts.success("Playlist created!");
            newPlaylistName = "";
            await playlists.refresh();
            await master.refresh();
        } catch (err) {
            toasts.error("Failed to create playlist: " + err.message);
        } finally {
            creatingPlaylist = false;
        }
    }

    function startEdit(pl) {
        editingPlaylist = true;
        editName = pl.name || pl.Name;
        editTag = pl.tag || pl.Tag;
        selectedPlaylistId = pl.id || pl.ID;
    }

    async function saveEdit() {
        if (!editName.trim()) {
            toasts.warning("Playlist name cannot be empty.");
            return;
        }
        try {
            await updatePlaylist(selectedPlaylistId, {
                name: editName.trim(),
                tag: editTag,
            });
            toasts.success("Playlist updated!");
            editingPlaylist = false;
            await playlists.refresh();
            await master.refresh();
            if (selectedPlaylist) {
                await loadPlaylistDetail(selectedPlaylistId);
            }
        } catch (err) {
            toasts.error("Failed to update playlist: " + err.message);
        }
    }

    function cancelEdit() {
        editingPlaylist = false;
    }

    async function handleDeletePlaylist(id) {
        try {
            await deletePlaylist(id);
            toasts.success("Playlist deleted.");
            confirmDeleteId = null;
            if (selectedPlaylistId === id) {
                selectedPlaylist = null;
                selectedPlaylistId = null;
            }
            await playlists.refresh();
            await master.refresh();
        } catch (err) {
            toasts.error("Failed to delete playlist: " + err.message);
        }
    }

    // ---------------------------------------------------------------------------
    // Track manipulation
    // ---------------------------------------------------------------------------

    async function handleRemoveTrack(e) {
        const { track } = e.detail;
        if (!selectedPlaylistId) return;
        try {
            await removeTrackFromPlaylist(selectedPlaylistId, track.id);
            toasts.success(`Removed "${track.title}"`);
            await loadPlaylistDetail(selectedPlaylistId);
            await playlists.refresh();
        } catch (err) {
            toasts.error("Failed to remove track: " + err.message);
        }
    }

    async function handleMoveTrack(e) {
        const { from, to } = e.detail;
        if (!selectedPlaylistId) return;
        try {
            await moveTrackInPlaylist(selectedPlaylistId, from, to);
            await loadPlaylistDetail(selectedPlaylistId);
        } catch (err) {
            toasts.error("Failed to move track: " + err.message);
        }
    }

    async function handleShuffle() {
        if (!selectedPlaylistId) return;
        try {
            await shufflePlaylist(selectedPlaylistId);
            toasts.success("Playlist shuffled!");
            await loadPlaylistDetail(selectedPlaylistId);
        } catch (err) {
            toasts.error("Failed to shuffle: " + err.message);
        }
    }

    async function openAddTrackModal() {
        showAddTrackModal = true;
        addTrackSearch = "";
        addTrackSource = "existing";
        if (allTracks.length === 0) await loadAllTracks();
        if (orphanedTracks.length === 0) await loadOrphanedTracksData();
    }

    async function addTrackById(trackId) {
        if (!selectedPlaylistId) return;
        try {
            await addTrackToPlaylist(selectedPlaylistId, { trackId });
            toasts.success("Track added!");
            await loadPlaylistDetail(selectedPlaylistId);
            await playlists.refresh();
        } catch (err) {
            toasts.error("Failed to add track: " + err.message);
        }
    }

    function closeAddTrackModal() {
        showAddTrackModal = false;
    }

    // ---------------------------------------------------------------------------
    // Export
    // ---------------------------------------------------------------------------

    async function handleExport(id) {
        try {
            const { blob, filename } = await exportPlaylist(id);
            const url = URL.createObjectURL(blob);
            const a = document.createElement("a");
            a.href = url;
            a.download = filename;
            document.body.appendChild(a);
            a.click();
            a.remove();
            URL.revokeObjectURL(url);
            toasts.success("Playlist exported!");
        } catch (err) {
            toasts.error("Export failed: " + err.message);
        }
    }

    // ---------------------------------------------------------------------------
    // Derived
    // ---------------------------------------------------------------------------

    $: allPlaylistsList = $playlists || [];
    $: addTrackList = addTrackSource === "orphaned" ? orphanedTracks : allTracks;
    $: filteredAddTracks = addTrackSearch.trim()
        ? addTrackList.filter(
              (t) =>
                  (t.title || "").toLowerCase().includes(addTrackSearch.toLowerCase()) ||
                  (t.artist || "").toLowerCase().includes(addTrackSearch.toLowerCase()) ||
                  (t.album || "").toLowerCase().includes(addTrackSearch.toLowerCase()),
          )
        : addTrackList;
</script>

<h1 class="text-2xl font-bold text-gray-900 dark:text-white">Playlists</h1>

<!-- Create playlist form -->
<div class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-5">
    <h3 class="text-base font-semibold text-gray-900 dark:text-white mb-4">Create New Playlist</h3>
    <div class="flex flex-col sm:flex-row gap-3">
        <input
            type="text"
            bind:value={newPlaylistName}
            placeholder="Playlist name"
            class="flex-1 px-4 py-2.5 rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-white placeholder-gray-400 dark:placeholder-gray-500 text-sm focus:ring-2 focus:ring-primary-500 focus:border-primary-500"
            on:keydown={(e) => { if (e.key === "Enter") handleCreatePlaylist(); }}
        />
        <select
            bind:value={newPlaylistTag}
            class="px-4 py-2.5 rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-white text-sm focus:ring-2 focus:ring-primary-500 focus:border-primary-500"
        >
            <option value="morning">🌅 Morning</option>
            <option value="afternoon">☀️ Afternoon</option>
            <option value="evening">🌇 Evening</option>
            <option value="night">🌙 Night</option>
        </select>
        <button
            type="button"
            class="px-6 py-2.5 text-sm font-semibold text-white bg-primary-600 hover:bg-primary-700 rounded-lg transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
            on:click={handleCreatePlaylist}
            disabled={creatingPlaylist}
        >
            {creatingPlaylist ? "Creating…" : "Create"}
        </button>
    </div>
</div>

<!-- Playlist list -->
<div class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 overflow-hidden">
    <div class="px-5 py-4 border-b border-gray-200 dark:border-gray-700 flex items-center justify-between">
        <h3 class="text-base font-semibold text-gray-900 dark:text-white">All Playlists</h3>
        <button
            type="button"
            class="text-xs text-primary-600 dark:text-primary-400 hover:underline"
            on:click={() => playlists.refresh()}
        >
            Refresh
        </button>
    </div>

    {#if allPlaylistsList.length === 0}
        <div class="flex flex-col items-center justify-center py-12 text-gray-400 dark:text-gray-500">
            <span class="text-4xl mb-3">🎵</span>
            <p class="text-sm font-medium">No playlists yet</p>
            <p class="text-xs mt-1">Create one above to get started!</p>
        </div>
    {:else}
        <div class="divide-y divide-gray-100 dark:divide-gray-800">
            {#each allPlaylistsList as pl (pl.id)}
                <div class="px-5 py-3 flex items-center gap-4 hover:bg-gray-50 dark:hover:bg-gray-800/50 transition-colors group">
                    <div
                        class="flex-shrink-0 w-10 h-10 rounded-lg flex items-center justify-center text-lg {tagColors[pl.tag] || 'bg-gray-100 dark:bg-gray-700'}"
                    >
                        {tagEmoji[pl.tag] || "🎵"}
                    </div>

                    <div class="flex-1 min-w-0">
                        <p class="text-sm font-semibold text-gray-900 dark:text-white truncate">{pl.name}</p>
                        <p class="text-xs text-gray-500 dark:text-gray-400">
                            {tagLabel[pl.tag] || pl.tag} · {pl.trackCount} track{pl.trackCount !== 1 ? "s" : ""}
                        </p>
                    </div>

                    <div class="flex items-center gap-1 opacity-0 group-hover:opacity-100 focus-within:opacity-100 transition-opacity">
                        <button
                            type="button"
                            class="p-2 rounded-lg text-gray-400 hover:text-primary-600 hover:bg-primary-50 dark:hover:bg-primary-900/30 dark:hover:text-primary-400 transition-colors"
                            title="Edit playlist"
                            on:click={() => loadPlaylistDetail(pl.id)}
                        >
                            <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
                                <path stroke-linecap="round" stroke-linejoin="round" d="m16.862 4.487 1.687-1.688a1.875 1.875 0 1 1 2.652 2.652L10.582 16.07a4.5 4.5 0 0 1-1.897 1.13L6 18l.8-2.685a4.5 4.5 0 0 1 1.13-1.897l8.932-8.931Zm0 0L19.5 7.125M18 14v4.75A2.25 2.25 0 0 1 15.75 21H5.25A2.25 2.25 0 0 1 3 18.75V8.25A2.25 2.25 0 0 1 5.25 6H10" />
                            </svg>
                        </button>
                        <button
                            type="button"
                            class="p-2 rounded-lg text-gray-400 hover:text-blue-600 hover:bg-blue-50 dark:hover:bg-blue-900/30 dark:hover:text-blue-400 transition-colors"
                            title="Export playlist"
                            on:click={() => handleExport(pl.id)}
                        >
                            <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
                                <path stroke-linecap="round" stroke-linejoin="round" d="M3 16.5v2.25A2.25 2.25 0 0 0 5.25 21h13.5A2.25 2.25 0 0 0 21 18.75V16.5M16.5 12 12 16.5m0 0L7.5 12m4.5 4.5V3" />
                            </svg>
                        </button>
                        <button
                            type="button"
                            class="p-2 rounded-lg text-gray-400 hover:text-red-600 hover:bg-red-50 dark:hover:bg-red-900/30 dark:hover:text-red-400 transition-colors"
                            title="Delete playlist"
                            on:click={() => (confirmDeleteId = pl.id)}
                        >
                            <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
                                <path stroke-linecap="round" stroke-linejoin="round" d="m14.74 9-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 0 1-2.244 2.077H8.084a2.25 2.25 0 0 1-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 0 0-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 0 1 3.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 0 0-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 0 0-7.5 0" />
                            </svg>
                        </button>
                    </div>
                </div>
            {/each}
        </div>
    {/if}
</div>

<!-- Delete confirmation modal -->
{#if confirmDeleteId}
    <div
        class="fixed inset-0 z-50 flex items-center justify-center bg-black/50"
        on:click={() => (confirmDeleteId = null)}
        on:keydown={(e) => { if (e.key === "Escape") confirmDeleteId = null; }}
        role="button"
        tabindex="-1"
        aria-label="Close"
    >
        <!-- svelte-ignore a11y-no-noninteractive-element-interactions -->
        <div
            class="bg-white dark:bg-gray-800 rounded-2xl shadow-2xl border border-gray-200 dark:border-gray-700 p-6 max-w-sm w-full mx-4"
            on:click|stopPropagation
            on:keydown|stopPropagation
            role="dialog"
            aria-modal="true"
            tabindex="-1"
        >
            <div class="text-center">
                <div class="w-12 h-12 mx-auto rounded-full bg-red-100 dark:bg-red-900/30 flex items-center justify-center mb-4">
                    <svg class="w-6 h-6 text-red-500" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
                        <path stroke-linecap="round" stroke-linejoin="round" d="m14.74 9-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 0 1-2.244 2.077H8.084a2.25 2.25 0 0 1-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 0 0-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 0 1 3.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 0 0-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 0 0-7.5 0" />
                    </svg>
                </div>
                <h3 class="text-lg font-bold text-gray-900 dark:text-white mb-2">Delete Playlist?</h3>
                <p class="text-sm text-gray-500 dark:text-gray-400 mb-6">
                    This action cannot be undone. All tracks in this playlist will be unassigned.
                </p>
                <div class="flex gap-3 justify-center">
                    <button
                        type="button"
                        class="px-5 py-2 text-sm font-medium rounded-lg border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
                        on:click={() => (confirmDeleteId = null)}
                    >
                        Cancel
                    </button>
                    <button
                        type="button"
                        class="px-5 py-2 text-sm font-semibold rounded-lg bg-red-600 text-white hover:bg-red-700 transition-colors"
                        on:click={() => handleDeletePlaylist(confirmDeleteId)}
                    >
                        Delete
                    </button>
                </div>
            </div>
        </div>
    </div>
{/if}

<!-- Playlist detail / editor -->
{#if selectedPlaylist}
    <div class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 overflow-hidden">
        <div class="px-5 py-4 border-b border-gray-200 dark:border-gray-700">
            <div class="flex items-center justify-between">
                <div class="flex items-center gap-3">
                    <button
                        type="button"
                        class="p-1.5 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-700 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 transition-colors"
                        on:click={() => {
                            selectedPlaylist = null;
                            selectedPlaylistId = null;
                            editingPlaylist = false;
                        }}
                        title="Back to list"
                    >
                        <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
                            <path stroke-linecap="round" stroke-linejoin="round" d="M10.5 19.5 3 12m0 0 7.5-7.5M3 12h18" />
                        </svg>
                    </button>

                    {#if editingPlaylist}
                        <input
                            type="text"
                            bind:value={editName}
                            class="text-lg font-bold bg-transparent border-b-2 border-primary-500 text-gray-900 dark:text-white focus:outline-none px-1"
                        />
                        <select
                            bind:value={editTag}
                            class="text-sm rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-white px-2 py-1"
                        >
                            <option value="morning">🌅 Morning</option>
                            <option value="afternoon">☀️ Afternoon</option>
                            <option value="evening">🌇 Evening</option>
                            <option value="night">🌙 Night</option>
                        </select>
                        <button
                            type="button"
                            class="text-sm text-primary-600 dark:text-primary-400 font-semibold hover:underline"
                            on:click={saveEdit}>Save</button
                        >
                        <button
                            type="button"
                            class="text-sm text-gray-400 hover:underline"
                            on:click={cancelEdit}>Cancel</button
                        >
                    {:else}
                        <div>
                            <h3 class="text-lg font-bold text-gray-900 dark:text-white">
                                {selectedPlaylist.name}
                            </h3>
                            <div class="flex items-center gap-2 mt-0.5">
                                <span
                                    class="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium border {tagColors[selectedPlaylist.tag]}"
                                >
                                    {tagEmoji[selectedPlaylist.tag]}
                                    {tagLabel[selectedPlaylist.tag]}
                                </span>
                                <span class="text-xs text-gray-400 dark:text-gray-500">ID: {selectedPlaylist.id}</span>
                            </div>
                        </div>
                    {/if}
                </div>

                <div class="flex items-center gap-2">
                    {#if !editingPlaylist}
                        <button
                            type="button"
                            class="p-2 rounded-lg text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
                            title="Edit playlist info"
                            on:click={() => startEdit(selectedPlaylist)}
                        >
                            <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
                                <path stroke-linecap="round" stroke-linejoin="round" d="m16.862 4.487 1.687-1.688a1.875 1.875 0 1 1 2.652 2.652L10.582 16.07a4.5 4.5 0 0 1-1.897 1.13L6 18l.8-2.685a4.5 4.5 0 0 1 1.13-1.897l8.932-8.931Zm0 0L19.5 7.125M18 14v4.75A2.25 2.25 0 0 1 15.75 21H5.25A2.25 2.25 0 0 1 3 18.75V8.25A2.25 2.25 0 0 1 5.25 6H10" />
                            </svg>
                        </button>
                    {/if}
                    <button
                        type="button"
                        class="px-3 py-1.5 text-xs font-semibold rounded-lg bg-primary-100 text-primary-700 dark:bg-primary-900/40 dark:text-primary-300 hover:bg-primary-200 dark:hover:bg-primary-800/60 transition-colors"
                        on:click={openAddTrackModal}
                    >
                        + Add Track
                    </button>
                    <button
                        type="button"
                        class="px-3 py-1.5 text-xs font-medium rounded-lg border border-gray-300 dark:border-gray-600 text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
                        on:click={handleShuffle}
                    >
                        🔀 Shuffle
                    </button>
                </div>
            </div>
        </div>

        {#if loadingPlaylist}
            <div class="flex items-center justify-center py-12">
                <div class="animate-spin rounded-full h-8 w-8 border-b-2 border-primary-500"></div>
            </div>
        {:else}
            <TrackList
                tracks={selectedPlaylist.tracks || []}
                editable={true}
                showIndex={true}
                showFormat={true}
                highlightChecksum={selectedPlaylist.currentTrackChecksum || ""}
                on:remove={handleRemoveTrack}
                on:move={handleMoveTrack}
            />
        {/if}
    </div>
{/if}

<!-- Add Track Modal -->
{#if showAddTrackModal}
    <div
        class="fixed inset-0 z-50 flex items-start justify-center bg-black/50 pt-20 px-4"
        on:click={closeAddTrackModal}
        on:keydown={(e) => { if (e.key === "Escape") closeAddTrackModal(); }}
        role="button"
        tabindex="-1"
        aria-label="Close"
    >
        <!-- svelte-ignore a11y-no-noninteractive-element-interactions -->
        <div
            class="bg-white dark:bg-gray-800 rounded-2xl shadow-2xl border border-gray-200 dark:border-gray-700 w-full max-w-2xl max-h-[70vh] flex flex-col"
            on:click|stopPropagation
            on:keydown|stopPropagation
            role="dialog"
            aria-modal="true"
            tabindex="-1"
        >
            <div class="px-5 py-4 border-b border-gray-200 dark:border-gray-700 flex items-center justify-between">
                <h3 class="text-lg font-bold text-gray-900 dark:text-white">Add Track</h3>
                <button
                    type="button"
                    class="p-1.5 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-700 text-gray-400"
                    on:click={closeAddTrackModal}
                >
                    <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
                        <path stroke-linecap="round" stroke-linejoin="round" d="M6 18 18 6M6 6l12 12" />
                    </svg>
                </button>
            </div>

            <div class="px-5 py-3 border-b border-gray-100 dark:border-gray-700 flex items-center gap-3">
                <div class="flex rounded-lg border border-gray-300 dark:border-gray-600 overflow-hidden text-sm">
                    <button
                        type="button"
                        class="px-3 py-1.5 font-medium transition-colors {addTrackSource === 'existing'
                            ? 'bg-primary-100 text-primary-700 dark:bg-primary-900/40 dark:text-primary-300'
                            : 'text-gray-600 dark:text-gray-400 hover:bg-gray-50 dark:hover:bg-gray-700'}"
                        on:click={() => (addTrackSource = "existing")}
                    >
                        All Tracks
                    </button>
                    <button
                        type="button"
                        class="px-3 py-1.5 font-medium transition-colors border-l border-gray-300 dark:border-gray-600 {addTrackSource === 'orphaned'
                            ? 'bg-primary-100 text-primary-700 dark:bg-primary-900/40 dark:text-primary-300'
                            : 'text-gray-600 dark:text-gray-400 hover:bg-gray-50 dark:hover:bg-gray-700'}"
                        on:click={() => (addTrackSource = "orphaned")}
                    >
                        Orphaned ({orphanedTracks.length})
                    </button>
                </div>
                <input
                    type="text"
                    bind:value={addTrackSearch}
                    placeholder="Search tracks…"
                    class="flex-1 px-3 py-1.5 rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-white text-sm focus:ring-2 focus:ring-primary-500 focus:border-primary-500"
                />
            </div>

            <div class="flex-1 overflow-y-auto">
                {#if (loadingTracks && addTrackSource === "existing") || (loadingOrphaned && addTrackSource === "orphaned")}
                    <div class="flex items-center justify-center py-12">
                        <div class="animate-spin rounded-full h-8 w-8 border-b-2 border-primary-500"></div>
                    </div>
                {:else if filteredAddTracks.length === 0}
                    <div class="flex flex-col items-center justify-center py-12 text-gray-400 dark:text-gray-500">
                        <p class="text-sm">No tracks found.</p>
                    </div>
                {:else}
                    <div class="divide-y divide-gray-100 dark:divide-gray-800">
                        {#each filteredAddTracks.slice(0, 100) as track (track.id || track.checksum)}
                            <div class="px-5 py-2.5 flex items-center gap-3 hover:bg-gray-50 dark:hover:bg-gray-800/50 transition-colors">
                                <div class="flex-1 min-w-0">
                                    <p class="text-sm font-medium text-gray-900 dark:text-white truncate">
                                        {track.title || "Untitled"}
                                    </p>
                                    <p class="text-xs text-gray-500 dark:text-gray-400 truncate">
                                        {track.artist || "—"} · {track.album || "—"}
                                    </p>
                                </div>
                                {#if track.format}
                                    <span class="text-xs uppercase font-medium text-gray-400 dark:text-gray-500">{track.format}</span>
                                {/if}
                                <button
                                    type="button"
                                    class="flex-shrink-0 px-3 py-1 text-xs font-semibold rounded-lg bg-primary-100 text-primary-700 dark:bg-primary-900/40 dark:text-primary-300 hover:bg-primary-200 dark:hover:bg-primary-800/60 transition-colors"
                                    on:click={() => addTrackById(track.id)}
                                >
                                    + Add
                                </button>
                            </div>
                        {/each}
                        {#if filteredAddTracks.length > 100}
                            <div class="px-5 py-3 text-center text-xs text-gray-400 dark:text-gray-500">
                                Showing 100 of {filteredAddTracks.length} — use search to narrow results.
                            </div>
                        {/if}
                    </div>
                {/if}
            </div>
        </div>
    </div>
{/if}
