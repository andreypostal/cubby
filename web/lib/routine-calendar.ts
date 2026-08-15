import type { Routine, RoutineDayStatus } from "@/lib/api"

export type CalendarCell = {
  date: Date
  dateStr: string
  day: number
  inMonth: boolean
  isToday: boolean
  status: RoutineDayStatus | null
}

const WEEKDAY_KEYS = [
  "on_monday",
  "on_tuesday",
  "on_wednesday",
  "on_thursday",
  "on_friday",
  "on_saturday",
  "on_sunday",
] as const satisfies ReadonlyArray<keyof Routine>

function formatDateStr(date: Date) {
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, "0")
  const day = String(date.getDate()).padStart(2, "0")
  return `${year}-${month}-${day}`
}

/** Monday-first weekday index (0 = Monday, 6 = Sunday) for a given date. */
function mondayFirstWeekday(date: Date) {
  return (date.getDay() + 6) % 7
}

/**
 * Builds a full calendar grid (weeks of 7 days, Monday-first) for the given
 * year/month, including grayed-out leading/trailing days from neighboring
 * months so every week row has 7 cells.
 */
function buildCalendarGrid(
  year: number,
  month: number,
  statusByDate: Map<string, RoutineDayStatus>
): CalendarCell[][] {
  const firstOfMonth = new Date(year, month - 1, 1)
  const daysInMonth = new Date(year, month, 0).getDate()
  const leadingCount = mondayFirstWeekday(firstOfMonth)

  const today = new Date()
  const todayStr = formatDateStr(today)

  const cells: CalendarCell[] = []

  for (let i = leadingCount; i > 0; i--) {
    const date = new Date(year, month - 1, 1 - i)
    cells.push({
      date,
      dateStr: formatDateStr(date),
      day: date.getDate(),
      inMonth: false,
      isToday: false,
      status: null,
    })
  }

  for (let day = 1; day <= daysInMonth; day++) {
    const date = new Date(year, month - 1, day)
    const dateStr = formatDateStr(date)
    cells.push({
      date,
      dateStr,
      day,
      inMonth: true,
      isToday: dateStr === todayStr,
      status: statusByDate.get(dateStr) ?? null,
    })
  }

  const trailingCount = (7 - (cells.length % 7)) % 7
  for (let day = 1; day <= trailingCount; day++) {
    const date = new Date(year, month, day)
    cells.push({
      date,
      dateStr: formatDateStr(date),
      day: date.getDate(),
      inMonth: false,
      isToday: false,
      status: null,
    })
  }

  const weeks: CalendarCell[][] = []
  for (let i = 0; i < cells.length; i += 7) {
    weeks.push(cells.slice(i, i + 7))
  }
  return weeks
}

function isDayClickable(cell: CalendarCell) {
  return (
    cell.inMonth &&
    (cell.status === "waiting" ||
      cell.status === "achieved" ||
      cell.status === "failed")
  )
}

function monthIndex(year: number, month: number) {
  return year * 12 + (month - 1)
}

export { buildCalendarGrid, formatDateStr, isDayClickable, monthIndex, WEEKDAY_KEYS }
