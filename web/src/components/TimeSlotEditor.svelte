<script lang="ts">
    import type { TimeSlot } from "../lib/api";
    import { createEventDispatcher, onMount } from "svelte";
    import { getTagEmoji } from "../lib/tags";

    // ---------------------------------------------------------------------------
    // Props / events
    // ---------------------------------------------------------------------------

    export let slots: TimeSlot[] = [];
    export let saving = false;

    const dispatch = createEventDispatcher<{ save: TimeSlot[]; cancel: void }>();

    // ---------------------------------------------------------------------------
    // Working copy with stable keys
    //
    // Every slot gets a stable _key so the {#each} identity never changes
    // even when the tag/label fields are edited. This prevents Svelte 4
    // from losing track of DOM elements during reactivity cycles.
    // ---------------------------------------------------------------------------

    interface SlotItem extends TimeSlot {
        _key: number;
    }

    let editSlots: SlotItem[] = [];
    let nextKey = 1;

    function slotToItem(s: TimeSlot, key?: number): SlotItem {
        return { ...s, _key: key ?? nextKey++ };
    }

    // Initialise once from the parent prop.
    onMount(() => {
        editSlots = slots.map((s) => slotToItem(s));
    });

    // ---------------------------------------------------------------------------
    // Geometry helpers
    // ---------------------------------------------------------------------------

    /** Duration of a slot in circular hour space (1–24). */
    function circularDur(s: TimeSlot): number {
        if (s.endHour === s.startHour) return 24;
        return ((s.endHour - s.startHour) + 24) % 24;
    }

    /** True when the slot crosses midnight (e.g. start=22 end=4). */
    function wraps(s: TimeSlot): boolean {
        return s.endHour !== 0 && s.endHour < s.startHour;
    }

    /**
     * Linearise a circular hour relative to a fixed anchor so that all hours
     * lie on a continuous 0–48+ line.  This eliminates the wrap-around
     * headache when clamping boundary drags.
     */
    function linearise(hour: number, anchor: number): number {
        let h = hour - anchor;
        if (h < 0) h += 24;
        return h;
    }

    /**
     * Convert a linearised hour back to the 0–23 circle.
     */
    function delinearise(linear: number, anchor: number): number {
        return (anchor + linear) % 24;
    }

    /** Visual segment(s) for a slot as { left%, width% }.
     *  A midnight-crossing slot produces two segments. */
    interface Seg { left: number; width: number }
    function segments(s: TimeSlot): Seg[] {
        const dur = circularDur(s);
        if (wraps(s)) {
            return [
                { left: (s.startHour / 24) * 100, width: ((24 - s.startHour) / 24) * 100 },
                { left: 0,                         width: (s.endHour / 24) * 100 },
            ];
        }
        return [{ left: (s.startHour / 24) * 100, width: (dur / 24) * 100 }];
    }

    // ---------------------------------------------------------------------------
    // Boundary handles
    // ---------------------------------------------------------------------------

    interface Handle { hour: number; leftTag: string; rightTag: string }

    $: handles = computeHandles(editSlots);

    function computeHandles(sl: SlotItem[]): Handle[] {
        const out: Handle[] = [];
        const seen = new Set<number>();
        for (const s of sl) {
            const h = s.startHour;
            if (seen.has(h)) continue;
            const prev = sl.find((o) => o.tag !== s.tag && o.endHour === h);
            if (prev) {
                out.push({ hour: h, leftTag: prev.tag, rightTag: s.tag });
                seen.add(h);
            }
        }
        return out;
    }

    // ---------------------------------------------------------------------------
    // Drag state
    // ---------------------------------------------------------------------------

    type DragState =
        | {
              kind: "resize";
              leftTag: string;
              rightTag: string;
              anchor: number;    // left slot's startHour (linear anchor)
              total: number;     // combined duration of both slots
          }
        | {
              kind: "move";
              tag: string;
              duration: number;
              origStart: number;
              prevTag: string;
              nextTag: string;
              prevOrigDur: number;
              nextOrigDur: number;
              sameNeighbour: boolean;
              mouseHour0: number;
          }
        | null;

    let drag: DragState = null;
    let timelineEl: HTMLDivElement | null = null;

    /** Raw (unsnapped) hour position from a pixel X coordinate. */
    function pxToHour(clientX: number): number {
        if (!timelineEl) return 0;
        const { left, width } = timelineEl.getBoundingClientRect();
        return Math.max(0, Math.min(24, ((clientX - left) / width) * 24));
    }

    // ---------------------------------------------------------------------------
    // Resize drag (boundary handle)
    // ---------------------------------------------------------------------------

    function startResize(e: MouseEvent | TouchEvent, h: Handle) {
        e.preventDefault();
        e.stopPropagation();
        const ls = editSlots.find((s) => s.tag === h.leftTag)!;
        const rs = editSlots.find((s) => s.tag === h.rightTag)!;
        drag = {
            kind: "resize",
            leftTag: h.leftTag,
            rightTag: h.rightTag,
            anchor: ls.startHour,
            total: circularDur(ls) + circularDur(rs),
        };
        attachListeners();
    }

    function applyResize(clientX: number) {
        if (drag?.kind !== "resize") return;
        const { leftTag, rightTag, anchor, total } = drag;

        const rawHour = Math.round(pxToHour(clientX));
        const linearMouse = linearise(rawHour, anchor);
        const linearClamp = Math.max(1, Math.min(total - 1, linearMouse));
        const newHour = delinearise(linearClamp, anchor);

        editSlots = editSlots.map((s) => {
            if (s.tag === leftTag)  return { ...s, endHour: newHour };
            if (s.tag === rightTag) return { ...s, startHour: newHour };
            return s;
        });
    }

    // ---------------------------------------------------------------------------
    // Move drag (slot body)
    // ---------------------------------------------------------------------------

    function startMove(e: MouseEvent | TouchEvent, tag: string) {
        e.preventDefault();
        const slot = editSlots.find((s) => s.tag === tag);
        if (!slot) return;

        const dur = circularDur(slot);
        const endH  = slot.endHour;
        const prev  = editSlots.find((s) => s.tag !== tag && s.endHour === slot.startHour);
        const next  = editSlots.find((s) => s.tag !== tag && s.startHour === endH);

        if (!prev || !next) return;

        const clientX = e instanceof MouseEvent ? e.clientX : e.touches[0].clientX;

        drag = {
            kind: "move",
            tag,
            duration: dur,
            origStart: slot.startHour,
            prevTag: prev.tag,
            nextTag: next.tag,
            prevOrigDur: circularDur(prev),
            nextOrigDur: circularDur(next),
            sameNeighbour: prev.tag === next.tag,
            mouseHour0: pxToHour(clientX),
        };
        attachListeners();
    }

    function applyMove(clientX: number) {
        if (drag?.kind !== "move") return;
        const {
            tag, duration, origStart,
            prevTag, nextTag,
            prevOrigDur, nextOrigDur,
            sameNeighbour, mouseHour0,
        } = drag;

        const rawDelta = pxToHour(clientX) - mouseHour0;

        const delta = sameNeighbour
            ? rawDelta
            : Math.max(-(prevOrigDur - 1), Math.min(nextOrigDur - 1, rawDelta));

        const deltaH = Math.round(delta);
        if (deltaH === 0) return;

        const newStart = ((origStart + deltaH) % 24 + 24) % 24;
        const newEnd   = (newStart + duration) % 24;

        editSlots = editSlots.map((s) => {
            if (s.tag === tag) return { ...s, startHour: newStart, endHour: newEnd };
            if (sameNeighbour && s.tag === prevTag) {
                return { ...s, startHour: newEnd, endHour: newStart };
            }
            if (!sameNeighbour && s.tag === prevTag) return { ...s, endHour: newStart };
            if (!sameNeighbour && s.tag === nextTag) return { ...s, startHour: newEnd };
            return s;
        });
    }

    // ---------------------------------------------------------------------------
    // Pointer event wiring
    // ---------------------------------------------------------------------------

    function onPointerMove(e: MouseEvent | TouchEvent) {
        const cx = e instanceof MouseEvent ? e.clientX : e.touches[0].clientX;
        if (drag?.kind === "resize") applyResize(cx);
        if (drag?.kind === "move")   applyMove(cx);
    }

    function onPointerUp() { drag = null; removeListeners(); }

    function onMouseMove(e: MouseEvent) { onPointerMove(e); }
    function onTouchMove(e: TouchEvent) { e.preventDefault(); onPointerMove(e); }
    function onMouseUp()  { onPointerUp(); }
    function onTouchEnd() { onPointerUp(); }

    function attachListeners() {
        window.addEventListener("mousemove", onMouseMove);
        window.addEventListener("mouseup",   onMouseUp);
        window.addEventListener("touchmove", onTouchMove, { passive: false });
        window.addEventListener("touchend",  onTouchEnd);
    }
    function removeListeners() {
        window.removeEventListener("mousemove", onMouseMove);
        window.removeEventListener("mouseup",   onMouseUp);
        window.removeEventListener("touchmove", onTouchMove);
        window.removeEventListener("touchend",  onTouchEnd);
    }

    onMount(() => removeListeners);

    // ---------------------------------------------------------------------------
    // Bar colours  (stable per tag, hex so they work in inline style)
    // ---------------------------------------------------------------------------

    const BAR_HEX = [
        "#f59e0b", "#38bdf8", "#818cf8", "#94a3b8", "#2dd4bf",
        "#fb7185", "#a78bfa", "#a3e635", "#fb923c", "#22d3ee",
    ];

    let tagColorMap: Record<string, number> = {};
    let colorCounter = 0;

    $: {
        for (const s of editSlots) {
            if (!(s.tag in tagColorMap)) {
                tagColorMap[s.tag] = colorCounter++ % BAR_HEX.length;
                tagColorMap = tagColorMap;
            }
        }
    }

    function barHex(tag: string): string {
        return BAR_HEX[tagColorMap[tag] ?? 0];
    }

    // ---------------------------------------------------------------------------
    // Add / remove
    // ---------------------------------------------------------------------------

    function addSlot() {
        if (editSlots.length === 0) {
            editSlots = [{ _key: nextKey++, tag: "slot1", label: "New Slot", startHour: 0, endHour: 0 }];
            return;
        }
        const largest = [...editSlots].sort((a, b) => circularDur(b) - circularDur(a))[0];
        const dur = circularDur(largest);
        if (dur < 2) return;
        const mid = (largest.startHour + Math.floor(dur / 2)) % 24;
        const newTag = generateTag();
        const key = nextKey++;
        editSlots = editSlots
            .map((s) => (s._key === largest._key ? { ...s, endHour: mid } : s))
            .concat([{ _key: key, tag: newTag, label: "New Slot", startHour: mid, endHour: largest.endHour }]);
    }

    function generateTag(): string {
        const existing = new Set(editSlots.map((s) => s.tag));
        let i = 1;
        while (existing.has(`slot${i}`)) i++;
        return `slot${i}`;
    }

    function removeSlot(tag: string) {
        if (editSlots.length <= 1) return;
        const removed = editSlots.find((s) => s.tag === tag);
        if (!removed) return;

        const prev = editSlots.find((s) => s.tag !== tag && s.endHour === removed.startHour);
        if (prev) {
            editSlots = editSlots
                .map((s) => (s._key === prev._key ? { ...s, endHour: removed.endHour } : s))
                .filter((s) => s.tag !== tag);
        } else {
            editSlots = editSlots.filter((s) => s.tag !== tag);
        }
    }

    // ---------------------------------------------------------------------------
    // Validation
    // ---------------------------------------------------------------------------

    interface VErr { message: string }
    $: errors  = validate(editSlots);
    $: isValid = errors.length === 0;

    function validate(sl: SlotItem[]): VErr[] {
        const errs: VErr[] = [];
        if (sl.length === 0) { errs.push({ message: "At least one time slot is required." }); return errs; }

        const tags   = sl.map((x) => x.tag.trim());
        const labels = sl.map((x) => x.label.trim());
        if (tags.some((t)   => !t)) errs.push({ message: "Every slot needs a non-empty tag identifier." });
        if (labels.some((l) => !l)) errs.push({ message: "Every slot needs a non-empty label." });
        if (new Set(tags).size   < tags.length)   errs.push({ message: "Tag identifiers must be unique." });
        if (new Set(labels).size < labels.length) errs.push({ message: "Labels must be unique." });

        for (const s of sl) {
            if (circularDur(s) < 1) {
                errs.push({ message: `Slot "${s.label || s.tag}" must cover at least 1 hour.` });
            }
        }

        const covered = new Array(24).fill(false);
        for (const s of sl) {
            const hrs = slotHours(s);
            for (const h of hrs) {
                if (covered[h]) errs.push({ message: `Hour ${h}:00 is claimed by more than one slot.` });
                covered[h] = true;
            }
        }
        const gap = covered.indexOf(false);
        if (gap !== -1) errs.push({ message: `Gap in coverage starting at ${gap}:00.` });
        return errs;
    }

    function slotHours(s: TimeSlot): number[] {
        const hrs: number[] = [];
        if (!wraps(s)) {
            const end = s.endHour === 0 ? 24 : s.endHour;
            for (let h = s.startHour; h < end; h++) hrs.push(h);
        } else {
            for (let h = s.startHour; h < 24;        h++) hrs.push(h);
            for (let h = 0;           h < s.endHour; h++) hrs.push(h);
        }
        return hrs;
    }

    // ---------------------------------------------------------------------------
    // Tick marks / formatting
    // ---------------------------------------------------------------------------

    const TICKS = [0, 3, 6, 9, 12, 15, 18, 21, 24];
    function tickLabel(h: number): string {
        if (h === 0 || h === 24) return "12am";
        if (h === 12) return "12pm";
        return h < 12 ? `${h}am` : `${h - 12}pm`;
    }
    function rangeLabel(s: TimeSlot): string {
        const e = s.endHour === 0 ? 24 : s.endHour;
        return `${s.startHour}:00–${e}:00`;
    }

    $: timelineCursor =
        drag?.kind === "move"   ? "cursor-grabbing" :
        drag?.kind === "resize" ? "cursor-col-resize" : "";

    // ---------------------------------------------------------------------------
    // Row drag-to-reorder
    // ---------------------------------------------------------------------------

    let rowDragKey:     number | null = null;
    let rowDragOverKey: number | null = null;

    function onRowDragStart(key: number) {
        rowDragKey = key;
    }

    function onRowDragOver(e: DragEvent, key: number) {
        e.preventDefault();
        rowDragOverKey = key;
    }

    function onRowDrop(key: number) {
        if (!rowDragKey || rowDragKey === key) { rowDragKey = rowDragOverKey = null; return; }
        const arr      = [...editSlots];
        const fromIdx  = arr.findIndex((s) => s._key === rowDragKey);
        const toIdx    = arr.findIndex((s) => s._key === key);
        if (fromIdx !== -1 && toIdx !== -1) {
            const [item] = arr.splice(fromIdx, 1);
            arr.splice(toIdx, 0, item);
            editSlots = arr;
        }
        rowDragKey = rowDragOverKey = null;
    }

    function onRowDragEnd() { rowDragKey = rowDragOverKey = null; }

    // ---------------------------------------------------------------------------
    // Save / Cancel
    //
    // Strip the internal _key before dispatching so the parent never sees it.
    // ---------------------------------------------------------------------------

    function handleSave() {
        const clean: TimeSlot[] = editSlots.map(({ _key: _k, ...rest }) => rest);
        dispatch("save", clean);
    }

    function handleCancel() { dispatch("cancel"); }
