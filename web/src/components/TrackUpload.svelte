<script>
    import { createEventDispatcher } from "svelte";
    import { uploadTrack } from "../lib/api.js";

    const dispatch = createEventDispatcher();

    // --------------------------------------------------------------------------
    // Constants
    // --------------------------------------------------------------------------

    const ACCEPTED = [".mp3", ".wav", ".flac", ".aac", ".ogg"];
    const ACCEPT_ATTR = ACCEPTED.join(",");
    const MAX_BYTES = 100 * 1024 * 1024; // 100 MiB

    // --------------------------------------------------------------------------
    // State
    // --------------------------------------------------------------------------

    /** @type {Array<{id: number, file: File, status: 'queued'|'uploading'|'done'|'error'|'duplicate', progress: number, track?: object, error?: string, meta: {title:string,artist:string,album:string,genre:string}, expanded: boolean}>} */
    let queue = [];
    let idCounter = 0;
    let isDragging = false;
    let fileInput;

    // --------------------------------------------------------------------------
    // Derived
    // --------------------------------------------------------------------------

    $: uploading = queue.some((e) => e.status === "uploading");
    $: hasItems = queue.length > 0;
    $: allDone = hasItems && queue.every((e) => ["done", "error", "duplicate"].includes(e.status));
    $: successCount = queue.filter((e) => e.status === "done").length;
    $: duplicateCount = queue.filter((e) => e.status === "duplicate").length;
    $: errorCount = queue.filter((e) => e.status === "error").length;

    // --------------------------------------------------------------------------
    // Helpers
    // --------------------------------------------------------------------------

    function formatBytes(bytes) {
        if (bytes < 1024) return `${bytes} B`;
        if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
        return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
    }

    function isAudioFile(file) {
        const ext = "." + file.name.split(".").pop().toLowerCase();
        return ACCEPTED.includes(ext);
    }

    function addFiles(files) {
        for (const file of files) {
            if (!isAudioFile(file)) continue;
            if (file.size > MAX_BYTES) {
                queue = [
                    ...queue,
                    {
                        id: ++idCounter,
                        file,
                        status: "error",
                        progress: 0,
                        error: `File exceeds 100 MB limit (${formatBytes(file.size)})`,
                        meta: { title: "", artist: "", album: "", genre: "" },
                        expanded: false,
                    },
                ];
                continue;
            }
            queue = [
                ...queue,
                {
                    id: ++idCounter,
                    file,
                    status: "queued",
                    progress: 0,
                    meta: { title: "", artist: "", album: "", genre: "" },
                    expanded: false,
                },
            ];
        }
    }

    // --------------------------------------------------------------------------
    // Upload logic
    // --------------------------------------------------------------------------

    async function startUpload() {
        const pending = queue.filter((e) => e.status === "queued");
        for (const entry of pending) {
            setStatus(entry.id, "uploading");
            try {
                const result = await uploadTrack(entry.file, {
                    onProgress: (pct) => setProgress(entry.id, pct),
                    meta: entry.meta,
                });
                if (result.added) {
                    setStatus(entry.id, "done", { track: result.track });
                    dispatch("uploaded", { track: result.track });
                } else {
                    setStatus(entry.id, "duplicate", { track: result.track });
                }
            } catch (err) {
                setStatus(entry.id, "error", { error: err.message });
            }
        }
    }

    function setStatus(id, status, extra = {}) {
        queue = queue.map((e) =>
            e.id === id ? { ...e, status, progress: status === "done" || status === "duplicate" ? 100 : e.progress, ...extra } : e,
        );
    }

    function setProgress(id, progress) {
        queue = queue.map((e) => (e.id === id ? { ...e, progress } : e));
    }

    function removeEntry(id) {
        queue = queue.filter((e) => e.id !== id);
    }

    function toggleMeta(id) {
        queue = queue.map((e) => e.id === id ? { ...e, expanded: !e.expanded } : e);
    }

    function updateMeta(id, field, value) {
        queue = queue.map((e) => e.id === id ? { ...e, meta: { ...e.meta, [field]: value } } : e);
    }

    function clearCompleted() {
        queue = queue.filter((e) => !["done", "error", "duplicate"].includes(e.status));
    }

    // --------------------------------------------------------------------------
    // Drag & drop
    // --------------------------------------------------------------------------

    function onDragOver(e) {
        e.preventDefault();
        isDragging = true;
    }

    function onDragLeave(e) {
        // Only clear when truly leaving the drop zone (not entering a child).
        if (!e.currentTarget.contains(e.relatedTarget)) {
            isDragging = false;
        }
    }

    function onDrop(e) {
        e.preventDefault();
        isDragging = false;
        if (e.dataTransfer?.files) {
            addFiles(Array.from(e.dataTransfer.files));
        }
    }

    function onFileInput(e) {
        addFiles(Array.from(e.target.files));
        // Reset so on:change fires again even if the same file is picked.
        e.target.value = "";
    }

    // --------------------------------------------------------------------------
    // Status helpers
    // --------------------------------------------------------------------------

    const statusLabel = {
        queued: "Queued",
        uploading: "Uploading…",
        done: "Uploaded",
        duplicate: "Already exists",
        error: "Failed",
    };

    const statusIconCls = {
        queued:   "text-gray-400",
        uploading:"text-primary-500",
        done:     "text-green-500",
        duplicate:"text-amber-500",
        error:    "text-red-500",
    };

    const rowBg = {
        queued:    "bg-white dark:bg-gray-800",
        uploading: "bg-primary-50 dark:bg-primary-900/10",
        done:      "bg-green-50 dark:bg-green-900/10",
        duplicate: "bg-amber-50 dark:bg-amber-900/10",
        error:     "bg-red-50 dark:bg-red-900/10",
    };
