<script lang="ts">
    import { login, ApiError } from "../lib/api";
    import { auth } from "../lib/auth";
    import { navigate } from "../lib/router";
    import { toasts } from "../lib/stores";

    let username = "";
    let password = "";
    let loading = false;
    let error: string | null = null;

    async function handleSubmit() {
        if (!username.trim() || !password.trim()) {
            error = "Please enter both username and password.";
            return;
        }

        loading = true;
        error = null;

        try {
            const data = await login(username.trim(), password);
            auth.setLoggedIn(data.username);
            toasts.success(`Welcome back, ${data.username}!`);
            navigate("/dj");
        } catch (err) {
            if (err instanceof ApiError && err.status === 429) {
                error =
                    "Too many login attempts. Please wait a few minutes before trying again.";
            } else if (err instanceof ApiError && err.status === 401) {
                error = "Invalid username or password.";
            } else {
                error = err instanceof Error ? err instanceof Error ? err.message : String(err) : "Login failed. Please try again.";
            }
        } finally {
            loading = false;
        }
    }

    function handleKeydown(e: KeyboardEvent) {
        if (e.key === "Enter") {
            handleSubmit();
        }
    }
</script>

<div
    class="min-h-[calc(100vh-10rem)] flex items-center justify-center px-4 sm:px-6 lg:px-8"
