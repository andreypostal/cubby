import type {
  Task,
  TaskDayStatus,
  TaskOccurrence,
  TaskRecurrenceType,
} from "@/lib/api"
import type { Locale, TranslationKey } from "@/lib/i18n/translations"

export const SECTION_LABEL_KEYS: Record<TaskRecurrenceType, TranslationKey> = {
  once: "task.section.once",
  annual: "task.section.annual",
  monthly: "task.section.monthly",
  weekly: "task.section.weekly",
  daily: "task.section.daily",
}

export const STATUS_LABEL_KEYS: Record<TaskDayStatus, TranslationKey> = {
  completed: "task.status.completed",
  skipped: "task.status.skipped",
  failed: "task.status.failed",
  pending: "task.status.pending",
  future: "task.status.future",
}

export const STATUS_BADGE_CLASS: Record<TaskDayStatus, string> = {
  completed:
    "bg-emerald-500/15 text-emerald-700 ring-1 ring-emerald-500/40 dark:text-emerald-400",
  skipped: "bg-muted text-muted-foreground ring-1 ring-foreground/10",
  failed: "bg-red-500/10 text-red-700 ring-1 ring-red-500/30 dark:text-red-400",
  pending: "bg-amber-500/10 text-foreground ring-1 ring-amber-500/40",
  future: "bg-transparent text-muted-foreground ring-1 ring-border",
}

/** API weekdays are 0 = Sunday .. 6 = Saturday. */
export const WEEKDAY_LABEL_KEYS: TranslationKey[] = [
  "day.sunday",
  "day.monday",
  "day.tuesday",
  "day.wednesday",
  "day.thursday",
  "day.friday",
  "day.saturday",
]

/** Monday-first ordering, matching how the pickers read. */
export function sortWeekdays(weekdays: number[]): number[] {
  return [...weekdays].sort((a, b) => ((a + 6) % 7) - ((b + 6) % 7))
}

/**
 * Order the board renders its sections in. It is deliberately not the API's
 * order — a day reads best from the one-off commitments down to the habits.
 */
export const SECTION_ORDER: TaskRecurrenceType[] = [
  "once",
  "annual",
  "monthly",
  "weekly",
  "daily",
]

export type TaskSection = {
  type: TaskRecurrenceType
  occurrences: TaskOccurrence[]
}

/**
 * Groups a day's occurrences into the board's sections, dropping empty ones and
 * floating pinned tasks to the top of each section.
 */
export function buildSections(occurrences: TaskOccurrence[]): TaskSection[] {
  return SECTION_ORDER.map((type) => ({
    type,
    occurrences: occurrences
      .filter((occurrence) => occurrence.task.recurrence_type === type)
      .sort((a, b) => {
        if (a.task.pinned !== b.task.pinned) {
          return a.task.pinned ? -1 : 1
        }
        return a.task.id - b.task.id
      }),
  })).filter((section) => section.occurrences.length > 0)
}

/** Days that count as "handled" for the day's progress indicator. */
export function isSettled(occurrence: TaskOccurrence) {
  return occurrence.status !== "pending" && occurrence.status !== "future"
}

/**
 * Adds days to a `YYYY-MM-DD` string. Parsing happens in UTC so the result never
 * shifts by a day when the browser sits east or west of the board's timezone.
 */
export function shiftDate(date: string, days: number): string {
  const [year, month, day] = date.split("-").map(Number)
  const shifted = new Date(Date.UTC(year, month - 1, day + days))
  return shifted.toISOString().slice(0, 10)
}

/** Turns `YYYY-MM-DD` into a `Date` anchored at UTC midnight. */
export function parseDate(date: string): Date {
  const [year, month, day] = date.split("-").map(Number)
  return new Date(Date.UTC(year, month - 1, day))
}

/**
 * Renders a `YYYY-MM-DD` string in the active locale. Formatting stays anchored
 * at UTC so the label never drifts a day away from the board's own dates.
 */
export function formatTaskDate(
  date: string,
  locale: Locale,
  options?: Intl.DateTimeFormatOptions
): string {
  return new Intl.DateTimeFormat(locale === "pt" ? "pt-BR" : "en-US", {
    day: "2-digit",
    month: "short",
    year: "numeric",
    timeZone: "UTC",
    ...options,
  }).format(parseDate(date))
}