</script>

<div class="space-y-4">
    <!-- Drop zone -->
    <div
        role="button"
        aria-label="Upload audio files — click to browse or drag and drop"
        class="relative flex flex-col items-center justify-center w-full min-h-36 rounded-xl border-2 border-dashed transition-all duration-200 cursor-pointer select-none
            {isDragging
            ? 'border-primary-500 bg-primary-50 dark:bg-primary-900/20'
            : 'border-gray-300 dark:border-gray-600 bg-gray-50 dark:bg-gray-700/30 hover:border-primary-400 dark:hover:border-primary-500 hover:bg-primary-50/50 dark:hover:bg-primary-900/10'}"
        on:dragover={onDragOver}
        on:dragleave={onDragLeave}
        on:drop={onDrop}
        on:click={() => fileInput.click()}
        on:keydown={(e) => e.key === "Enter" && fileInput.click()}
        tabindex="0"
    >
        <input
            bind:this={fileInput}
            type="file"
            accept={ACCEPT_ATTR}
            multiple
            class="hidden"
            on:change={onFileInput}
        />

        {#if isDragging}
            <svg class="w-10 h-10 mb-2 text-primary-500 animate-bounce" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" d="M3 16.5v2.25A2.25 2.25 0 0 0 5.25 21h13.5A2.25 2.25 0 0 0 21 18.75V16.5m-13.5-9L12 3m0 0 4.5 4.5M12 3v13.5" />
            </svg>
            <p class="text-sm font-semibold text-primary-600 dark:text-primary-400">Drop to add files</p>
        {:else}
            <svg class="w-10 h-10 mb-2 text-gray-400 dark:text-gray-500" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" d="M3 16.5v2.25A2.25 2.25 0 0 0 5.25 21h13.5A2.25 2.25 0 0 0 21 18.75V16.5m-13.5-9L12 3m0 0 4.5 4.5M12 3v13.5" />
            </svg>
            <p class="text-sm font-medium text-gray-600 dark:text-gray-400">
                <span class="text-primary-600 dark:text-primary-400 font-semibold">Click to browse</span>
                &nbsp;or drag &amp; drop audio files
            </p>
            <p class="mt-1 text-xs text-gray-400 dark:text-gray-500">
                {ACCEPTED.join(", ")} &nbsp;·&nbsp; max 100 MB per file
            </p>
        {/if}
    </div>

    <!-- Queue -->
    {#if hasItems}
        <div class="rounded-xl border border-gray-200 dark:border-gray-700 overflow-hidden">
            <!-- Header -->
            <div class="flex items-center justify-between px-4 py-3 bg-gray-50 dark:bg-gray-800 border-b border-gray-200 dark:border-gray-700">
                <span class="text-sm font-semibold text-gray-700 dark:text-gray-300">
                    Upload queue &mdash; {queue.length} file{queue.length !== 1 ? "s" : ""}
                </span>
                {#if allDone}
                    <div class="flex items-center gap-3 text-xs">
                        {#if successCount > 0}
                            <span class="text-green-600 dark:text-green-400 font-medium">✓ {successCount} added</span>
                        {/if}
                        {#if duplicateCount > 0}
                            <span class="text-amber-600 dark:text-amber-400 font-medium">⊜ {duplicateCount} duplicate{duplicateCount !== 1 ? "s" : ""}</span>
                        {/if}
                        {#if errorCount > 0}
                            <span class="text-red-500 dark:text-red-400 font-medium">✕ {errorCount} failed</span>
                        {/if}
                        <button
                            type="button"
                            class="text-gray-400 hover:text-gray-700 dark:hover:text-gray-200 transition-colors"
                            on:click={clearCompleted}
                        >
                            Clear all
                        </button>
                    </div>
                {/if}
            </div>

            <!-- Rows -->
            <ul class="divide-y divide-gray-100 dark:divide-gray-700/60">
                {#each queue as entry (entry.id)}
                    <li class="{rowBg[entry.status]} transition-colors">
                        <!-- Main row -->
                        <div class="flex items-center gap-3 px-4 py-3">
                        <!-- Status icon -->
                        <div class="flex-shrink-0 w-8 h-8 rounded-full flex items-center justify-center {statusIconCls[entry.status]}">
                            {#if entry.status === "queued"}
                                <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
                                    <path stroke-linecap="round" stroke-linejoin="round" d="M9 9l10.5-3m0 6.553v3.75a2.25 2.25 0 0 1-1.632 2.163l-1.32.377a1.803 1.803 0 1 1-.99-3.467l2.31-.66a2.25 2.25 0 0 0 1.632-2.163Zm0 0V2.25L9 5.25v10.303m0 0v3.75a2.25 2.25 0 0 1-1.632 2.163l-1.32.377a1.803 1.803 0 0 1-.99-3.467l2.31-.66A2.25 2.25 0 0 0 9 15.553Z" />
                                </svg>
                            {:else if entry.status === "uploading"}
                                <svg class="animate-spin w-4 h-4" fill="none" viewBox="0 0 24 24">
                                    <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
                                    <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
                                </svg>
                            {:else if entry.status === "done"}
                                <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke-width="2.5" stroke="currentColor">
                                    <path stroke-linecap="round" stroke-linejoin="round" d="M4.5 12.75l6 6 9-13.5" />
                                </svg>
                            {:else if entry.status === "duplicate"}
                                <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
                                    <path stroke-linecap="round" stroke-linejoin="round" d="M8.25 7.5V6.108c0-1.135.845-2.098 1.976-2.192.373-.03.748-.057 1.123-.08M15.75 18H18a2.25 2.25 0 0 0 2.25-2.25V6.108c0-1.135-.845-2.098-1.976-2.192a48.424 48.424 0 0 0-1.123-.08M15.75 18.75v-1.875a3.375 3.375 0 0 0-3.375-3.375h-1.5a1.125 1.125 0 0 1-1.125-1.125v-1.5A3.375 3.375 0 0 0 6.375 7.5H5.25m11.9-3.664A2.251 2.251 0 0 0 15 2.25h-1.5a2.251 2.251 0 0 0-2.15 1.586m5.8 0c.065.21.1.433.1.664v.75h-6V4.5c0-.231.035-.454.1-.664M6.75 7.5H4.875c-.621 0-1.125.504-1.125 1.125v12c0 .621.504 1.125 1.125 1.125h9.75c.621 0 1.125-.504 1.125-1.125V16.5a9 9 0 0 0-9-9Z" />
                                </svg>
                            {:else}
                                <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
                                    <path stroke-linecap="round" stroke-linejoin="round" d="M12 9v3.75m9-.75a9 9 0 1 1-18 0 9 9 0 0 1 18 0Zm-9 3.75h.008v.008H12v-.008Z" />
                                </svg>
                            {/if}
                        </div>

                        <!-- File info -->
                        <div class="flex-1 min-w-0">
                            <p class="text-sm font-medium text-gray-800 dark:text-gray-200 truncate" title={entry.file.name}>
                                {#if entry.status === 'queued' && entry.meta.title.trim()}
                                    {entry.meta.title.trim()}{entry.file.name.slice(entry.file.name.lastIndexOf('.'))}
                                {:else}
                                    {entry.file.name}
                                {/if}
                            </p>
                            <div class="flex items-center gap-2 mt-0.5">
                                <span class="text-xs text-gray-400 dark:text-gray-500">{formatBytes(entry.file.size)}</span>
                                <span class="text-xs {statusIconCls[entry.status]}">
                                    {#if entry.status === "uploading"}
                                        {entry.progress}%
                                    {:else if entry.status === "done" && entry.track}
                                        {entry.track.title || entry.file.name}{entry.track.artist ? ` — ${entry.track.artist}` : ""}
                                    {:else if entry.status === "duplicate" && entry.track}
                                        Already in library (ID {entry.track.id})
                                    {:else if entry.status === "error"}
                                        {entry.error}
                                    {:else}
                                        {statusLabel[entry.status]}
                                    {/if}
                                </span>
                            </div>

                            <!-- Progress bar (uploading only) -->
                            {#if entry.status === "uploading"}
                                <div class="mt-1.5 w-full bg-gray-200 dark:bg-gray-700 rounded-full h-1.5 overflow-hidden">
                                    <div
                                        class="h-1.5 rounded-full bg-primary-500 transition-all duration-200"
                                        style="width: {entry.progress}%"
                                    ></div>
                                </div>
                            {/if}
                        </div>

                        <!-- Actions: metadata toggle (queued only) + remove -->
                        {#if entry.status !== "uploading"}
                            <div class="flex-shrink-0 flex items-center gap-1">
                                {#if entry.status === "queued"}
                                    <button
                                        type="button"
                                        class="p-1.5 rounded-lg transition-colors {entry.expanded ? 'text-primary-500 dark:text-primary-400 bg-primary-50 dark:bg-primary-900/20' : 'text-gray-400 hover:text-primary-500 dark:hover:text-primary-400 hover:bg-gray-100 dark:hover:bg-gray-700'}"
                                        title="{entry.expanded ? 'Hide' : 'Edit'} metadata"
                                        on:click|stopPropagation={() => toggleMeta(entry.id)}
                                    >
                                        <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
                                            <path stroke-linecap="round" stroke-linejoin="round" d="m16.862 4.487 1.687-1.688a1.875 1.875 0 1 1 2.652 2.652L10.582 16.07a4.5 4.5 0 0 1-1.897 1.13L6 18l.8-2.685a4.5 4.5 0 0 1 1.13-1.897l8.932-8.931Zm0 0L19.5 7.125" />
                                        </svg>
                                    </button>
                                {/if}
                                <button
                                    type="button"
                                    class="p-1.5 rounded-lg text-gray-300 dark:text-gray-600 hover:text-red-400 dark:hover:text-red-400 hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
                                    title="Remove from queue"
                                    on:click|stopPropagation={() => removeEntry(entry.id)}
                                >
                                    <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
                                        <path stroke-linecap="round" stroke-linejoin="round" d="M6 18 18 6M6 6l12 12" />
                                    </svg>
                                </button>
                            </div>
                        {/if}
                        </div><!-- end main row -->

                        <!-- Metadata form (queued + expanded) -->
                        {#if entry.status === "queued" && entry.expanded}
                            <div class="px-4 pb-3 pt-1 grid grid-cols-2 gap-2 border-t border-gray-100 dark:border-gray-700/60 mt-0">
                                <div>
                                    <label class="block text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">
                                        Title <span class="font-normal opacity-60">(becomes filename)</span>
                                    </label>
                                    <input
                                        type="text"
                                        value={entry.meta.title}
                                        on:input={(e) => updateMeta(entry.id, 'title', e.currentTarget.value)}
                                        placeholder="Auto-detected from file"
                                        class="w-full px-2.5 py-1.5 text-xs rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-white placeholder-gray-400 focus:ring-1 focus:ring-primary-500 focus:border-primary-500"
                                    />
                                </div>
                                <div>
                                    <label class="block text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">Artist</label>
                                    <input
                                        type="text"
                                        value={entry.meta.artist}
                                        on:input={(e) => updateMeta(entry.id, 'artist', e.currentTarget.value)}
                                        placeholder="Auto-detected from file"
                                        class="w-full px-2.5 py-1.5 text-xs rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-white placeholder-gray-400 focus:ring-1 focus:ring-primary-500 focus:border-primary-500"
                                    />
                                </div>
                                <div>
                                    <label class="block text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">Album</label>
                                    <input
                                        type="text"
                                        value={entry.meta.album}
                                        on:input={(e) => updateMeta(entry.id, 'album', e.currentTarget.value)}
                                        placeholder="Auto-detected from file"
                                        class="w-full px-2.5 py-1.5 text-xs rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-white placeholder-gray-400 focus:ring-1 focus:ring-primary-500 focus:border-primary-500"
                                    />
                                </div>
                                <div>
                                    <label class="block text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">Genre</label>
                                    <input
                                        type="text"
                                        value={entry.meta.genre}
                                        on:input={(e) => updateMeta(entry.id, 'genre', e.currentTarget.value)}
                                        placeholder="Auto-detected from file"
                                        class="w-full px-2.5 py-1.5 text-xs rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-white placeholder-gray-400 focus:ring-1 focus:ring-primary-500 focus:border-primary-500"
                                    />
                                </div>
                            </div>
                        {/if}
                    </li>
                {/each}
            </ul>
        </div>

        <!-- Action row -->
        <div class="flex items-center justify-between gap-3">
            {#if !allDone}
                <button
                    type="button"
                    disabled={uploading || queue.filter((e) => e.status === "queued").length === 0}
                    class="inline-flex items-center gap-2 px-5 py-2.5 text-sm font-semibold rounded-lg transition-colors
                        bg-primary-600 hover:bg-primary-700 disabled:bg-primary-300 dark:disabled:bg-primary-900 text-white disabled:cursor-not-allowed"
                    on:click={startUpload}
                >
                    {#if uploading}
                        <svg class="animate-spin w-4 h-4" fill="none" viewBox="0 0 24 24">
                            <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
                            <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
                        </svg>
                        Uploading…
                    {:else}
                        <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
                            <path stroke-linecap="round" stroke-linejoin="round" d="M3 16.5v2.25A2.25 2.25 0 0 0 5.25 21h13.5A2.25 2.25 0 0 0 21 18.75V16.5m-13.5-9L12 3m0 0 4.5 4.5M12 3v13.5" />
                        </svg>
                        Upload {queue.filter((e) => e.status === "queued").length} file{queue.filter((e) => e.status === "queued").length !== 1 ? "s" : ""}
                    {/if}
                </button>
            {:else}
                <p class="text-sm text-gray-500 dark:text-gray-400">
                    All uploads complete.
                    {#if successCount > 0}
                        <span class="text-green-600 dark:text-green-400 font-medium">{successCount} track{successCount !== 1 ? "s" : ""} added to library.</span>
                    {/if}
                </p>
            {/if}

            <button
                type="button"
                class="text-sm text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 transition-colors"
                on:click={clearCompleted}
            >
                Clear finished
            </button>
        </div>
    {/if}
</div>