>
    <div class="w-full max-w-md">
        <!-- Card -->
        <div
            class="bg-white dark:bg-gray-800 rounded-2xl shadow-xl border border-gray-200 dark:border-gray-700 overflow-hidden"
        >
            <!-- Header gradient -->
            <div
                class="h-2 bg-gradient-to-r from-primary-400 via-purple-500 to-pink-500"
            ></div>

            <div class="p-8">
                <!-- Logo / title -->
                <div class="text-center mb-8">
                    <div
                        class="inline-flex items-center justify-center w-16 h-16 rounded-full bg-primary-100 dark:bg-primary-900/40 mb-4"
                    >
                        <span class="text-3xl">🎛️</span>
                    </div>
                    <h1
                        class="text-2xl font-bold text-gray-900 dark:text-white"
                    >
                        DJ Panel
                    </h1>
                    <p class="mt-2 text-sm text-gray-500 dark:text-gray-400">
                        Sign in to manage your radio station
                    </p>
                </div>

                <!-- Error message -->
                {#if error}
                    <div
                        class="mb-6 flex items-start gap-3 p-4 rounded-xl bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800"
                    >
                        <div class="flex-shrink-0 mt-0.5">
                            <svg
                                class="w-5 h-5 text-red-500"
                                fill="none"
                                viewBox="0 0 24 24"
                                stroke-width="2"
                                stroke="currentColor"
                            >
                                <path
                                    stroke-linecap="round"
                                    stroke-linejoin="round"
                                    d="M12 9v3.75m9-.75a9 9 0 1 1-18 0 9 9 0 0 1 18 0Zm-9 3.75h.008v.008H12v-.008Z"
                                />
                            </svg>
                        </div>
                        <div>
                            <p
                                class="text-sm font-medium text-red-800 dark:text-red-300"
                            >
                                {error}
                            </p>
                        </div>
                    </div>
                {/if}

                <!-- Form -->
                <form on:submit|preventDefault={handleSubmit} class="space-y-5">
                    <!-- Username -->
                    <div>
                        <label
                            for="username"
                            class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1.5"
                        >
                            Username
                        </label>
                        <div class="relative">
                            <div
                                class="absolute inset-y-0 left-0 pl-3.5 flex items-center pointer-events-none"
                            >
                                <svg
                                    class="w-5 h-5 text-gray-400 dark:text-gray-500"
                                    fill="none"
                                    viewBox="0 0 24 24"
                                    stroke-width="1.5"
                                    stroke="currentColor"
                                >
                                    <path
                                        stroke-linecap="round"
                                        stroke-linejoin="round"
                                        d="M15.75 6a3.75 3.75 0 1 1-7.5 0 3.75 3.75 0 0 1 7.5 0ZM4.501 20.118a7.5 7.5 0 0 1 14.998 0A17.933 17.933 0 0 1 12 21.75c-2.676 0-5.216-.584-7.499-1.632Z"
                                    />
                                </svg>
                            </div>
                            <input
                                id="username"
                                type="text"
                                bind:value={username}
                                on:keydown={handleKeydown}
                                placeholder="Enter your username"
                                autocomplete="username"
                                disabled={loading}
                                class="block w-full pl-11 pr-4 py-3 rounded-xl border border-gray-300 dark:border-gray-600
                  bg-white dark:bg-gray-700 text-gray-900 dark:text-white
                  placeholder-gray-400 dark:placeholder-gray-500
                  focus:ring-2 focus:ring-primary-500 focus:border-primary-500
                  disabled:opacity-50 disabled:cursor-not-allowed
                  transition-colors text-sm"
                            />
                        </div>
                    </div>

                    <!-- Password -->
                    <div>
                        <label
                            for="password"
                            class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1.5"
                        >
                            Password
                        </label>
                        <div class="relative">
                            <div
                                class="absolute inset-y-0 left-0 pl-3.5 flex items-center pointer-events-none"
                            >
                                <svg
                                    class="w-5 h-5 text-gray-400 dark:text-gray-500"
                                    fill="none"
                                    viewBox="0 0 24 24"
                                    stroke-width="1.5"
                                    stroke="currentColor"
                                >
                                    <path
                                        stroke-linecap="round"
                                        stroke-linejoin="round"
                                        d="M16.5 10.5V6.75a4.5 4.5 0 1 0-9 0v3.75m-.75 11.25h10.5a2.25 2.25 0 0 0 2.25-2.25v-6.75a2.25 2.25 0 0 0-2.25-2.25H6.75a2.25 2.25 0 0 0-2.25 2.25v6.75a2.25 2.25 0 0 0 2.25 2.25Z"
                                    />
                                </svg>
                            </div>
                            <input
                                id="password"
                                type="password"
                                bind:value={password}
                                on:keydown={handleKeydown}
                                placeholder="Enter your password"
                                autocomplete="current-password"
                                disabled={loading}
                                class="block w-full pl-11 pr-4 py-3 rounded-xl border border-gray-300 dark:border-gray-600
                  bg-white dark:bg-gray-700 text-gray-900 dark:text-white
                  placeholder-gray-400 dark:placeholder-gray-500
                  focus:ring-2 focus:ring-primary-500 focus:border-primary-500
                  disabled:opacity-50 disabled:cursor-not-allowed
                  transition-colors text-sm"
                            />
                        </div>
                    </div>

                    <!-- Submit button -->
                    <button
                        type="submit"
                        disabled={loading}
                        class="w-full flex items-center justify-center gap-2 px-6 py-3 rounded-xl text-sm font-semibold text-white
              bg-primary-600 hover:bg-primary-700 focus:ring-4 focus:ring-primary-300 dark:focus:ring-primary-800
              disabled:opacity-50 disabled:cursor-not-allowed
              transition-all duration-200 shadow-lg shadow-primary-200 dark:shadow-primary-900/30
              hover:shadow-xl hover:shadow-primary-300 dark:hover:shadow-primary-900/40"
                    >
                        {#if loading}
                            <svg
                                class="animate-spin w-5 h-5"
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
                                    d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                                />
                            </svg>
                            Signing in…
                        {:else}
                            Sign In
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
                                    d="M13.5 4.5 21 12m0 0-7.5 7.5M21 12H3"
                                />
                            </svg>
                        {/if}
                    </button>
                </form>
            </div>

            <!-- Footer -->
            <div
                class="px-8 py-4 bg-gray-50 dark:bg-gray-800/50 border-t border-gray-200 dark:border-gray-700"
            >
                <p class="text-xs text-center text-gray-400 dark:text-gray-500">
                    Access is restricted to authorized DJs only.
                    <br />
                    Contact your station administrator for credentials.
                </p>
            </div>
        </div>

        <!-- Back to listen link -->
        <div class="mt-6 text-center">
            <a
                href="#/"
                class="text-sm text-gray-500 dark:text-gray-400 hover:text-primary-600 dark:hover:text-primary-400 transition-colors inline-flex items-center gap-1.5"
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
                        d="M10.5 19.5 3 12m0 0 7.5-7.5M3 12h18"
                    />
                </svg>
                Back to listening
            </a>
        </div>
    </div>
</div>
