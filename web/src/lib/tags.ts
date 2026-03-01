// Shared tag constants used across DJ panel sections.

export type TagKey = "morning" | "afternoon" | "evening" | "night";

export const tagEmoji: Record<string, string> = {
    morning: "🌅",
    afternoon: "☀️",
    evening: "🌇",
    night: "🌙",
};

export const tagLabel: Record<string, string> = {
    morning: "Morning",
    afternoon: "Afternoon",
    evening: "Evening",
    night: "Night",
};

export const tagColors: Record<string, string> = {
    morning:
        "bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300 border-amber-200 dark:border-amber-800",
    afternoon:
        "bg-orange-100 text-orange-700 dark:bg-orange-900/40 dark:text-orange-300 border-orange-200 dark:border-orange-800",
    evening:
        "bg-indigo-100 text-indigo-700 dark:bg-indigo-900/40 dark:text-indigo-300 border-indigo-200 dark:border-indigo-800",
    night: "bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-300 border-slate-200 dark:border-slate-700",
};
