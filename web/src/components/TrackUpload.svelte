<script lang="ts">
    import { createEventDispatcher } from "svelte";
    import { uploadTrack } from "../lib/api";
    import type { Track } from "../lib/api";

    const dispatch = createEventDispatcher();

    // --------------------------------------------------------------------------
    // Constants
    // --------------------------------------------------------------------------

    const ACCEPTED = [".mp3", ".wav", ".flac", ".aac", ".ogg"];
    const ACCEPT_ATTR = ACCEPTED.join(",");
    const MAX_BYTES = 100 * 1024 * 1024; // 100 MiB

    // --------------------------------------------------------------------------
    // Types
    // --------------------------------------------------------------------------

    type EntryStatus = "queued" | "uploading" | "done" | "error" | "duplicate";

    interface UploadMeta {
        title: string;
        artist: string;
        album: string;
        genre: string;
    }

    interface QueueEntry {
        id: number;
        file: File;
        status: EntryStatus;
        progress: number;
        track?: Track;
        error?: string;
        meta: UploadMeta;
        optimize: boolean;
    }

    // --------------------------------------------------------------------------
    // State
    // --------------------------------------------------------------------------

    let queue: QueueEntry[] = [];
    let idCounter = 0;
    let isDragging = false;
    let fileInput: HTMLInputElement | undefined;

    // Global default for optimize toggle
    let globalOptimize = true;

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

    function formatBytes(bytes: number): string {
        if (bytes < 1024) return `${bytes} B`;
        if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
        return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
    }

    function isAudioFile(file: File): boolean {
        const ext = "." + file.name.split(".").pop()!.toLowerCase();
        return ACCEPTED.includes(ext);
    }

    function getFileExt(name: string): string {
        return name.slice(name.lastIndexOf('.')).toLowerCase();
    }

    /** Compute the resulting filename that will be stored on disk */
    function resultingFilename(entry: QueueEntry): string {
        const ext = getFileExt(entry.file.name);
        const base = entry.meta.title.trim()
            ? entry.meta.title.trim()
            : entry.file.name.slice(0, entry.file.name.lastIndexOf('.'));
        const isAlreadyOgg = ext === '.ogg';
        const outExt = (entry.optimize && !isAlreadyOgg) ? '.ogg' : ext;
        return base + outExt;
    }

    function addFiles(files: File[]): void {
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
                        optimize: globalOptimize,
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
                    optimize: globalOptimize,
                },
            ];
        }
    }

    // --------------------------------------------------------------------------
    // Upload logic
    // --------------------------------------------------------------------------

    async function startUpload(): Promise<void> {
        const pending = queue.filter((e) => e.status === "queued");
        for (const entry of pending) {
            setStatus(entry.id, "uploading");
            try {
                const result = await uploadTrack(entry.file, {
                    onProgress: (pct) => setProgress(entry.id, pct),
                    meta: entry.meta,
                    optimize: entry.optimize,
                });
                if (result.added) {
                    setStatus(entry.id, "done", { track: result.track });
                    dispatch("uploaded", { track: result.track });
                } else {
                    setStatus(entry.id, "duplicate", { track: result.track });
                }
            } catch (err) {
                setStatus(entry.id, "error", { error: err instanceof Error ? err instanceof Error ? err.message : String(err) : String(err) });
            }
        }
    }

    function setStatus(id: number, status: EntryStatus, extra: Partial<QueueEntry> = {}): void {
        queue = queue.map((e) =>
            e.id === id ? { ...e, status, progress: status === "done" || status === "duplicate" ? 100 : e.progress, ...extra } : e,
        );
    }

    function setProgress(id: number, progress: number): void {
        queue = queue.map((e) => (e.id === id ? { ...e, progress } : e));
    }

    function removeEntry(id: number): void {
        queue = queue.filter((e) => e.id !== id);
    }

    function updateMeta(id: number, field: keyof UploadMeta, value: string): void {
        queue = queue.map((e) => e.id === id ? { ...e, meta: { ...e.meta, [field]: value } } : e);
    }

    function toggleOptimize(id: number): void {
        queue = queue.map((e) => e.id === id ? { ...e, optimize: !e.optimize } : e);
    }

    function clearCompleted(): void {
        queue = queue.filter((e) => !["done", "error", "duplicate"].includes(e.status));
    }

    // --------------------------------------------------------------------------
    // Drag & drop (file input)
    // --------------------------------------------------------------------------

    function onDragOver(e: DragEvent): void {
        e.preventDefault();
        isDragging = true;
    }

    function onDragLeave(e: DragEvent): void {
        if (!(e.currentTarget as HTMLElement).contains(e.relatedTarget as Node)) {
            isDragging = false;
        }
    }

    function onDrop(e: DragEvent): void {
        e.preventDefault();
        isDragging = false;
        if (e.dataTransfer?.files) {
            addFiles(Array.from(e.dataTransfer.files));
        }
    }

    function onFileInput(e: Event): void {
        const input = e.target as HTMLInputElement;
        addFiles(Array.from(input.files ?? []));
        input.value = "";
    }

    // --------------------------------------------------------------------------
    // Status helpers
    // --------------------------------------------------------------------------

    const statusLabel: Record<EntryStatus, string> = {
        queued: "Queued",
        uploading: "Uploading…",
        done: "Uploaded",
        duplicate: "Already exists",
        error: "Failed",
    };

    const statusIconCls: Record<EntryStatus, string> = {
        queued:   "text-gray-400",
        uploading:"text-primary-500",
        done:     "text-green-500",
        duplicate:"text-amber-500",
        error:    "text-red-500",
    };

    const rowBg: Record<EntryStatus, string> = {
        queued:    "bg-white dark:bg-gray-800",
        uploading: "bg-primary-50 dark:bg-primary-900/10",
        done:      "bg-green-50 dark:bg-green-900/10",
        duplicate: "bg-amber-50 dark:bg-amber-900/10",
        error:     "bg-red-50 dark:bg-red-900/10",
    };
