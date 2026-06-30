<script lang="ts">
  import '../app.css';
  import { onMount } from 'svelte';
  import { auth, isAuthenticated } from '$lib/auth';
  import { status, toasts } from '$lib/stores';
  import Navbar from '$lib/components/Navbar.svelte';
  import Toast from '$lib/components/Toast.svelte';

  let { children } = $props();

  onMount(() => {
    // Start polling radio status
    status.startPolling(5000);

    // Verify existing token if present
    if ($isAuthenticated) {
      auth.verify();
    }

    return () => {
      status.stopPolling();
    };
  });
</script>

<div class="min-h-screen flex flex-col bg-gray-50 dark:bg-gray-900">
  <Navbar />

  <main class="flex-1">
    {@render children()}
  </main>

  <footer class="py-4 text-center text-sm text-gray-500 dark:text-gray-400 border-t border-gray-200 dark:border-gray-700">
    <p>Denpa Radio &mdash; Powered by love and denpa waves 📻</p>
  </footer>
</div>

<!-- Toast notifications -->
<div class="fixed bottom-4 right-4 z-50 flex flex-col gap-2 max-w-sm w-full pointer-events-none">
  {#each $toasts as toast (toast.id)}
    <Toast {toast} ondismiss={() => toasts.remove(toast.id)} />
  {/each}
</div>