/**
 * Locale-aware abbreviated weekday names, indexed 0 = Sunday to match the API.
 * A weekly task listing five full weekday names is wider than any row that has
 * to share space with actions.
 */
export function shortWeekdayNames(locale: Locale): string[] {
  const formatter = new Intl.DateTimeFormat(
    locale === "pt" ? "pt-BR" : "en-US",
    { weekday: "short", timeZone: "UTC" }
  )
  // 2024-01-07 was a Sunday, so the week lines up with the API's numbering.
  return Array.from({ length: 7 }, (_, index) =>
    formatter.format(new Date(Date.UTC(2024, 0, 7 + index)))
  )
}

/**
 * Locale-aware month names, indexed 1 = January to match the API's
 * `month_of_year`. Index 0 is left empty so the lookup needs no arithmetic.
 */
export function monthNames(
  locale: Locale,
  style: "long" | "short" = "long"
): string[] {
  const formatter = new Intl.DateTimeFormat(
    locale === "pt" ? "pt-BR" : "en-US",
    { month: style, timeZone: "UTC" }
  )
  return [
    "",
    ...Array.from({ length: 12 }, (_, index) =>
      formatter.format(new Date(Date.UTC(2024, index, 1)))
    ),
  ]
}

/**
 * One-line description of when a task repeats. `shortWeekdays` abbreviates the
 * weekday names, for rows too tight to spell them out.
 */
export function describeSchedule(
  task: Task,
  t: (key: TranslationKey, vars?: Record<string, string | number>) => string,
  locale: Locale,
  options?: { shortWeekdays?: boolean }
): string {
  if (task.recurrence_type === "weekly") {
    const names = options?.shortWeekdays ? shortWeekdayNames(locale) : null
    return t("task.summary.weekly", {
      days: sortWeekdays(task.weekdays)
        .map((weekday) => names?.[weekday] ?? t(WEEKDAY_LABEL_KEYS[weekday]))
        .join(", "),
    })
  }
  if (task.recurrence_type === "monthly") {
    return t("task.summary.monthly", { day: task.day_of_month })
  }
  if (task.recurrence_type === "annual") {
    return t("task.summary.annual", {
      day: task.day_of_month,
      month: monthNames(locale)[task.month_of_year] ?? "",
    })
  }
  if (task.recurrence_type === "daily") {
    return t("task.summary.daily")
  }
  return t("task.summary.once")
}

export type CalendarMonth = { year: number; month: number }

/** Reads the year and 1-based month out of a `YYYY-MM-DD` string. */
export function monthOf(date: string): CalendarMonth {
  const [year, month] = date.split("-").map(Number)
  return { year, month }
}

/** Adds months to a year/month pair, rolling the year over as needed. */
export function shiftMonth(
  { year, month }: CalendarMonth,
  months: number
): CalendarMonth {
  const shifted = new Date(Date.UTC(year, month - 1 + months, 1))
  return { year: shifted.getUTCFullYear(), month: shifted.getUTCMonth() + 1 }
}

export type CalendarDay = {
  /** `YYYY-MM-DD` */
  date: string
  day: number
  /** False for the neighbouring-month days that pad the first and last weeks. */
  inMonth: boolean
}

/**
 * Builds the weeks (Monday-first, always 7 cells each) of a month's calendar.
 * Everything runs in UTC so the grid matches the board's dates regardless of
 * where the browser sits.
 */
export function buildMonthGrid({
  year,
  month,
}: CalendarMonth): CalendarDay[][] {
  const firstWeekday =
    (new Date(Date.UTC(year, month - 1, 1)).getUTCDay() + 6) % 7
  const daysInMonth = new Date(Date.UTC(year, month, 0)).getUTCDate()
  const cellCount = Math.ceil((firstWeekday + daysInMonth) / 7) * 7

  const weeks: CalendarDay[][] = []
  for (let index = 0; index < cellCount; index++) {
    const date = new Date(Date.UTC(year, month - 1, index - firstWeekday + 1))
    if (index % 7 === 0) {
      weeks.push([])
    }
    weeks[weeks.length - 1].push({
      date: date.toISOString().slice(0, 10),
      day: date.getUTCDate(),
      inMonth: date.getUTCMonth() === month - 1,
    })
  }
  return weeks
}