</script>

<div class="space-y-4">
    <!-- Global optimize toggle -->
    <div class="flex items-center gap-3 px-4 py-3 rounded-xl border border-blue-200 dark:border-blue-800 bg-blue-50 dark:bg-blue-900/20">
        <div class="flex-1">
            <div class="flex items-center gap-2">
                <svg class="w-4 h-4 text-blue-600 dark:text-blue-400" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M9.594 3.94c.09-.542.56-.94 1.11-.94h2.593c.55 0 1.02.398 1.11.94l.213 1.281c.063.374.313.686.645.87.074.04.147.083.22.127.325.196.72.257 1.075.124l1.217-.456a1.125 1.125 0 0 1 1.37.49l1.296 2.247a1.125 1.125 0 0 1-.26 1.431l-1.003.827c-.293.241-.438.613-.43.992a7.723 7.723 0 0 1 0 .255c-.008.378.137.75.43.991l1.004.827c.424.35.534.955.26 1.43l-1.298 2.247a1.125 1.125 0 0 1-1.369.491l-1.217-.456c-.355-.133-.75-.072-1.076.124a6.47 6.47 0 0 1-.22.128c-.331.183-.581.495-.644.869l-.213 1.281c-.09.543-.56.94-1.11.94h-2.594c-.55 0-1.019-.398-1.11-.94l-.213-1.281c-.062-.374-.312-.686-.644-.87a6.52 6.52 0 0 1-.22-.127c-.325-.196-.72-.257-1.076-.124l-1.217.456a1.125 1.125 0 0 1-1.369-.49l-1.297-2.247a1.125 1.125 0 0 1 .26-1.431l1.004-.827c.292-.24.437-.613.43-.991a6.932 6.932 0 0 1 0-.255c.007-.38-.138-.751-.43-.992l-1.004-.827a1.125 1.125 0 0 1-.26-1.43l1.297-2.247a1.125 1.125 0 0 1 1.37-.491l1.216.456c.356.133.751.072 1.076-.124.072-.044.146-.086.22-.128.332-.183.582-.495.644-.869l.214-1.28Z" />
                    <path stroke-linecap="round" stroke-linejoin="round" d="M15 12a3 3 0 1 1-6 0 3 3 0 0 1 6 0Z" />
                </svg>
                <span class="text-sm font-semibold text-blue-800 dark:text-blue-300">OGG Optimization</span>
            </div>
            <p class="mt-0.5 text-xs text-blue-600 dark:text-blue-400">
                Convert uploaded files to OGG Vorbis for smaller file sizes and standardized format.
                Files already in OGG format will be kept as-is.
            </p>
        </div>
        <label class="relative inline-flex items-center cursor-pointer flex-shrink-0">
            <input type="checkbox" class="sr-only peer" bind:checked={globalOptimize}
                on:change={() => {
                    queue = queue.map(e => e.status === 'queued' ? { ...e, optimize: globalOptimize } : e);
                }}
            />
            <div class="w-11 h-6 bg-gray-300 dark:bg-gray-600 peer-focus:ring-2 peer-focus:ring-blue-300 dark:peer-focus:ring-blue-800 rounded-full peer peer-checked:after:translate-x-full rtl:peer-checked:after:-translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:start-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all dark:border-gray-600 peer-checked:bg-blue-600"></div>
        </label>
    </div>

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
        on:click={() => fileInput?.click()}
        on:keydown={(e) => e.key === "Enter" && fileInput?.click()}
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
                                {entry.file.name}
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

                        <!-- Remove button -->
                        {#if entry.status !== "uploading"}
                            <div class="flex-shrink-0">
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

                        <!-- Info & metadata section (shown for queued items by default) -->
                        {#if entry.status === "queued"}
                            <div class="px-4 pb-3 pt-1 border-t border-gray-100 dark:border-gray-700/60 mt-0 space-y-3">
                                <!-- Output info banner -->
                                <div class="flex items-start gap-2 p-2.5 rounded-lg bg-gray-50 dark:bg-gray-700/50">
                                    <svg class="w-4 h-4 text-gray-500 dark:text-gray-400 mt-0.5 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
                                        <path stroke-linecap="round" stroke-linejoin="round" d="m11.25 11.25.041-.02a.75.75 0 0 1 1.063.852l-.708 2.836a.75.75 0 0 0 1.063.853l.041-.021M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Zm-9-3.75h.008v.008H12V8.25Z" />
                                    </svg>
                                    <div class="text-xs text-gray-600 dark:text-gray-400 space-y-1">
                                        <div class="flex items-center gap-2">
                                            <span class="font-medium text-gray-700 dark:text-gray-300">Output file:</span>
                                            <code class="px-1.5 py-0.5 rounded bg-gray-200 dark:bg-gray-600 text-gray-800 dark:text-gray-200 font-mono text-xs">
                                                {resultingFilename(entry)}
                                            </code>
                                        </div>
                                        {#if entry.optimize && getFileExt(entry.file.name) !== '.ogg'}
                                            <div class="flex items-center gap-1.5 text-blue-600 dark:text-blue-400">
                                                <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
                                                    <path stroke-linecap="round" stroke-linejoin="round" d="M16.023 9.348h4.992v-.001M2.985 19.644v-4.992m0 0h4.992m-4.993 0 3.181 3.183a8.25 8.25 0 0 0 13.803-3.7M4.031 9.865a8.25 8.25 0 0 1 13.803-3.7l3.181 3.182" />
                                                </svg>
                                                <span>Will be converted from <strong>{getFileExt(entry.file.name).replace('.','').toUpperCase()}</strong> to <strong>OGG Vorbis</strong></span>
                                            </div>
                                        {:else if getFileExt(entry.file.name) === '.ogg'}
                                            <div class="flex items-center gap-1.5 text-green-600 dark:text-green-400">
                                                <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
                                                    <path stroke-linecap="round" stroke-linejoin="round" d="M4.5 12.75l6 6 9-13.5" />
                                                </svg>
                                                <span>Already in OGG format — no conversion needed</span>
                                            </div>
                                        {:else}
                                            <div class="flex items-center gap-1.5 text-gray-500 dark:text-gray-400">
                                                <span>Will be uploaded as-is ({getFileExt(entry.file.name).replace('.','').toUpperCase()})</span>
                                            </div>
                                        {/if}
                                    </div>
                                </div>

                                <!-- Per-file optimize toggle -->
                                <div class="flex items-center gap-2">
                                    <label class="relative inline-flex items-center cursor-pointer">
                                        <input
                                            type="checkbox"
                                            class="sr-only peer"
                                            checked={entry.optimize}
                                            on:change={() => toggleOptimize(entry.id)}
                                        />
                                        <div class="w-9 h-5 bg-gray-300 dark:bg-gray-600 peer-focus:ring-2 peer-focus:ring-blue-300 dark:peer-focus:ring-blue-800 rounded-full peer peer-checked:after:translate-x-full rtl:peer-checked:after:-translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:start-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-4 after:w-4 after:transition-all dark:border-gray-600 peer-checked:bg-blue-600"></div>
                                    </label>
                                    <span class="text-xs text-gray-600 dark:text-gray-400">Convert to OGG</span>
                                </div>

                                <!-- Metadata fields (always visible) -->
                                <div class="grid grid-cols-2 gap-2">
                                    <div>
                                        <label for="meta-title-{entry.id}" class="block text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">
                                            Title <span class="font-normal opacity-60">(becomes filename)</span>
                                        </label>
                                        <input
                                            id="meta-title-{entry.id}"
                                            type="text"
                                            value={entry.meta.title}
                                            on:input={(e) => updateMeta(entry.id, 'title', e.currentTarget.value)}
                                            placeholder="Auto-detected from file"
                                            class="w-full px-2.5 py-1.5 text-xs rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-white placeholder-gray-400 focus:ring-1 focus:ring-primary-500 focus:border-primary-500"
                                        />
                                    </div>
                                    <div>
                                        <label for="meta-artist-{entry.id}" class="block text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">Artist</label>
                                        <input
                                            id="meta-artist-{entry.id}"
                                            type="text"
                                            value={entry.meta.artist}
                                            on:input={(e) => updateMeta(entry.id, 'artist', e.currentTarget.value)}
                                            placeholder="Auto-detected from file"
                                            class="w-full px-2.5 py-1.5 text-xs rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-white placeholder-gray-400 focus:ring-1 focus:ring-primary-500 focus:border-primary-500"
                                        />
                                    </div>
                                    <div>
                                        <label for="meta-album-{entry.id}" class="block text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">Album</label>
                                        <input
                                            id="meta-album-{entry.id}"
                                            type="text"
                                            value={entry.meta.album}
                                            on:input={(e) => updateMeta(entry.id, 'album', e.currentTarget.value)}
                                            placeholder="Auto-detected from file"
                                            class="w-full px-2.5 py-1.5 text-xs rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-white placeholder-gray-400 focus:ring-1 focus:ring-primary-500 focus:border-primary-500"
                                        />
                                    </div>
                                    <div>
                                        <label for="meta-genre-{entry.id}" class="block text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">Genre</label>
                                        <input
                                            id="meta-genre-{entry.id}"
                                            type="text"
                                            value={entry.meta.genre}
                                            on:input={(e) => updateMeta(entry.id, 'genre', e.currentTarget.value)}
                                            placeholder="Auto-detected from file"
                                            class="w-full px-2.5 py-1.5 text-xs rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-white placeholder-gray-400 focus:ring-1 focus:ring-primary-500 focus:border-primary-500"
                                        />
                                    </div>
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
