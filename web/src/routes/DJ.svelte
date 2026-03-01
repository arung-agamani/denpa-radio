<script>
    import { onMount } from "svelte";
    import { segments, navigate } from "../lib/router.js";
    import { playlists, master, scheduler, status } from "../lib/stores.js";
    import Dashboard from "./dj/Dashboard.svelte";
    import Playlists from "./dj/Playlists.svelte";
    import Master from "./dj/Master.svelte";
    import Tracks from "./dj/Tracks.svelte";
    import ImportExport from "./dj/ImportExport.svelte";

    const sections = [
        { id: "dashboard", label: "Dashboard", icon: "📊" },
        { id: "playlists", label: "Playlists", icon: "🎵" },
        { id: "master", label: "Master Playlist", icon: "🕐" },
        { id: "tracks", label: "Track Library", icon: "📚" },
        { id: "importexport", label: "Import / Export", icon: "📦" },
    ];

    let sidebarOpen = false;

    // Derive the active section from the URL: /dj/playlists → "playlists"
    $: activeSection = $segments[1] && sections.some((s) => s.id === $segments[1])
        ? $segments[1]
        : "dashboard";

    function selectSection(id) {
        navigate("/dj/" + id);
        sidebarOpen = false;
    }

    onMount(() => {
        playlists.refresh();
        master.refresh();
        scheduler.refresh();
    });

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
            {activeSection === section.id
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
                <span>
                    {statusData.active_clients || 0} listener{(statusData.active_clients || 0) !== 1 ? "s" : ""} connected
                </span>
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
        <div class="w-full px-4 sm:px-6 py-6 space-y-6">
            {#if activeSection === "dashboard"}
                <Dashboard />
            {:else if activeSection === "playlists"}
                <Playlists />
            {:else if activeSection === "master"}
                <Master />
            {:else if activeSection === "tracks"}
                <Tracks />
            {:else if activeSection === "importexport"}
                <ImportExport />
            {/if}
        </div>
    </div>
</div>
