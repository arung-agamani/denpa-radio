<script>
    import { assignPlaylistToTag, removePlaylistFromTag } from "../../lib/api.js";
    import { playlists, master, status, toasts } from "../../lib/stores.js";
    import { tagEmoji, tagLabel, tagColors } from "../../lib/tags.js";

    // ---------------------------------------------------------------------------
    // Assign form state
    // ---------------------------------------------------------------------------

    let assignTag = "morning";
    let assignPlaylistId = null;

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
    // Derived
    // ---------------------------------------------------------------------------

    $: allPlaylistsList = $playlists || [];
    $: masterData = $master;
    $: statusData = $status;
</script>

<h1 class="text-2xl font-bold text-gray-900 dark:text-white">Master Playlist</h1>
<p class="text-sm text-gray-500 dark:text-gray-400 -mt-4">
    Assign playlists to time slots. The scheduler automatically switches to the matching slot throughout the day.
</p>

<!-- Assign form -->
<div class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-5">
    <h3 class="text-base font-semibold text-gray-900 dark:text-white mb-4">Assign Playlist to Time Slot</h3>
    <div class="flex flex-col sm:flex-row gap-3">
        <select
            bind:value={assignTag}
            class="px-4 py-2.5 rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-white text-sm focus:ring-2 focus:ring-primary-500 focus:border-primary-500"
        >
            <option value="morning">🌅 Morning (6am–12pm)</option>
            <option value="afternoon">☀️ Afternoon (12pm–6pm)</option>
            <option value="evening">🌇 Evening (6pm–9pm)</option>
            <option value="night">🌙 Night (9pm–6am)</option>
        </select>
        <select
            bind:value={assignPlaylistId}
            class="flex-1 px-4 py-2.5 rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-white text-sm focus:ring-2 focus:ring-primary-500 focus:border-primary-500"
        >
            <option value={null}>Select a playlist…</option>
            {#each allPlaylistsList as pl}
                <option value={pl.id}>{pl.name} ({pl.trackCount} tracks)</option>
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
        {@const tagData = (masterData.tags || {})[tag] || { playlists: [], count: 0 }}
        {@const isActive = (masterData.active_tag || statusData.active_tag) === tag}
        <div
            class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 overflow-hidden {isActive
                ? 'ring-2 ring-primary-400 dark:ring-primary-600'
                : ''}"
        >
            <div class="px-5 py-4 border-b border-gray-200 dark:border-gray-700 flex items-center justify-between">
                <div class="flex items-center gap-3">
                    <span class="text-2xl">{tagEmoji[tag]}</span>
                    <div>
                        <h3 class="text-base font-bold text-gray-900 dark:text-white">
                            {tagLabel[tag]}
                            {#if isActive}
                                <span class="ml-2 text-xs font-bold px-2 py-0.5 rounded-full bg-primary-100 dark:bg-primary-900/50 text-primary-700 dark:text-primary-300">ACTIVE</span>
                            {/if}
                        </h3>
                        <p class="text-xs text-gray-500 dark:text-gray-400">
                            {tagData.count} playlist{tagData.count !== 1 ? "s" : ""} assigned
                        </p>
                    </div>
                </div>
            </div>

            {#if tagData.playlists && tagData.playlists.length > 0}
                <div class="divide-y divide-gray-100 dark:divide-gray-800">
                    {#each tagData.playlists as pl (pl.id)}
                        <div class="px-5 py-3 flex items-center justify-between hover:bg-gray-50 dark:hover:bg-gray-800/50 transition-colors">
                            <div class="flex-1 min-w-0">
                                <p class="text-sm font-medium text-gray-900 dark:text-white truncate">{pl.name}</p>
                                <p class="text-xs text-gray-500 dark:text-gray-400">
                                    {(pl.tracks || []).length} track{(pl.tracks || []).length !== 1 ? "s" : ""} · ID: {pl.id}
                                </p>
                            </div>
                            <button
                                type="button"
                                class="ml-3 p-2 rounded-lg text-gray-400 hover:text-red-600 hover:bg-red-50 dark:hover:bg-red-900/30 dark:hover:text-red-400 transition-colors"
                                title="Remove from this time slot"
                                on:click={() => handleRemoveFromTag(tag, pl.id)}
                            >
                                <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
                                    <path stroke-linecap="round" stroke-linejoin="round" d="M6 18 18 6M6 6l12 12" />
                                </svg>
                            </button>
                        </div>
                    {/each}
                </div>
            {:else}
                <div class="px-5 py-6 text-center text-sm text-gray-400 dark:text-gray-500">
                    No playlists assigned. Use the form above to add one.
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
        on:click={() => { master.refresh(); status.refresh(); }}
    >
        🔄 Refresh
    </button>
</div>
