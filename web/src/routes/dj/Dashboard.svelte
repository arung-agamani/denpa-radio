<script lang="ts">
    import type { ReconcileResult } from "../../lib/api";
    import { status, playlists, master, scheduler, timezone } from "../../lib/stores";
    import { navigate } from "../../lib/router";
    import { reconcile, setTimezone, skipNext, skipPrev } from "../../lib/api";
    import { toasts } from "../../lib/stores";
    import NowPlaying from "../../components/NowPlaying.svelte";
    import { tagEmoji, tagLabel, tagColors } from "../../lib/tags";
    import { onMount } from "svelte";

    // ---------------------------------------------------------------------------
    // Skip controls
    // ---------------------------------------------------------------------------

    let skipping = false;

    async function handleSkipNext() {
        skipping = true;
        try {
            await skipNext();
            toasts.success("Skipped to next track.");
            status.refresh();
        } catch (err) {
            toasts.error("Skip failed: " + (err instanceof Error ? err.message : String(err)));
        } finally {
            skipping = false;
        }
    }

    async function handleSkipPrev() {
        skipping = true;
        try {
            await skipPrev();
            toasts.success("Jumped to previous track.");
            status.refresh();
        } catch (err) {
            toasts.error("Skip failed: " + (err instanceof Error ? err.message : String(err)));
        } finally {
            skipping = false;
        }
    }

    // ---------------------------------------------------------------------------
    // Reconcile
    // ---------------------------------------------------------------------------

    let reconciling = false;
    let reconcileResult: ReconcileResult | null = null;

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
        } catch (err) {
            toasts.error("Reconcile failed: " + (err instanceof Error ? err.message : String(err)));
        } finally {
            reconciling = false;
        }
    }

    // ---------------------------------------------------------------------------
    // Timezone
    // ---------------------------------------------------------------------------

    let timezoneInput = "";
    let savingTimezone = false;

    const commonTimezones = [
        "UTC",
        "US/Eastern",
        "US/Central",
        "US/Mountain",
        "US/Pacific",
        "US/Hawaii",
        "Canada/Eastern",
        "Canada/Central",
        "Canada/Pacific",
        "Europe/London",
        "Europe/Paris",
        "Europe/Berlin",
        "Europe/Moscow",
        "Asia/Tokyo",
        "Asia/Shanghai",
        "Asia/Hong_Kong",
        "Asia/Singapore",
        "Asia/Jakarta",
        "Asia/Kolkata",
        "Asia/Dubai",
        "Asia/Seoul",
        "Australia/Sydney",
        "Australia/Melbourne",
        "Australia/Perth",
        "Pacific/Auckland",
        "America/New_York",
        "America/Chicago",
        "America/Denver",
        "America/Los_Angeles",
        "America/Sao_Paulo",
        "America/Argentina/Buenos_Aires",
        "America/Mexico_City",
        "Africa/Cairo",
        "Africa/Johannesburg",
        "Africa/Lagos",
    ];

    onMount(() => {
        scheduler.refresh();
        timezoneInput = $timezone || "UTC";

        const unsubTz = timezone.subscribe((tz) => {
            if (!savingTimezone) {
                timezoneInput = tz || "UTC";
            }
        });

        return () => unsubTz();
    });

    $: if (!savingTimezone && $timezone) {
        timezoneInput = $timezone;
    }

    async function handleSetTimezone() {
        savingTimezone = true;
        try {
            const value =
                timezoneInput.trim() === "UTC" ? "" : timezoneInput.trim();
            const data = await setTimezone(value);
            toasts.success(`Timezone set to ${data.timezone}`);
            status.refresh();
            scheduler.refresh();
        } catch (err) {
            toasts.error(`Failed to set timezone(: ${err instanceof Error ? err.message : String(err)}`);
        } finally {
            savingTimezone = false;
        }
    }

    // ---------------------------------------------------------------------------
    // Derived
    // ---------------------------------------------------------------------------

    $: statusData = $status;
    $: schedulerData = $scheduler;
    $: masterData = $master;
    $: allPlaylistsList = $playlists || [];
</script>

<h1 class="text-2xl font-bold text-gray-900 dark:text-white">Dashboard</h1>

<!-- Now Playing -->
<NowPlaying />

