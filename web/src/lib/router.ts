import { writable, derived } from 'svelte/store';

// Current path derived from window.location hash.
// We use hash-based routing (#/path) so the Go server always serves index.html
// and the client handles navigation.

function getPath(): string {
    const hash = window.location.hash || '#/';
    return hash.replace(/^#/, '') || '/';
}

const _path = writable<string>(getPath());

// Update the store whenever the hash changes.
if (typeof window !== 'undefined') {
    window.addEventListener('hashchange', () => {
        _path.set(getPath());
    });

    window.addEventListener('popstate', () => {
        _path.set(getPath());
    });
}

// Public readable store for the current path.
export const path = { subscribe: _path.subscribe };

// Derived store that extracts just the first path segment for top-level routing.
// e.g. "/dj/playlists/3" → "dj"
export const segment = derived(path, ($path) => {
    const parts = $path.replace(/^\//, '').split('/');
    return parts[0] || '';
});

// Derived store for the full split segments array.
// e.g. "/dj/playlists/3" → ["dj", "playlists", "3"]
export const segments = derived(path, ($path) => {
    return $path
        .replace(/^\//, '')
        .split('/')
        .filter((s) => s.length > 0);
});

// Navigate to a new hash path programmatically.
export function navigate(to: string): void {
    const target = to.startsWith('/') ? to : `/${to}`;
    window.location.hash = `#${target}`;
}

// Check if the current path starts with the given prefix.
export function matchPrefix(currentPath: string, prefix: string): boolean {
    if (prefix === '/') return currentPath === '/';
    return currentPath === prefix || currentPath.startsWith(prefix + '/');
}
