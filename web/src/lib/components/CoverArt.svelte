<script lang="ts">
  let { src = '', alt = '', size = 'full', class: className = '' }: {
    src?: string;
    alt?: string;
    size?: 'sm' | 'md' | 'lg' | 'full';
    class?: string;
  } = $props();

  let loaded = $state(false);
  let errored = $state(false);

  // Generate a deterministic gradient from the alt text for placeholders.
  function gradientColors(label: string): string {
    let hash = 0;
    for (let i = 0; i < label.length; i++) {
      hash = label.charCodeAt(i) + ((hash << 5) - hash);
    }
    const hue1 = Math.abs(hash % 360);
    const hue2 = (hue1 + 40) % 360;
    return `linear-gradient(135deg, hsl(${hue1}, 60%, 35%) 0%, hsl(${hue2}, 55%, 25%) 100%)`;
  }

  let gradient = $derived(gradientColors(alt || 'music'));

  function initials(label: string): string {
    return label
      .split(/[\s_-]+/)
      .map(w => w[0])
      .filter(Boolean)
      .slice(0, 2)
      .join('')
      .toUpperCase();
  }

  let label = $derived(initials(alt));

  const sizes: Record<string, string> = {
    sm: 'w-10 h-10 rounded-lg text-xs',
    md: 'w-16 h-16 rounded-xl text-sm',
    lg: 'w-32 h-32 rounded-2xl text-2xl',
    full: 'w-full aspect-square rounded-2xl text-3xl',
  };
  let sizeClass = $derived(sizes[size] || sizes.full);
</script>

<div class="relative overflow-hidden {sizeClass} {className} flex-shrink-0">
  {#if src && !errored}
    <img
      {src}
      alt={alt}
      class="absolute inset-0 w-full h-full object-cover"
      class:hidden={!loaded}
      onload={() => (loaded = true)}
      onerror={() => (errored = true)}
    />
  {/if}

  <!-- Placeholder: shown while loading or when no image available -->
  <div
    class="absolute inset-0 flex items-center justify-center"
    class:hidden={loaded && !errored}
    style="background: {gradient};"
  >
    <span class="font-bold text-white/80 select-none">{label}</span>
  </div>
</div>