</script>

<div class="space-y-5">
    <!-- ------------------------------------------------------------------ -->
    <!-- Timeline bar                                                        -->
    <!-- ------------------------------------------------------------------ -->
    <div class="select-none">
        <p class="text-xs font-semibold uppercase tracking-wider text-gray-400 dark:text-gray-500 mb-2">
            Timeline — drag a slot to move it · drag a boundary handle to resize
        </p>

        <!-- The bar itself -->
        <div
            bind:this={timelineEl}
            class="relative h-14 rounded-lg overflow-hidden bg-gray-200 dark:bg-gray-700 {timelineCursor}"
            role="group"
            aria-label="24-hour time slot timeline"
        >
            <!-- Slot segments – one or two visual bars per slot (midnight-crossing yields two) -->
            {#each editSlots as slot (slot.tag)}
                {#each segments(slot) as seg, segIdx}
                    <!-- svelte-ignore a11y-interactive-supports-focus -->
                    <button
                        type="button"
                        class="absolute top-0 h-full flex items-center justify-center overflow-hidden focus:outline-none focus-visible:ring-2 focus-visible:ring-white/70
                               {drag?.kind === 'move' && drag.tag === slot.tag ? 'cursor-grabbing opacity-80' : 'cursor-grab'}"
                        style="left:{seg.left}%;width:{seg.width}%;background-color:{barHex(slot.tag)};"
                        title="{slot.label || slot.tag} · {rangeLabel(slot)}{wraps(slot) ? ' (crosses midnight)' : ''}"
                        on:mousedown={(e) => startMove(e, slot.tag)}
                        on:touchstart|preventDefault={(e) => startMove(e, slot.tag)}
                        aria-label="Move {slot.label || slot.tag}"
                    >
                        {#if seg.width > 5}
                            <span class="text-xs font-bold text-white drop-shadow-sm px-1 truncate pointer-events-none leading-tight text-center">
                                {segIdx === 0 ? (slot.label || slot.tag) : "↩"}
                            </span>
                        {/if}
                        {#if wraps(slot) && segIdx === 0}
                            <!-- Midnight-wrap dot on the trailing edge of the first segment -->
                            <span class="absolute right-0.5 top-1/2 -translate-y-1/2 w-1.5 h-1.5 rounded-full bg-white/70 pointer-events-none"></span>
                        {/if}
                    </button>
                {/each}
            {/each}

            <!-- Boundary handles (z-10, rendered above slots) -->
            {#each handles as h (h.hour)}
                {@const pct = (h.hour / 24) * 100}
                <!-- svelte-ignore a11y-interactive-supports-focus -->
                <div
                    class="absolute top-0 h-full w-4 z-10 flex flex-col items-center justify-center cursor-col-resize group/bh"
                    style="left:calc({pct}% - 8px);"
                    role="slider"
                    tabindex="0"
                    aria-label="Resize boundary at {h.hour}:00"
                    aria-valuenow={h.hour}
                    aria-valuemin={0}
                    aria-valuemax={24}
                    on:mousedown={(e) => startResize(e, h)}
                    on:touchstart|preventDefault={(e) => startResize(e, h)}
                    on:keydown={(e) => {
                        if (e.key !== 'ArrowLeft' && e.key !== 'ArrowRight') return;
                        e.preventDefault();
                        const delta = e.key === 'ArrowLeft' ? -1 : 1;
                        const leftSlot  = editSlots.find((s) => s.tag === h.leftTag);
                        const rightSlot = editSlots.find((s) => s.tag === h.rightTag);
                        if (!leftSlot || !rightSlot) return;

                        // Linearise boundary hour relative to the left slot's
                        // startHour so clamping works correctly even when one
                        // of the slots crosses midnight.
                        const anchor = leftSlot.startHour;
                        const total  = circularDur(leftSlot) + circularDur(rightSlot);
                        const curLin = linearise(h.hour, anchor);
                        const newLin = Math.max(1, Math.min(total - 1, curLin + delta));
                        const newH   = delinearise(newLin, anchor);

                        editSlots = editSlots.map((s) => {
                            if (s.tag === leftSlot.tag)  return { ...s, endHour: newH };
                            if (s.tag === rightSlot.tag) return { ...s, startHour: newH };
                            return s;
                        });
                    }}
                >
                    <div class="w-0.5 h-9 rounded-full bg-white/70 dark:bg-white/50 shadow-md
                                group-hover/bh:w-1 group-hover/bh:bg-white transition-all duration-100 pointer-events-none"></div>
                    <span class="absolute bottom-0 text-[9px] font-semibold text-white/90 drop-shadow pointer-events-none
                                 opacity-0 group-hover/bh:opacity-100 transition-opacity whitespace-nowrap pb-0.5">
                        {h.hour}:00
                    </span>
                </div>
            {/each}
        </div>

        <!-- Hour tick labels -->
        <div class="relative h-5 mt-0.5">
            {#each TICKS as h}
                <span
                    class="absolute text-[10px] text-gray-400 dark:text-gray-500 -translate-x-1/2 pointer-events-none"
                    style="left:{(h / 24) * 100}%;"
                >{tickLabel(h)}</span>
            {/each}
        </div>
    </div>

    <!-- ------------------------------------------------------------------ -->
    <!-- Validation banner                                                   -->
    <!-- ------------------------------------------------------------------ -->
    {#if errors.length > 0}
        <div class="rounded-lg border border-red-200 dark:border-red-900 bg-red-50 dark:bg-red-950/40 px-4 py-3 space-y-1">
            <p class="text-xs font-bold text-red-700 dark:text-red-400 flex items-center gap-1.5">
                <svg class="w-3.5 h-3.5 flex-shrink-0" fill="currentColor" viewBox="0 0 20 20">
                    <path fill-rule="evenodd" d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7 4a1 1 0 11-2 0 1 1 0 012 0zm-1-9a1 1 0 00-1 1v4a1 1 0 102 0V6a1 1 0 00-1-1z" clip-rule="evenodd" />
                </svg>
                This configuration is not valid:
            </p>
            {#each errors as err}
                <p class="text-xs text-red-600 dark:text-red-400 pl-5">– {err.message}</p>
            {/each}
        </div>
    {:else}
        <div class="rounded-lg border border-green-200 dark:border-green-900 bg-green-50 dark:bg-green-950/40 px-4 py-2.5 flex items-center gap-2">
            <svg class="w-4 h-4 text-green-600 dark:text-green-400 flex-shrink-0" fill="currentColor" viewBox="0 0 20 20">
                <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd" />
            </svg>
            <p class="text-xs font-semibold text-green-700 dark:text-green-400">All 24 hours are covered with no gaps or overlaps.</p>
        </div>
    {/if}

    <!-- ------------------------------------------------------------------ -->
    <!-- Per-slot detail rows                                                -->
    <!-- ------------------------------------------------------------------ -->
    <div class="space-y-2">
        <p class="text-xs font-semibold uppercase tracking-wider text-gray-400 dark:text-gray-500">Slot Details</p>
        <!--
            Key by object reference (slot) rather than (slot.tag) so that
            editing the tag field doesn't recreate the DOM node and lose
            input focus. Using editSlots (not sortedSlots) so drag-reorder
            reflects in array save order.
        -->
        {#each editSlots as slot (slot._key)}
            <!-- svelte-ignore a11y-no-static-element-interactions -->
            <div
                class="flex items-center gap-2 p-3 rounded-lg bg-gray-50 dark:bg-gray-900/50 border transition-colors
                       {rowDragOverKey === slot._key && rowDragKey !== slot._key
                           ? 'border-primary-400 dark:border-primary-500'
                           : 'border-gray-200 dark:border-gray-700'}"
                on:dragover={(e) => onRowDragOver(e, slot._key)}
                on:drop={() => onRowDrop(slot._key)}
                on:dragleave={() => { if (rowDragOverKey === slot._key) rowDragOverKey = null; }}
            >
                <!-- Drag grip handle -->
                <!-- svelte-ignore a11y-no-static-element-interactions -->
                <span
                    class="flex-shrink-0 cursor-grab active:cursor-grabbing p-0.5 text-gray-300 hover:text-gray-500 dark:text-gray-600 dark:hover:text-gray-400 select-none"
                    draggable="true"
                    title="Drag to reorder"
                    on:dragstart={() => onRowDragStart(slot._key)}
                    on:dragend={onRowDragEnd}
                >
                    <svg class="w-4 h-4" fill="currentColor" viewBox="0 0 20 20">
                        <path d="M7 4a1 1 0 100 2 1 1 0 000-2zM7 9a1 1 0 100 2 1 1 0 000-2zM7 14a1 1 0 100 2 1 1 0 000-2zM13 4a1 1 0 100 2 1 1 0 000-2zM13 9a1 1 0 100 2 1 1 0 000-2zM13 14a1 1 0 100 2 1 1 0 000-2z"/>
                    </svg>
                </span>

                <!-- Colour swatch -->
                <span class="flex-shrink-0 w-3 h-8 rounded-sm" style="background-color:{barHex(slot.tag)};"></span>

                <!-- Hour range badge (read-only; controlled by the timeline) -->
                <span class="flex-shrink-0 w-28 text-center text-xs font-mono text-gray-500 dark:text-gray-400 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded px-1.5 py-1.5">
                    {rangeLabel(slot)}{wraps(slot) ? " ↩" : ""}
                </span>

                <!-- Tag identifier -->
                <input
                    type="text"
                    aria-label="Tag identifier"
                    bind:value={slot.tag}
                    placeholder="tag"
                    class="w-28 px-3 py-1.5 rounded-md border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-white text-xs focus:ring-2 focus:ring-primary-500 focus:border-primary-500"
                />

                <!-- Label -->
                <input
                    type="text"
                    aria-label="Label"
                    bind:value={slot.label}
                    placeholder="Label"
                    class="flex-1 px-3 py-1.5 rounded-md border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-white text-xs focus:ring-2 focus:ring-primary-500 focus:border-primary-500"
                />

                <!-- Emoji hint -->
                <span class="text-xl w-8 text-center select-none" title="Emoji based on tag name">
                    {getTagEmoji(slot.tag)}
                </span>

                <!-- Remove -->
                <button
                    type="button"
                    class="p-1.5 rounded-md text-gray-400 hover:text-red-600 hover:bg-red-50 dark:hover:bg-red-900/30 dark:hover:text-red-400 transition-colors"
                    title="Remove this slot"
                    on:click={() => removeSlot(slot.tag)}
                >
                    <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
                        <path stroke-linecap="round" stroke-linejoin="round" d="M6 18 18 6M6 6l12 12" />
                    </svg>
                </button>
            </div>
        {/each}
    </div>

    <!-- ------------------------------------------------------------------ -->
    <!-- Actions                                                             -->
    <!-- ------------------------------------------------------------------ -->
    <div class="flex items-center gap-2 pt-1">
        <button
            type="button"
            class="px-4 py-2 text-xs font-semibold text-gray-600 dark:text-gray-300 border border-gray-300 dark:border-gray-600 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
            on:click={addSlot}
        >
            + Add Slot
        </button>
        <div class="flex-1"></div>
        <button
            type="button"
            class="px-4 py-2 text-xs font-semibold text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200 transition-colors"
            on:click={handleCancel}
        >
            Cancel
        </button>
        <button
            type="button"
            class="px-5 py-2 text-xs font-semibold text-white bg-primary-600 hover:bg-primary-700 rounded-lg transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
            on:click={handleSave}
            disabled={saving || !isValid}
        >
            {saving ? "Saving…" : "Save"}
        </button>
    </div>
</div>