<!-- Playback controls -->
<div class="flex items-center gap-3">
    <button
        type="button"
        class="inline-flex items-center gap-2 px-5 py-2.5 rounded-xl text-sm font-semibold border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-200 bg-white dark:bg-gray-800 hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors disabled:opacity-50 disabled:cursor-not-allowed shadow-sm"
        on:click={handleSkipPrev}
        disabled={skipping}
        title="Previous track"
    >
        <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" d="M21 16.811c0 .864-.933 1.406-1.683.977l-7.108-4.061a1.125 1.125 0 0 1 0-1.954l7.108-4.061A1.125 1.125 0 0 1 21 8.689v8.122ZM11.25 16.811c0 .864-.933 1.406-1.683.977l-7.108-4.061a1.125 1.125 0 0 1 0-1.954l7.108-4.061a1.125 1.125 0 0 1 1.683.977v8.122Z" />
        </svg>
        Prev
    </button>
    <button
        type="button"
        class="inline-flex items-center gap-2 px-5 py-2.5 rounded-xl text-sm font-semibold border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-200 bg-white dark:bg-gray-800 hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors disabled:opacity-50 disabled:cursor-not-allowed shadow-sm"
        on:click={handleSkipNext}
        disabled={skipping}
        title="Next track"
    >
        Next
        <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" d="M3 8.689c0-.864.933-1.406 1.683-.977l7.108 4.061a1.125 1.125 0 0 1 0 1.954l-7.108 4.061A1.125 1.125 0 0 1 3 16.811V8.69ZM12.75 8.689c0-.864.933-1.406 1.683-.977l7.108 4.061a1.125 1.125 0 0 1 0 1.954l-7.108 4.061a1.125 1.125 0 0 1-1.683-.977V8.69Z" />
        </svg>
    </button>
</div>

<!-- Stats grid -->
<div class="grid grid-cols-2 md:grid-cols-5 gap-4">
    <div class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-4">
        <p class="text-xs font-semibold uppercase tracking-wider text-gray-400 dark:text-gray-500 mb-1">
            📚 Library Tracks
        </p>
        <p class="text-2xl font-bold text-gray-900 dark:text-white">
            {statusData.library_tracks || 0}
        </p>
    </div>
    <div class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-4">
        <p class="text-xs font-semibold uppercase tracking-wider text-gray-400 dark:text-gray-500 mb-1">
            Playlist Tracks
        </p>
        <p class="text-2xl font-bold text-gray-900 dark:text-white">
            {statusData.total_tracks || 0}
        </p>
    </div>
    <div class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-4">
        <p class="text-xs font-semibold uppercase tracking-wider text-gray-400 dark:text-gray-500 mb-1">
            Playlists
        </p>
        <p class="text-2xl font-bold text-gray-900 dark:text-white">
            {allPlaylistsList.length}
        </p>
    </div>
    <div class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-4">
        <p class="text-xs font-semibold uppercase tracking-wider text-gray-400 dark:text-gray-500 mb-1">
            Listeners
        </p>
        <p class="text-2xl font-bold text-gray-900 dark:text-white">
            {statusData.active_clients || 0}
        </p>
    </div>
    <div class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-4">
        <p class="text-xs font-semibold uppercase tracking-wider text-gray-400 dark:text-gray-500 mb-1">
            Active Tag
        </p>
        <p class="text-2xl font-bold text-gray-900 dark:text-white flex items-center gap-2">
            <span>{tagEmoji[statusData.active_tag] || "🕐"}</span>
            <span class="text-base">{tagLabel[statusData.active_tag] || statusData.active_tag || "—"}</span>
        </p>
    </div>
</div>

