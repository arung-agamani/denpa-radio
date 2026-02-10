<script>
    import { onMount } from "svelte";
    import {
        djActiveSection,
        toasts,
        status,
        playlists,
        master,
        scheduler,
    } from "../lib/stores.js";
    import { navigate, segments } from "../lib/router.js";
    import { auth } from "../lib/auth.js";
    import {
        listTracks,
        listOrphanedTracks,
        getPlaylist,
        createPlaylist,
        updatePlaylist,
        deletePlaylist,
        addTrackToPlaylist,
        removeTrackFromPlaylist,
        moveTrackInPlaylist,
        shufflePlaylist,
        exportPlaylist,
        importPlaylist,
        assignPlaylistToTag,
        removePlaylistFromTag,
        reconcile,
        getMasterPlaylist,
    } from "../lib/api.js";
    import TrackList from "../components/TrackList.svelte";
    import NowPlaying from "../components/NowPlaying.svelte";

    // ---------------------------------------------------------------------------
    // Sidebar state
    // ---------------------------------------------------------------------------

    const sections = [
        { id: "dashboard", label: "Dashboard", icon: "📊" },
        { id: "playlists", label: "Playlists", icon: "🎵" },
        { id: "master", label: "Master Playlist", icon: "🕐" },
        { id: "tracks", label: "Tracks", icon: "💿" },
        { id: "importexport", label: "Import / Export", icon: "📦" },
    ];

    let sidebarOpen = false;

    function selectSection(id) {
        $djActiveSection = id;
        sidebarOpen = false;
    }

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

    // Import
    let importText = "";
    let importing = false;

    // Reconcile
    let reconciling = false;
    let reconcileResult = null;

    // Add track modal
    let showAddTrackModal = false;
    let addTrackSearch = "";
    let addTrackSource = "existing"; // 'existing' | 'orphaned'

    // Confirm delete
    let confirmDeleteId = null;

    // Assign to tag
    let assignTag = "morning";
    let assignPlaylistId = null;

    const tagEmoji = {
        morning: "🌅",
        afternoon: "☀️",
        evening: "🌇",
        night: "🌙",
    };
    const tagLabel = {
        morning: "Morning",
        afternoon: "Afternoon",
        evening: "Evening",
        night: "Night",
    };
    const tagColors = {
        morning:
            "bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300 border-amber-200 dark:border-amber-800",
        afternoon:
            "bg-orange-100 text-orange-700 dark:bg-orange-900/40 dark:text-orange-300 border-orange-200 dark:border-orange-800",
        evening:
            "bg-indigo-100 text-indigo-700 dark:bg-indigo-900/40 dark:text-indigo-300 border-indigo-200 dark:border-indigo-800",
        night: "bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-300 border-slate-200 dark:border-slate-700",
    };

    // ---------------------------------------------------------------------------
    // Lifecycle
    // ---------------------------------------------------------------------------

    onMount(() => {
        playlists.refresh();
        master.refresh();
        scheduler.refresh();

        // Check URL segments for deep linking
        if ($segments.length >= 2) {
            const sec = $segments[1];
            if (sections.some((s) => s.id === sec)) {
                $djActiveSection = sec;
            }
        }
    });

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

    async function loadOrphanedTracks() {
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
        if (orphanedTracks.length === 0) await loadOrphanedTracks();
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
    // Import / export
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

    async function handleImport() {
        if (!importText.trim()) {
            toasts.warning("Please paste playlist JSON to import.");
            return;
        }
        importing = true;
        try {
            await importPlaylist(importText.trim());
            toasts.success("Playlist imported!");
            importText = "";
            await playlists.refresh();
            await master.refresh();
        } catch (err) {
            toasts.error("Import failed: " + err.message);
        } finally {
            importing = false;
        }
    }

    async function handleImportFile(e) {
        const file = e.target.files?.[0];
        if (!file) return;
        const text = await file.text();
        importText = text;
    }

    // ---------------------------------------------------------------------------
    // Master playlist management
    // ---------------------------------------------------------------------------

    async function handleAssignToTag() {
        if (!assignPlaylistId) {
            toasts.warning("Please select a playlist.");
            return;
        }
        try {
            await assignPlaylistToTag(assignTag, parseInt(assignPlaylistId));
            toasts.success("Playlist assigned to " + tagLabel[assignTag] + "!");
            assignPlaylistId = null;
            await master.refresh();
            await playlists.refresh();
        } catch (err) {
            toasts.error("Failed to assign: " + err.message);
        }
    }

    async function handleRemoveFromTag(tag, playlistId) {
        try {
            await removePlaylistFromTag(tag, playlistId);
            toasts.success("Playlist removed from " + tagLabel[tag]);
            await master.refresh();
            await playlists.refresh();
        } catch (err) {
            toasts.error("Failed to remove: " + err.message);
        }
    }

    // ---------------------------------------------------------------------------
    // Reconcile
    // ---------------------------------------------------------------------------

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
            // Refresh orphaned tracks
            await loadOrphanedTracks();
        } catch (err) {
            toasts.error("Reconcile failed: " + err.message);
        } finally {
            reconciling = false;
        }
    }

    // ---------------------------------------------------------------------------
    // Filtered tracks for add modal
    // ---------------------------------------------------------------------------

    $: addTrackList =
        addTrackSource === "orphaned" ? orphanedTracks : allTracks;
    $: filteredAddTracks = addTrackSearch.trim()
        ? addTrackList.filter(
              (t) =>
                  (t.title || "")
                      .toLowerCase()
                      .includes(addTrackSearch.toLowerCase()) ||
                  (t.artist || "")
                      .toLowerCase()
                      .includes(addTrackSearch.toLowerCase()) ||
                  (t.album || "")
                      .toLowerCase()
                      .includes(addTrackSearch.toLowerCase()),
          )
        : addTrackList;

    // Playlists for the assign dropdown (from the playlists store)
    $: allPlaylistsList = $playlists || [];
    $: masterData = $master;
    $: schedulerData = $scheduler;
    $: statusData = $status;
