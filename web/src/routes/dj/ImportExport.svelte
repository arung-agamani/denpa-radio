<script lang="ts">
    import { exportPlaylist, importPlaylist } from "../../lib/api";
    import { playlists, master, toasts } from "../../lib/stores";
    import { tagEmoji } from "../../lib/tags";

    // ---------------------------------------------------------------------------
    // Import state
    // ---------------------------------------------------------------------------

    let importText = "";
    let importing = false;

    async function handleExport(id: number): Promise<void> {
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
            toasts.error("Export failed: " + (err instanceof Error ? err.message : String(err)));
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
            toasts.error("Import failed: " + (err instanceof Error ? err.message : String(err)));
        } finally {
            importing = false;
        }
    }

    async function handleImportFile(e: Event): Promise<void> {
        const file = (e.target as HTMLInputElement).files?.[0];
        if (!file) return;
        const text = await file.text();
        importText = text;
    }

    // ---------------------------------------------------------------------------
    // Derived
    // ---------------------------------------------------------------------------

    $: allPlaylistsList = $playlists || [];
</script>

<h1 class="text-2xl font-bold text-gray-900 dark:text-white">Import / Export</h1>

<!-- Export section -->
<div class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-5">
    <h3 class="text-base font-semibold text-gray-900 dark:text-white mb-4 flex items-center gap-2">
        <span>📤</span> Export Playlist
    </h3>
    <p class="text-sm text-gray-500 dark:text-gray-400 mb-4">
        Select a playlist to download as a JSON file for backup or sharing.
    </p>

    {#if allPlaylistsList.length === 0}
        <p class="text-sm text-gray-400 dark:text-gray-500">No playlists available to export.</p>
    {:else}
        <div class="grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
            {#each allPlaylistsList as pl (pl.id)}
                <button
                    type="button"
                    class="flex items-center gap-3 p-3 rounded-lg border border-gray-200 dark:border-gray-700 hover:border-primary-300 dark:hover:border-primary-700 hover:bg-primary-50 dark:hover:bg-primary-900/20 transition-colors text-left"
                    on:click={() => handleExport(pl.id)}
                >
                    <span class="text-lg">{tagEmoji[pl.tag] || "🎵"}</span>
                    <div class="flex-1 min-w-0">
                        <p class="text-sm font-medium text-gray-900 dark:text-white truncate">{pl.name}</p>
                        <p class="text-xs text-gray-500 dark:text-gray-400">{pl.trackCount} tracks</p>
                    </div>
                    <svg class="w-4 h-4 text-gray-400 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
                        <path stroke-linecap="round" stroke-linejoin="round" d="M3 16.5v2.25A2.25 2.25 0 0 0 5.25 21h13.5A2.25 2.25 0 0 0 21 18.75V16.5M16.5 12 12 16.5m0 0L7.5 12m4.5 4.5V3" />
                    </svg>
                </button>
            {/each}
        </div>
    {/if}
</div>

<!-- Import section -->
<div class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-5">
    <h3 class="text-base font-semibold text-gray-900 dark:text-white mb-4 flex items-center gap-2">
        <span>📥</span> Import Playlist
    </h3>
    <p class="text-sm text-gray-500 dark:text-gray-400 mb-4">
        Upload a previously exported playlist JSON file, or paste the JSON directly.
    </p>

    <!-- File upload -->
    <div class="mb-4">
        <label
            class="flex flex-col items-center justify-center w-full h-32 rounded-xl border-2 border-dashed border-gray-300 dark:border-gray-600 hover:border-primary-400 dark:hover:border-primary-600 bg-gray-50 dark:bg-gray-700/30 cursor-pointer transition-colors"
        >
            <div class="flex flex-col items-center text-gray-500 dark:text-gray-400">
                <svg class="w-8 h-8 mb-2" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M3 16.5v2.25A2.25 2.25 0 0 0 5.25 21h13.5A2.25 2.25 0 0 0 21 18.75V16.5m-13.5-9L12 3m0 0 4.5 4.5M12 3v13.5" />
                </svg>
                <p class="text-sm font-medium">Click to upload a JSON file</p>
                <p class="text-xs text-gray-400 dark:text-gray-500">or drag and drop</p>
            </div>
            <input type="file" accept=".json" class="hidden" on:change={handleImportFile} />
        </label>
    </div>

    <!-- JSON textarea -->
    <div class="mb-4">
        <label for="import-json" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1.5">
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
            <svg class="animate-spin w-4 h-4" fill="none" viewBox="0 0 24 24">
                <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/>
                <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/>
            </svg>
            Importing…
        {:else}
            📥 Import Playlist
        {/if}
    </button>
</div>