<!-- Scheduler status -->
<div class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-5">
    <div class="flex items-center justify-between mb-4">
        <h3 class="text-base font-semibold text-gray-900 dark:text-white">⏰ Scheduler</h3>
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
            {@const count = (statusData.playlist_summary || {})[tag] || 0}
            {@const isActive = statusData.active_tag === tag}
            <div class="rounded-lg border p-3 {isActive ? 'ring-2 ring-primary-500 ' : ''}{tagColors[tag]}">
                <div class="flex items-center gap-2 mb-1">
                    <span class="text-lg">{tagEmoji[tag]}</span>
                    <span class="text-sm font-semibold">{tagLabel[tag]}</span>
                    {#if isActive}
                        <span class="ml-auto text-xs font-bold px-1.5 py-0.5 rounded-full bg-primary-200 dark:bg-primary-800 text-primary-700 dark:text-primary-300">ACTIVE</span>
                    {/if}
                </div>
                <p class="text-xs opacity-75">{count} playlist{count !== 1 ? "s" : ""}</p>
            </div>
        {/each}
    </div>
</div>

<!-- Timezone settings -->
<div class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-5">
    <div class="flex items-center justify-between mb-4">
        <h3 class="text-base font-semibold text-gray-900 dark:text-white">🌐 Timezone</h3>
        <span class="text-xs text-gray-400 dark:text-gray-500">
            Server time: {statusData.server_time
                ? new Date(statusData.server_time).toLocaleTimeString([], {
                      hour: "2-digit",
                      minute: "2-digit",
                      second: "2-digit",
                      hour12: false,
                  })
                : "—"}
        </span>
    </div>
    <p class="text-sm text-gray-500 dark:text-gray-400 mb-3">
        Set the timezone used for time-based playlist scheduling (morning/afternoon/evening/night).
    </p>
    <div class="flex flex-col sm:flex-row gap-3">
        <select
            bind:value={timezoneInput}
            class="flex-1 px-3 py-2 text-sm rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-primary-500 focus:border-primary-500"
        >
            {#each commonTimezones as tz}
                <option value={tz}>{tz}</option>
            {/each}
        </select>
        <div class="flex items-center gap-2">
            <span class="text-xs text-gray-400 dark:text-gray-500">or</span>
            <input
                type="text"
                bind:value={timezoneInput}
                placeholder="e.g. Asia/Tokyo"
                class="w-48 px-3 py-2 text-sm rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-white placeholder-gray-400 focus:ring-2 focus:ring-primary-500 focus:border-primary-500"
            />
        </div>
        <button
            type="button"
            class="px-4 py-2 text-sm font-medium text-white bg-primary-500 hover:bg-primary-600 rounded-lg transition-colors disabled:opacity-50 flex items-center gap-2"
            on:click={handleSetTimezone}
            disabled={savingTimezone}
        >
            {#if savingTimezone}
                <svg class="animate-spin w-4 h-4" fill="none" viewBox="0 0 24 24">
                    <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
                    <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
                </svg>
                Saving…
            {:else}
                Save
            {/if}
        </button>
    </div>
    <p class="text-xs text-gray-400 dark:text-gray-500 mt-2">
        Currently: <span class="font-semibold text-gray-600 dark:text-gray-300">{statusData.timezone || "UTC"}</span>
    </p>
</div>

<!-- Quick actions -->
<div class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-5">
    <h3 class="text-base font-semibold text-gray-900 dark:text-white mb-4">⚡ Quick Actions</h3>
    <div class="flex flex-wrap gap-3">
        <button
            type="button"
            class="px-4 py-2 text-sm font-medium rounded-lg border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors flex items-center gap-2"
            on:click={handleReconcile}
            disabled={reconciling}
        >
            {#if reconciling}
                <svg class="animate-spin w-4 h-4" fill="none" viewBox="0 0 24 24">
                    <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
                    <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
                </svg>
                Scanning…
            {:else}
                🔄 Reconcile Music Directory
            {/if}
        </button>
        <button
            type="button"
            class="px-4 py-2 text-sm font-medium rounded-lg border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
            on:click={() => navigate("/dj/playlists")}
        >
            ➕ Create Playlist
        </button>
        <button
            type="button"
            class="px-4 py-2 text-sm font-medium rounded-lg border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
            on:click={() => navigate("/dj/tracks")}
        >
            🔍 Check Orphaned Tracks
        </button>
    </div>

    {#if reconcileResult}
        <div class="mt-4 p-3 rounded-lg bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800 text-sm">
            <p class="text-green-800 dark:text-green-300">
                ✅ Reconcile complete: <strong>{reconcileResult.removed_count}</strong> track(s) removed,
                <strong>{reconcileResult.orphaned_count}</strong> new file(s) found.
                Total tracks: <strong>{reconcileResult.total_tracks}</strong>.
            </p>
        </div>
    {/if}
</div>