</script>

<div class="flex h-[calc(100vh-4rem)]">
    <!-- ===================================================================== -->
    <!-- Sidebar -->
    <!-- ===================================================================== -->

    <!-- Mobile overlay -->
    {#if sidebarOpen}
        <div
            class="fixed inset-0 z-30 bg-black/50 md:hidden"
            on:click={() => (sidebarOpen = false)}
            on:keydown={(e) => {
                if (e.key === "Escape") sidebarOpen = false;
            }}
            role="button"
            tabindex="-1"
            aria-label="Close sidebar"
        ></div>
    {/if}

    <aside
        class="fixed md:static inset-y-0 left-0 z-40 w-64 bg-white dark:bg-gray-800 border-r border-gray-200 dark:border-gray-700 flex flex-col transition-transform duration-300 md:translate-x-0
      {sidebarOpen ? 'translate-x-0' : '-translate-x-full'} md:mt-0 mt-16"
    >
        <div class="p-4 border-b border-gray-200 dark:border-gray-700">
            <h2
                class="text-lg font-bold text-gray-900 dark:text-white flex items-center gap-2"
            >
                <span>🎛️</span> DJ Panel
            </h2>
        </div>

        <nav class="flex-1 overflow-y-auto p-3 space-y-1">
            {#each sections as section}
                <button
                    type="button"
                    class="w-full flex items-center gap-3 px-4 py-2.5 rounded-xl text-sm font-medium transition-all duration-150
            {$djActiveSection === section.id
                        ? 'bg-primary-100 text-primary-700 dark:bg-primary-900/40 dark:text-primary-300 shadow-sm'
                        : 'text-gray-600 dark:text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-700 hover:text-gray-900 dark:hover:text-white'}"
                    on:click={() => selectSection(section.id)}
                >
                    <span class="text-lg">{section.icon}</span>
                    <span>{section.label}</span>
                </button>
            {/each}
        </nav>

        <!-- Sidebar footer -->
        <div class="p-4 border-t border-gray-200 dark:border-gray-700">
            <div
                class="flex items-center gap-2 text-xs text-gray-400 dark:text-gray-500"
            >
                <span class="relative flex h-2 w-2">
                    <span
                        class="animate-ping absolute inline-flex h-full w-full rounded-full bg-green-400 opacity-75"
                    ></span>
                    <span
                        class="relative inline-flex rounded-full h-2 w-2 bg-green-500"
                    ></span>
                </span>
                <span
                    >{statusData.active_clients || 0} listener{(statusData.active_clients ||
                        0) !== 1
                        ? "s"
                        : ""} connected</span
                >
            </div>
        </div>
    </aside>

    <!-- Mobile sidebar toggle -->
    <button
        type="button"
        class="md:hidden fixed bottom-4 left-4 z-50 w-12 h-12 rounded-full bg-primary-500 text-white shadow-lg flex items-center justify-center hover:bg-primary-600 transition-colors"
        on:click={() => (sidebarOpen = !sidebarOpen)}
        aria-label="Toggle sidebar"
    >
        <svg
            class="w-6 h-6"
            fill="none"
            viewBox="0 0 24 24"
            stroke-width="2"
            stroke="currentColor"
        >
            <path
                stroke-linecap="round"
                stroke-linejoin="round"
                d="M3.75 6.75h16.5M3.75 12h16.5m-16.5 5.25h16.5"
            />
        </svg>
    </button>

    <!-- ===================================================================== -->
    <!-- Main content -->
    <!-- ===================================================================== -->

    <div class="flex-1 overflow-y-auto bg-gray-50 dark:bg-gray-900">
        <div class="max-w-5xl mx-auto px-4 sm:px-6 py-6 space-y-6">
            <!-- ================================================================= -->
            <!-- DASHBOARD -->
            <!-- ================================================================= -->
            {#if $djActiveSection === "dashboard"}
                <h1 class="text-2xl font-bold text-gray-900 dark:text-white">
                    Dashboard
                </h1>

                <!-- Now Playing -->
                <NowPlaying />

                <!-- Stats grid -->
                <div class="grid grid-cols-2 md:grid-cols-4 gap-4">
                    <div
                        class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-4"
                    >
                        <p
                            class="text-xs font-semibold uppercase tracking-wider text-gray-400 dark:text-gray-500 mb-1"
                        >
                            Total Tracks
                        </p>
                        <p
                            class="text-2xl font-bold text-gray-900 dark:text-white"
                        >
                            {statusData.total_tracks || 0}
                        </p>
                    </div>
                    <div
                        class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-4"
                    >
                        <p
                            class="text-xs font-semibold uppercase tracking-wider text-gray-400 dark:text-gray-500 mb-1"
                        >
                            Playlists
                        </p>
                        <p
                            class="text-2xl font-bold text-gray-900 dark:text-white"
                        >
                            {allPlaylistsList.length}
                        </p>
                    </div>
                    <div
                        class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-4"
                    >
                        <p
                            class="text-xs font-semibold uppercase tracking-wider text-gray-400 dark:text-gray-500 mb-1"
                        >
                            Listeners
                        </p>
                        <p
                            class="text-2xl font-bold text-gray-900 dark:text-white"
                        >
                            {statusData.active_clients || 0}
                        </p>
                    </div>
                    <div
                        class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-4"
                    >
                        <p
                            class="text-xs font-semibold uppercase tracking-wider text-gray-400 dark:text-gray-500 mb-1"
                        >
                            Active Tag
                        </p>
                        <p
                            class="text-2xl font-bold text-gray-900 dark:text-white flex items-center gap-2"
                        >
                            <span
                                >{tagEmoji[statusData.active_tag] || "🕐"}</span
                            >
                            <span class="text-base"
                                >{tagLabel[statusData.active_tag] ||
                                    statusData.active_tag ||
                                    "—"}</span
                            >
                        </p>
                    </div>
                </div>

                <!-- Scheduler status -->
                <div
                    class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-5"
                >
                    <div class="flex items-center justify-between mb-4">
                        <h3
                            class="text-base font-semibold text-gray-900 dark:text-white"
                        >
                            ⏰ Scheduler
                        </h3>
                        <button
                            type="button"
                            class="text-xs text-primary-600 dark:text-primary-400 hover:underline"
                            on:click={() => scheduler.refresh()}
                        >
                            Refresh
                        </button>
                    </div>

                    <div class="grid grid-cols-2 md:grid-cols-4 gap-3">
                        {#each ["morning", "afternoon", "evening", "night"] as tag}
                            {@const count =
                                (statusData.playlist_summary || {})[tag] || 0}
                            {@const isActive = statusData.active_tag === tag}
                            <div
                                class="rounded-lg border p-3 {isActive
                                    ? 'ring-2 ring-primary-500 '
                                    : ''}{tagColors[tag]}"
                            >
                                <div class="flex items-center gap-2 mb-1">
                                    <span class="text-lg">{tagEmoji[tag]}</span>
                                    <span class="text-sm font-semibold"
                                        >{tagLabel[tag]}</span
                                    >
                                    {#if isActive}
                                        <span
                                            class="ml-auto text-xs font-bold px-1.5 py-0.5 rounded-full bg-primary-200 dark:bg-primary-800 text-primary-700 dark:text-primary-300"
                                            >ACTIVE</span
                                        >
                                    {/if}
                                </div>
                                <p class="text-xs opacity-75">
                                    {count} playlist{count !== 1 ? "s" : ""}
                                </p>
                            </div>
                        {/each}
                    </div>
                </div>

                <!-- Quick actions -->
                <div
                    class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-5"
                >
                    <h3
                        class="text-base font-semibold text-gray-900 dark:text-white mb-4"
                    >
                        ⚡ Quick Actions
                    </h3>
                    <div class="flex flex-wrap gap-3">
                        <button
                            type="button"
                            class="px-4 py-2 text-sm font-medium rounded-lg border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors flex items-center gap-2"
                            on:click={handleReconcile}
                            disabled={reconciling}
                        >
                            {#if reconciling}
                                <svg
                                    class="animate-spin w-4 h-4"
                                    fill="none"
                                    viewBox="0 0 24 24"
                                >
                                    <circle
                                        class="opacity-25"
                                        cx="12"
                                        cy="12"
                                        r="10"
                                        stroke="currentColor"
                                        stroke-width="4"
                                    />
                                    <path
                                        class="opacity-75"
                                        fill="currentColor"
                                        d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"
                                    />
                                </svg>
                                Scanning…
                            {:else}
                                🔄 Reconcile Music Directory
                            {/if}
                        </button>
                        <button
                            type="button"
                            class="px-4 py-2 text-sm font-medium rounded-lg border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
                            on:click={() => {
                                selectSection("playlists");
                            }}
                        >
                            ➕ Create Playlist
                        </button>
                        <button
                            type="button"
                            class="px-4 py-2 text-sm font-medium rounded-lg border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
                            on:click={() => {
                                selectSection("tracks");
                                loadOrphanedTracks();
                            }}
                        >
                            🔍 Check Orphaned Tracks
                        </button>
                    </div>

                    {#if reconcileResult}
                        <div
                            class="mt-4 p-3 rounded-lg bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800 text-sm"
                        >
                            <p class="text-green-800 dark:text-green-300">
                                ✅ Reconcile complete: <strong
                                    >{reconcileResult.removed_count}</strong
                                >
                                track(s) removed,
                                <strong>{reconcileResult.orphaned_count}</strong
                                >
                                new file(s) found. Total tracks:
                                <strong>{reconcileResult.total_tracks}</strong>.
                            </p>
                        </div>
                    {/if}
                </div>

                <!-- ================================================================= -->
                <!-- PLAYLISTS -->
                <!-- ================================================================= -->
            {:else if $djActiveSection === "playlists"}
                <h1 class="text-2xl font-bold text-gray-900 dark:text-white">
                    Playlists
                </h1>

                <!-- Create playlist form -->
                <div
                    class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-5"
                >
                    <h3
                        class="text-base font-semibold text-gray-900 dark:text-white mb-4"
                    >
                        Create New Playlist
                    </h3>
                    <div class="flex flex-col sm:flex-row gap-3">
                        <input
                            type="text"
                            bind:value={newPlaylistName}
                            placeholder="Playlist name"
                            class="flex-1 px-4 py-2.5 rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-white placeholder-gray-400 dark:placeholder-gray-500 text-sm focus:ring-2 focus:ring-primary-500 focus:border-primary-500"
                            on:keydown={(e) => {
                                if (e.key === "Enter") handleCreatePlaylist();
                            }}
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
                <div
                    class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 overflow-hidden"
                >
                    <div
                        class="px-5 py-4 border-b border-gray-200 dark:border-gray-700 flex items-center justify-between"
                    >
                        <h3
                            class="text-base font-semibold text-gray-900 dark:text-white"
                        >
                            All Playlists
                        </h3>
                        <button
                            type="button"
                            class="text-xs text-primary-600 dark:text-primary-400 hover:underline"
                            on:click={() => playlists.refresh()}
                        >
                            Refresh
                        </button>
                    </div>

                    {#if allPlaylistsList.length === 0}
                        <div
                            class="flex flex-col items-center justify-center py-12 text-gray-400 dark:text-gray-500"
                        >
                            <span class="text-4xl mb-3">🎵</span>
                            <p class="text-sm font-medium">No playlists yet</p>
                            <p class="text-xs mt-1">
                                Create one above to get started!
                            </p>
                        </div>
                    {:else}
                        <div
                            class="divide-y divide-gray-100 dark:divide-gray-800"
                        >
                            {#each allPlaylistsList as pl (pl.id)}
                                <div
                                    class="px-5 py-3 flex items-center gap-4 hover:bg-gray-50 dark:hover:bg-gray-800/50 transition-colors group"
                                >
                                    <div
                                        class="flex-shrink-0 w-10 h-10 rounded-lg flex items-center justify-center text-lg {tagColors[
                                            pl.tag
                                        ] || 'bg-gray-100 dark:bg-gray-700'}"
                                    >
                                        {tagEmoji[pl.tag] || "🎵"}
                                    </div>

                                    <div class="flex-1 min-w-0">
                                        <p
                                            class="text-sm font-semibold text-gray-900 dark:text-white truncate"
                                        >
                                            {pl.name}
                                        </p>
                                        <p
                                            class="text-xs text-gray-500 dark:text-gray-400"
                                        >
                                            {tagLabel[pl.tag] || pl.tag} · {pl.trackCount}
                                            track{pl.trackCount !== 1
                                                ? "s"
                                                : ""}
                                        </p>
                                    </div>

                                    <div
                                        class="flex items-center gap-1 opacity-0 group-hover:opacity-100 focus-within:opacity-100 transition-opacity"
                                    >
                                        <button
                                            type="button"
                                            class="p-2 rounded-lg text-gray-400 hover:text-primary-600 hover:bg-primary-50 dark:hover:bg-primary-900/30 dark:hover:text-primary-400 transition-colors"
                                            title="Edit playlist"
                                            on:click={() => {
                                                loadPlaylistDetail(pl.id);
                                            }}
                                        >
                                            <svg
                                                class="w-4 h-4"
                                                fill="none"
                                                viewBox="0 0 24 24"
                                                stroke-width="2"
                                                stroke="currentColor"
                                            >
                                                <path
                                                    stroke-linecap="round"
                                                    stroke-linejoin="round"
                                                    d="m16.862 4.487 1.687-1.688a1.875 1.875 0 1 1 2.652 2.652L10.582 16.07a4.5 4.5 0 0 1-1.897 1.13L6 18l.8-2.685a4.5 4.5 0 0 1 1.13-1.897l8.932-8.931Zm0 0L19.5 7.125M18 14v4.75A2.25 2.25 0 0 1 15.75 21H5.25A2.25 2.25 0 0 1 3 18.75V8.25A2.25 2.25 0 0 1 5.25 6H10"
                                                />
                                            </svg>
                                        </button>
                                        <button
                                            type="button"
                                            class="p-2 rounded-lg text-gray-400 hover:text-blue-600 hover:bg-blue-50 dark:hover:bg-blue-900/30 dark:hover:text-blue-400 transition-colors"
                                            title="Export playlist"
                                            on:click={() => handleExport(pl.id)}
                                        >
                                            <svg
                                                class="w-4 h-4"
                                                fill="none"
                                                viewBox="0 0 24 24"
                                                stroke-width="2"
                                                stroke="currentColor"
                                            >
                                                <path
                                                    stroke-linecap="round"
                                                    stroke-linejoin="round"
                                                    d="M3 16.5v2.25A2.25 2.25 0 0 0 5.25 21h13.5A2.25 2.25 0 0 0 21 18.75V16.5M16.5 12 12 16.5m0 0L7.5 12m4.5 4.5V3"
                                                />
                                            </svg>
                                        </button>
                                        <button
                                            type="button"
                                            class="p-2 rounded-lg text-gray-400 hover:text-red-600 hover:bg-red-50 dark:hover:bg-red-900/30 dark:hover:text-red-400 transition-colors"
                                            title="Delete playlist"
                                            on:click={() =>
                                                (confirmDeleteId = pl.id)}
                                        >
                                            <svg
                                                class="w-4 h-4"
                                                fill="none"
                                                viewBox="0 0 24 24"
                                                stroke-width="2"
                                                stroke="currentColor"
                                            >
                                                <path
                                                    stroke-linecap="round"
                                                    stroke-linejoin="round"
                                                    d="m14.74 9-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 0 1-2.244 2.077H8.084a2.25 2.25 0 0 1-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 0 0-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 0 1 3.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 0 0-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 0 0-7.5 0"
                                                />
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
                        on:keydown={(e) => {
                            if (e.key === "Escape") confirmDeleteId = null;
                        }}
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
                                <div
                                    class="w-12 h-12 mx-auto rounded-full bg-red-100 dark:bg-red-900/30 flex items-center justify-center mb-4"
                                >
                                    <svg
                                        class="w-6 h-6 text-red-500"
                                        fill="none"
                                        viewBox="0 0 24 24"
                                        stroke-width="2"
                                        stroke="currentColor"
                                    >
                                        <path
                                            stroke-linecap="round"
                                            stroke-linejoin="round"
                                            d="m14.74 9-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 0 1-2.244 2.077H8.084a2.25 2.25 0 0 1-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 0 0-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 0 1 3.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 0 0-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 0 0-7.5 0"
                                        />
                                    </svg>
                                </div>
                                <h3
                                    class="text-lg font-bold text-gray-900 dark:text-white mb-2"
                                >
                                    Delete Playlist?
                                </h3>
                                <p
                                    class="text-sm text-gray-500 dark:text-gray-400 mb-6"
                                >
                                    This action cannot be undone. All tracks in
                                    this playlist will be unassigned.
                                </p>
                                <div class="flex gap-3 justify-center">
                                    <button
                                        type="button"
                                        class="px-5 py-2 text-sm font-medium rounded-lg border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
                                        on:click={() =>
                                            (confirmDeleteId = null)}
                                    >
                                        Cancel
                                    </button>
                                    <button
                                        type="button"
                                        class="px-5 py-2 text-sm font-semibold rounded-lg bg-red-600 text-white hover:bg-red-700 transition-colors"
                                        on:click={() =>
                                            handleDeletePlaylist(
                                                confirmDeleteId,
                                            )}
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
                    <div
                        class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 overflow-hidden"
                    >
                        <div
                            class="px-5 py-4 border-b border-gray-200 dark:border-gray-700"
                        >
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
                                        <svg
                                            class="w-5 h-5"
                                            fill="none"
                                            viewBox="0 0 24 24"
                                            stroke-width="2"
                                            stroke="currentColor"
                                        >
                                            <path
                                                stroke-linecap="round"
                                                stroke-linejoin="round"
                                                d="M10.5 19.5 3 12m0 0 7.5-7.5M3 12h18"
                                            />
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
                                            <option value="morning"
                                                >🌅 Morning</option
                                            >
                                            <option value="afternoon"
                                                >☀️ Afternoon</option
                                            >
                                            <option value="evening"
                                                >🌇 Evening</option
                                            >
                                            <option value="night"
                                                >🌙 Night</option
                                            >
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
                                            <h3
                                                class="text-lg font-bold text-gray-900 dark:text-white"
                                            >
                                                {selectedPlaylist.name}
                                            </h3>
                                            <div
                                                class="flex items-center gap-2 mt-0.5"
                                            >
                                                <span
                                                    class="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium border {tagColors[
                                                        selectedPlaylist.tag
                                                    ]}"
                                                >
                                                    {tagEmoji[
                                                        selectedPlaylist.tag
                                                    ]}
                                                    {tagLabel[
                                                        selectedPlaylist.tag
                                                    ]}
                                                </span>
                                                <span
                                                    class="text-xs text-gray-400 dark:text-gray-500"
                                                    >ID: {selectedPlaylist.id}</span
                                                >
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
                                            on:click={() =>
                                                startEdit(selectedPlaylist)}
                                        >
                                            <svg
                                                class="w-4 h-4"
                                                fill="none"
                                                viewBox="0 0 24 24"
                                                stroke-width="2"
                                                stroke="currentColor"
                                            >
                                                <path
                                                    stroke-linecap="round"
                                                    stroke-linejoin="round"
                                                    d="m16.862 4.487 1.687-1.688a1.875 1.875 0 1 1 2.652 2.652L10.582 16.07a4.5 4.5 0 0 1-1.897 1.13L6 18l.8-2.685a4.5 4.5 0 0 1 1.13-1.897l8.932-8.931Zm0 0L19.5 7.125M18 14v4.75A2.25 2.25 0 0 1 15.75 21H5.25A2.25 2.25 0 0 1 3 18.75V8.25A2.25 2.25 0 0 1 5.25 6H10"
                                                />
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
                                <div
                                    class="animate-spin rounded-full h-8 w-8 border-b-2 border-primary-500"
                                ></div>
                            </div>
                        {:else}
                            <TrackList
                                tracks={selectedPlaylist.tracks || []}
                                editable={true}
                                showIndex={true}
                                showFormat={true}
                                highlightChecksum={selectedPlaylist.currentTrackChecksum ||
                                    ""}
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
                        on:keydown={(e) => {
                            if (e.key === "Escape") closeAddTrackModal();
                        }}
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
                            <div
                                class="px-5 py-4 border-b border-gray-200 dark:border-gray-700 flex items-center justify-between"
                            >
                                <h3
                                    class="text-lg font-bold text-gray-900 dark:text-white"
                                >
                                    Add Track
                                </h3>
                                <button
                                    type="button"
                                    class="p-1.5 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-700 text-gray-400"
                                    on:click={closeAddTrackModal}
                                >
                                    <svg
                                        class="w-5 h-5"
                                        fill="none"
                                        viewBox="0 0 24 24"
                                        stroke-width="2"
                                        stroke="currentColor"
                                    >
                                        <path
                                            stroke-linecap="round"
                                            stroke-linejoin="round"
                                            d="M6 18 18 6M6 6l12 12"
                                        />
                                    </svg>
                                </button>
                            </div>

                            <div
                                class="px-5 py-3 border-b border-gray-100 dark:border-gray-700 flex items-center gap-3"
                            >
                                <div
                                    class="flex rounded-lg border border-gray-300 dark:border-gray-600 overflow-hidden text-sm"
                                >
                                    <button
                                        type="button"
                                        class="px-3 py-1.5 font-medium transition-colors {addTrackSource ===
                                        'existing'
                                            ? 'bg-primary-100 text-primary-700 dark:bg-primary-900/40 dark:text-primary-300'
                                            : 'text-gray-600 dark:text-gray-400 hover:bg-gray-50 dark:hover:bg-gray-700'}"
                                        on:click={() =>
                                            (addTrackSource = "existing")}
                                    >
                                        All Tracks
                                    </button>
                                    <button
                                        type="button"
                                        class="px-3 py-1.5 font-medium transition-colors border-l border-gray-300 dark:border-gray-600 {addTrackSource ===
                                        'orphaned'
                                            ? 'bg-primary-100 text-primary-700 dark:bg-primary-900/40 dark:text-primary-300'
                                            : 'text-gray-600 dark:text-gray-400 hover:bg-gray-50 dark:hover:bg-gray-700'}"
                                        on:click={() =>
                                            (addTrackSource = "orphaned")}
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
                                    <div
                                        class="flex items-center justify-center py-12"
                                    >
                                        <div
                                            class="animate-spin rounded-full h-8 w-8 border-b-2 border-primary-500"
                                        ></div>
                                    </div>
                                {:else if filteredAddTracks.length === 0}
                                    <div
                                        class="flex flex-col items-center justify-center py-12 text-gray-400 dark:text-gray-500"
                                    >
                                        <p class="text-sm">No tracks found.</p>
                                    </div>
                                {:else}
                                    <div
                                        class="divide-y divide-gray-100 dark:divide-gray-800"
                                    >
                                        {#each filteredAddTracks.slice(0, 100) as track (track.id || track.checksum)}
                                            <div
                                                class="px-5 py-2.5 flex items-center gap-3 hover:bg-gray-50 dark:hover:bg-gray-800/50 transition-colors"
                                            >
                                                <div class="flex-1 min-w-0">
                                                    <p
                                                        class="text-sm font-medium text-gray-900 dark:text-white truncate"
                                                    >
                                                        {track.title ||
                                                            "Untitled"}
                                                    </p>
                                                    <p
                                                        class="text-xs text-gray-500 dark:text-gray-400 truncate"
                                                    >
                                                        {track.artist || "—"} · {track.album ||
                                                            "—"}
                                                    </p>
                                                </div>
                                                {#if track.format}
                                                    <span
                                                        class="text-xs uppercase font-medium text-gray-400 dark:text-gray-500"
                                                        >{track.format}</span
                                                    >
                                                {/if}
                                                <button
                                                    type="button"
                                                    class="flex-shrink-0 px-3 py-1 text-xs font-semibold rounded-lg bg-primary-100 text-primary-700 dark:bg-primary-900/40 dark:text-primary-300 hover:bg-primary-200 dark:hover:bg-primary-800/60 transition-colors"
                                                    on:click={() =>
                                                        addTrackById(track.id)}
                                                >
                                                    + Add
                                                </button>
                                            </div>
                                        {/each}
                                        {#if filteredAddTracks.length > 100}
                                            <div
                                                class="px-5 py-3 text-center text-xs text-gray-400 dark:text-gray-500"
                                            >
                                                Showing 100 of {filteredAddTracks.length}
                                                — use search to narrow results.
                                            </div>
                                        {/if}
                                    </div>
                                {/if}
                            </div>
                        </div>
                    </div>
                {/if}

                <!-- ================================================================= -->
                <!-- MASTER PLAYLIST -->
                <!-- ================================================================= -->
            {:else if $djActiveSection === "master"}
                <h1 class="text-2xl font-bold text-gray-900 dark:text-white">
                    Master Playlist
                </h1>
                <p class="text-sm text-gray-500 dark:text-gray-400 -mt-4">
                    Assign playlists to time slots. The scheduler automatically
                    switches to the matching slot throughout the day.
                </p>

                <!-- Assign form -->
                <div
                    class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-5"
                >
                    <h3
                        class="text-base font-semibold text-gray-900 dark:text-white mb-4"
                    >
                        Assign Playlist to Time Slot
                    </h3>
                    <div class="flex flex-col sm:flex-row gap-3">
                        <select
                            bind:value={assignTag}
                            class="px-4 py-2.5 rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-white text-sm focus:ring-2 focus:ring-primary-500 focus:border-primary-500"
                        >
                            <option value="morning"
                                >🌅 Morning (6am–12pm)</option
                            >
                            <option value="afternoon"
                                >☀️ Afternoon (12pm–6pm)</option
                            >
                            <option value="evening">🌇 Evening (6pm–9pm)</option
                            >
                            <option value="night">🌙 Night (9pm–6am)</option>
                        </select>
                        <select
                            bind:value={assignPlaylistId}
                            class="flex-1 px-4 py-2.5 rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-white text-sm focus:ring-2 focus:ring-primary-500 focus:border-primary-500"
                        >
                            <option value={null}>Select a playlist…</option>
                            {#each allPlaylistsList as pl}
                                <option value={pl.id}
                                    >{pl.name} ({pl.trackCount} tracks)</option
                                >
                            {/each}
                        </select>
                        <button
                            type="button"
                            class="px-6 py-2.5 text-sm font-semibold text-white bg-primary-600 hover:bg-primary-700 rounded-lg transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                            on:click={handleAssignToTag}
                            disabled={!assignPlaylistId}
                        >
                            Assign
                        </button>
                    </div>
                </div>

                <!-- Time slots -->
                <div class="grid gap-4">
                    {#each ["morning", "afternoon", "evening", "night"] as tag}
                        {@const tagData = (masterData.tags || {})[tag] || {
                            playlists: [],
                            count: 0,
                        }}
                        {@const isActive =
                            (masterData.active_tag || statusData.active_tag) ===
                            tag}
                        <div
                            class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 overflow-hidden {isActive
                                ? 'ring-2 ring-primary-400 dark:ring-primary-600'
                                : ''}"
                        >
                            <div
                                class="px-5 py-4 border-b border-gray-200 dark:border-gray-700 flex items-center justify-between"
                            >
                                <div class="flex items-center gap-3">
                                    <span class="text-2xl">{tagEmoji[tag]}</span
                                    >
                                    <div>
                                        <h3
                                            class="text-base font-bold text-gray-900 dark:text-white"
                                        >
                                            {tagLabel[tag]}
                                            {#if isActive}
                                                <span
                                                    class="ml-2 text-xs font-bold px-2 py-0.5 rounded-full bg-primary-100 dark:bg-primary-900/50 text-primary-700 dark:text-primary-300"
                                                    >ACTIVE</span
                                                >
                                            {/if}
                                        </h3>
                                        <p
                                            class="text-xs text-gray-500 dark:text-gray-400"
                                        >
                                            {tagData.count} playlist{tagData.count !==
                                            1
                                                ? "s"
                                                : ""} assigned
                                        </p>
                                    </div>
                                </div>
                            </div>

                            {#if tagData.playlists && tagData.playlists.length > 0}
                                <div
                                    class="divide-y divide-gray-100 dark:divide-gray-800"
                                >
                                    {#each tagData.playlists as pl (pl.id)}
                                        <div
                                            class="px-5 py-3 flex items-center justify-between hover:bg-gray-50 dark:hover:bg-gray-800/50 transition-colors"
                                        >
                                            <div class="flex-1 min-w-0">
                                                <p
                                                    class="text-sm font-medium text-gray-900 dark:text-white truncate"
                                                >
                                                    {pl.name}
                                                </p>
                                                <p
                                                    class="text-xs text-gray-500 dark:text-gray-400"
                                                >
                                                    {(pl.tracks || []).length} track{(
                                                        pl.tracks || []
                                                    ).length !== 1
                                                        ? "s"
                                                        : ""} · ID: {pl.id}
                                                </p>
                                            </div>
                                            <button
                                                type="button"
                                                class="ml-3 p-2 rounded-lg text-gray-400 hover:text-red-600 hover:bg-red-50 dark:hover:bg-red-900/30 dark:hover:text-red-400 transition-colors"
                                                title="Remove from this time slot"
                                                on:click={() =>
                                                    handleRemoveFromTag(
                                                        tag,
                                                        pl.id,
                                                    )}
                                            >
                                                <svg
                                                    class="w-4 h-4"
                                                    fill="none"
                                                    viewBox="0 0 24 24"
                                                    stroke-width="2"
                                                    stroke="currentColor"
                                                >
                                                    <path
                                                        stroke-linecap="round"
                                                        stroke-linejoin="round"
                                                        d="M6 18 18 6M6 6l12 12"
                                                    />
                                                </svg>
                                            </button>
                                        </div>
                                    {/each}
                                </div>
                            {:else}
                                <div
                                    class="px-5 py-6 text-center text-sm text-gray-400 dark:text-gray-500"
                                >
                                    No playlists assigned. Use the form above to
                                    add one.
                                </div>
                            {/if}
                        </div>
                    {/each}
                </div>

                <!-- Refresh button -->
                <div class="flex justify-center">
                    <button
                        type="button"
                        class="px-4 py-2 text-sm font-medium rounded-lg border border-gray-300 dark:border-gray-600 text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors flex items-center gap-2"
                        on:click={() => {
                            master.refresh();
                            status.refresh();
                        }}
                    >
                        🔄 Refresh
                    </button>
                </div>

                <!-- ================================================================= -->
                <!-- TRACKS -->
                <!-- ================================================================= -->
            {:else if $djActiveSection === "tracks"}
                <h1 class="text-2xl font-bold text-gray-900 dark:text-white">
                    Tracks
                </h1>

                <!-- Actions bar -->
                <div class="flex flex-wrap gap-3">
                    <button
                        type="button"
                        class="px-4 py-2 text-sm font-medium rounded-lg border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors flex items-center gap-2"
                        on:click={loadAllTracks}
                        disabled={loadingTracks}
                    >
                        {#if loadingTracks}
                            <svg
                                class="animate-spin w-4 h-4"
                                fill="none"
                                viewBox="0 0 24 24"
                                ><circle
                                    class="opacity-25"
                                    cx="12"
                                    cy="12"
                                    r="10"
                                    stroke="currentColor"
                                    stroke-width="4"
                                /><path
                                    class="opacity-75"
                                    fill="currentColor"
                                    d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"
                                /></svg
                            >
                        {/if}
                        💿 Load All Tracks
                    </button>
                    <button
                        type="button"
                        class="px-4 py-2 text-sm font-medium rounded-lg border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors flex items-center gap-2"
                        on:click={loadOrphanedTracks}
                        disabled={loadingOrphaned}
                    >
                        {#if loadingOrphaned}
                            <svg
                                class="animate-spin w-4 h-4"
                                fill="none"
                                viewBox="0 0 24 24"
                                ><circle
                                    class="opacity-25"
                                    cx="12"
                                    cy="12"
                                    r="10"
                                    stroke="currentColor"
                                    stroke-width="4"
                                /><path
                                    class="opacity-75"
                                    fill="currentColor"
                                    d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"
                                /></svg
                            >
                        {/if}
                        🔍 Find Orphaned Tracks
                    </button>
                    <button
                        type="button"
                        class="px-4 py-2 text-sm font-medium rounded-lg border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors flex items-center gap-2"
                        on:click={handleReconcile}
                        disabled={reconciling}
                    >
                        {#if reconciling}
                            <svg
                                class="animate-spin w-4 h-4"
                                fill="none"
                                viewBox="0 0 24 24"
                                ><circle
                                    class="opacity-25"
                                    cx="12"
                                    cy="12"
                                    r="10"
                                    stroke="currentColor"
                                    stroke-width="4"
                                /><path
                                    class="opacity-75"
                                    fill="currentColor"
                                    d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"
                                /></svg
                            >
                        {/if}
                        🔄 Reconcile
                    </button>
                </div>

                {#if reconcileResult}
                    <div
                        class="p-3 rounded-lg bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800 text-sm"
                    >
                        <p class="text-green-800 dark:text-green-300">
                            ✅ Reconcile: {reconcileResult.removed_count} removed,
                            {reconcileResult.orphaned_count} new. Total: {reconcileResult.total_tracks}.
                        </p>
                    </div>
                {/if}

                <!-- Orphaned tracks section -->
                {#if orphanedTracks.length > 0}
                    <div
                        class="bg-white dark:bg-gray-800 rounded-xl border border-amber-200 dark:border-amber-800 overflow-hidden"
                    >
                        <div
                            class="px-5 py-4 border-b border-amber-200 dark:border-amber-800 bg-amber-50 dark:bg-amber-900/20"
                        >
                            <h3
                                class="text-base font-semibold text-amber-800 dark:text-amber-300 flex items-center gap-2"
                            >
                                <span>⚠️</span>
                                Orphaned Tracks
                                <span
                                    class="text-xs font-normal text-amber-600 dark:text-amber-400"
                                >
                                    ({orphanedTracks.length} file{orphanedTracks.length !==
                                    1
                                        ? "s"
                                        : ""} on disk not in any playlist)
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

                <!-- All tracks section -->
                {#if allTracks.length > 0}
                    <div
                        class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 overflow-hidden"
                    >
                        <div
                            class="px-5 py-4 border-b border-gray-200 dark:border-gray-700 flex items-center justify-between"
                        >
                            <h3
                                class="text-base font-semibold text-gray-900 dark:text-white"
                            >
                                All Tracks ({allTracks.length})
                            </h3>
                        </div>
                        <TrackList
                            tracks={allTracks}
                            editable={false}
                            showIndex={true}
                            showFormat={true}
                            emptyMessage="No tracks loaded."
                        />
                    </div>
                {:else if !loadingTracks && orphanedTracks.length === 0}
                    <div
                        class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-12 text-center text-gray-400 dark:text-gray-500"
                    >
                        <span class="text-4xl block mb-3">💿</span>
                        <p class="text-sm font-medium">
                            Click "Load All Tracks" to see tracks in your
                            playlists.
                        </p>
                        <p class="text-xs mt-1">
                            Click "Find Orphaned Tracks" to discover unassigned
                            files.
                        </p>
                    </div>
                {/if}

                <!-- ================================================================= -->
                <!-- IMPORT / EXPORT -->
                <!-- ================================================================= -->
            {:else if $djActiveSection === "importexport"}
                <h1 class="text-2xl font-bold text-gray-900 dark:text-white">
                    Import / Export
                </h1>

                <!-- Export section -->
                <div
                    class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-5"
                >
                    <h3
                        class="text-base font-semibold text-gray-900 dark:text-white mb-4 flex items-center gap-2"
                    >
                        <span>📤</span> Export Playlist
                    </h3>
                    <p class="text-sm text-gray-500 dark:text-gray-400 mb-4">
                        Select a playlist to download as a JSON file for backup
                        or sharing.
                    </p>

                    {#if allPlaylistsList.length === 0}
                        <p class="text-sm text-gray-400 dark:text-gray-500">
                            No playlists available to export.
                        </p>
                    {:else}
                        <div class="grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
                            {#each allPlaylistsList as pl (pl.id)}
                                <button
                                    type="button"
                                    class="flex items-center gap-3 p-3 rounded-lg border border-gray-200 dark:border-gray-700 hover:border-primary-300 dark:hover:border-primary-700 hover:bg-primary-50 dark:hover:bg-primary-900/20 transition-colors text-left"
                                    on:click={() => handleExport(pl.id)}
                                >
                                    <span class="text-lg"
                                        >{tagEmoji[pl.tag] || "🎵"}</span
                                    >
                                    <div class="flex-1 min-w-0">
                                        <p
                                            class="text-sm font-medium text-gray-900 dark:text-white truncate"
                                        >
                                            {pl.name}
                                        </p>
                                        <p
                                            class="text-xs text-gray-500 dark:text-gray-400"
                                        >
                                            {pl.trackCount} tracks
                                        </p>
                                    </div>
                                    <svg
                                        class="w-4 h-4 text-gray-400 flex-shrink-0"
                                        fill="none"
                                        viewBox="0 0 24 24"
                                        stroke-width="2"
                                        stroke="currentColor"
                                    >
                                        <path
                                            stroke-linecap="round"
                                            stroke-linejoin="round"
                                            d="M3 16.5v2.25A2.25 2.25 0 0 0 5.25 21h13.5A2.25 2.25 0 0 0 21 18.75V16.5M16.5 12 12 16.5m0 0L7.5 12m4.5 4.5V3"
                                        />
                                    </svg>
                                </button>
                            {/each}
                        </div>
                    {/if}
                </div>

                <!-- Import section -->
                <div
                    class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-5"
                >
                    <h3
                        class="text-base font-semibold text-gray-900 dark:text-white mb-4 flex items-center gap-2"
                    >
                        <span>📥</span> Import Playlist
                    </h3>
                    <p class="text-sm text-gray-500 dark:text-gray-400 mb-4">
                        Upload a previously exported playlist JSON file, or
                        paste the JSON directly.
                    </p>

                    <!-- File upload -->
                    <div class="mb-4">
                        <label
                            class="flex flex-col items-center justify-center w-full h-32 rounded-xl border-2 border-dashed border-gray-300 dark:border-gray-600 hover:border-primary-400 dark:hover:border-primary-600 bg-gray-50 dark:bg-gray-700/30 cursor-pointer transition-colors"
                        >
                            <div
                                class="flex flex-col items-center text-gray-500 dark:text-gray-400"
                            >
                                <svg
                                    class="w-8 h-8 mb-2"
                                    fill="none"
                                    viewBox="0 0 24 24"
                                    stroke-width="1.5"
                                    stroke="currentColor"
                                >
                                    <path
                                        stroke-linecap="round"
                                        stroke-linejoin="round"
                                        d="M3 16.5v2.25A2.25 2.25 0 0 0 5.25 21h13.5A2.25 2.25 0 0 0 21 18.75V16.5m-13.5-9L12 3m0 0 4.5 4.5M12 3v13.5"
                                    />
                                </svg>
                                <p class="text-sm font-medium">
                                    Click to upload a JSON file
                                </p>
                                <p
                                    class="text-xs text-gray-400 dark:text-gray-500"
                                >
                                    or drag and drop
                                </p>
                            </div>
                            <input
                                type="file"
                                accept=".json"
                                class="hidden"
                                on:change={handleImportFile}
                            />
                        </label>
                    </div>

                    <!-- JSON textarea -->
                    <div class="mb-4">
                        <label
                            for="import-json"
                            class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1.5"
                        >
                            Or paste JSON directly:
                        </label>
                        <textarea
                            id="import-json"
                            bind:value={importText}
                            rows="6"
                            placeholder={'{"id": 1, "name": "My Playlist", "tag": "morning", "tracks": [...]}'}
                            class="w-full px-4 py-3 rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-white placeholder-gray-400 dark:placeholder-gray-500 text-sm font-mono focus:ring-2 focus:ring-primary-500 focus:border-primary-500 resize-y"
                        ></textarea>
                    </div>

                    <button
                        type="button"
                        class="px-6 py-2.5 text-sm font-semibold text-white bg-primary-600 hover:bg-primary-700 rounded-lg transition-colors disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-2"
                        on:click={handleImport}
                        disabled={importing || !importText.trim()}
                    >
                        {#if importing}
                            <svg
                                class="animate-spin w-4 h-4"
                                fill="none"
                                viewBox="0 0 24 24"
                                ><circle
                                    class="opacity-25"
                                    cx="12"
                                    cy="12"
                                    r="10"
                                    stroke="currentColor"
                                    stroke-width="4"
                                /><path
                                    class="opacity-75"
                                    fill="currentColor"
                                    d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"
                                /></svg
                            >
                            Importing…
                        {:else}
                            📥 Import Playlist
                        {/if}
                    </button>
                </div>
            {/if}
        </div>
    </div>
</div>
