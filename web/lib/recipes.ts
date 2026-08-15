import type { TranslationKey } from "@/lib/i18n/translations"

/**
 * Splits a total in minutes into the hours/minutes pair the form edits, and
 * back. The API only ever stores the total.
 */
function splitMinutes(total: number) {
  const safe = Number.isFinite(total) && total > 0 ? Math.floor(total) : 0
  return { hours: Math.floor(safe / 60), minutes: safe % 60 }
}

function joinMinutes(hours: number, minutes: number) {
  const h = Number.isFinite(hours) && hours > 0 ? Math.floor(hours) : 0
  const m = Number.isFinite(minutes) && minutes > 0 ? Math.floor(minutes) : 0
  return h * 60 + m
}

/**
 * Picks the translation key and variables for a duration, so `90` reads as
 * "1h 30min" instead of "90 min".
 */
function durationLabel(total: number): {
  key: TranslationKey
  vars?: Record<string, number>
} {
  if (!total || total <= 0) {
    return { key: "recipe.noTime" }
  }

  const { hours, minutes } = splitMinutes(total)

  if (hours === 0) {
    return { key: "recipe.minutes", vars: { minutes } }
  }
  if (minutes === 0) {
    return { key: "recipe.hours", vars: { hours } }
  }
  return { key: "recipe.hoursMinutes", vars: { hours, minutes } }
}

/** Moves an item one slot up or down, returning a new array. */
function moveItem<T>(items: T[], index: number, direction: -1 | 1): T[] {
  const target = index + direction
  if (target < 0 || target >= items.length) {
    return items
  }

  const next = [...items]
  const [moved] = next.splice(index, 1)
  next.splice(target, 0, moved)
  return next
}

/**
 * Flattens markdown into a single line of plain text, for the summary shown
 * while a step is collapsed. Mounting a second editor just to preview a
 * collapsed step would cost far more than it is worth.
 */
function markdownPreview(markdown: string, limit = 140) {
  const text = markdown
    .replace(/```[\s\S]*?```/g, " ")
    .replace(/!?\[([^\]]*)\]\([^)]*\)/g, "$1")
    .replace(/^\s{0,3}#{1,6}\s+/gm, "")
    .replace(/^\s{0,3}>\s?/gm, "")
    .replace(/^\s*[-*+]\s+(\[[ xX]\]\s+)?/gm, "")
    .replace(/^\s*\d+\.\s+/gm, "")
    .replace(/[*_`~]/g, "")
    .replace(/\s+/g, " ")
    .trim()

  return text.length > limit ? `${text.slice(0, limit).trimEnd()}...` : text
}

export {
  durationLabel,
  joinMinutes,
  markdownPreview,
  moveItem,
  splitMinutes,
}
