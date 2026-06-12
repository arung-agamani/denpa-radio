// Shared tag utilities for the DJ panel.
// Now supports dynamic time slots from the backend while providing sensible
// defaults for the original four tags.

import type { TimeSlot } from "./api";

export type TagKey = string;

// Default visual mappings for the built-in tags.
const defaultTagEmoji: Record<string, string> = {
    morning: "🌅",
    afternoon: "☀️",
    evening: "🌇",
    night: "🌙",
};

const defaultTagLabel: Record<string, string> = {
    morning: "Morning",
    afternoon: "Afternoon",
    evening: "Evening",
    night: "Night",
};

const defaultTagColors: Record<string, string> = {
    morning:
        "bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300 border-amber-200 dark:border-amber-800",
    afternoon:
        "bg-orange-100 text-orange-700 dark:bg-orange-900/40 dark:text-orange-300 border-orange-200 dark:border-orange-800",
    evening:
        "bg-indigo-100 text-indigo-700 dark:bg-indigo-900/40 dark:text-indigo-300 border-indigo-200 dark:border-indigo-800",
    night: "bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-300 border-slate-200 dark:border-slate-700",
};

// Palette for dynamically-created tags that don't have a built-in colour.
const dynamicColors = [
    "bg-teal-100 text-teal-700 dark:bg-teal-900/40 dark:text-teal-300 border-teal-200 dark:border-teal-800",
    "bg-rose-100 text-rose-700 dark:bg-rose-900/40 dark:text-rose-300 border-rose-200 dark:border-rose-800",
    "bg-cyan-100 text-cyan-700 dark:bg-cyan-900/40 dark:text-cyan-300 border-cyan-200 dark:border-cyan-800",
    "bg-violet-100 text-violet-700 dark:bg-violet-900/40 dark:text-violet-300 border-violet-200 dark:border-violet-800",
    "bg-lime-100 text-lime-700 dark:bg-lime-900/40 dark:text-lime-300 border-lime-200 dark:border-lime-800",
    "bg-fuchsia-100 text-fuchsia-700 dark:bg-fuchsia-900/40 dark:text-fuchsia-300 border-fuchsia-200 dark:border-fuchsia-800",
];

// Export legacy objects for components that still do simple lookups. These
// work fine for the default four tags and gracefully degrade for custom
// ones.
export const tagEmoji = defaultTagEmoji;
export const tagLabel = defaultTagLabel;
export const tagColors = defaultTagColors;

// ---------------------------------------------------------------------------
// Dynamic helpers that use the live time slot list
// ---------------------------------------------------------------------------

/** Return the label for a tag, using time slot data if available. */
export function getTagLabel(tag: string, slots?: TimeSlot[]): string {
    if (slots) {
        const slot = slots.find((s) => s.tag === tag);
        if (slot) return slot.label;
    }
    return defaultTagLabel[tag] || tag;
}

/** Return the emoji for a tag. */
export function getTagEmoji(tag: string): string {
    return defaultTagEmoji[tag] || "🕐";
}

/** Return the colour classes for a tag. Uses the built-in palette first, then
 *  cycles through the dynamic palette for custom tags. */
export function getTagColor(tag: string, slots?: TimeSlot[]): string {
    if (defaultTagColors[tag]) return defaultTagColors[tag];
    // Deterministic colour based on the tag's index in the slot list.
    const idx = slots ? slots.findIndex((s) => s.tag === tag) : -1;
    const i = idx >= 0 ? idx : Math.abs(hashCode(tag));
    return dynamicColors[i % dynamicColors.length];
}

function hashCode(s: string): number {
    let h = 0;
    for (let i = 0; i < s.length; i++) {
        h = (Math.imul(31, h) + s.charCodeAt(i)) | 0;
    }
    return h;
}

/** Format an hour range for display (e.g. "6am–12pm"). */
export function formatHourRange(startHour: number, endHour: number): string {
    return `${formatHour(startHour)}–${formatHour(endHour)}`;
}

function formatHour(h: number): string {
    if (h === 0) return "12am";
    if (h === 12) return "12pm";
    if (h < 12) return `${h}am`;
    return `${h - 12}pm`;
}
