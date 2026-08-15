import * as React from "react"
import { HugeiconsIcon } from "@hugeicons/react"
import {
  ArrowDown01Icon,
  ArrowLeft01Icon,
  ArrowRight01Icon,
} from "@hugeicons/core-free-icons"

import { cn } from "@/lib/utils"
import { Button } from "@/components/ui/button"
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover"
import { useI18n } from "@/lib/i18n/context"
import type { TranslationKey } from "@/lib/i18n/translations"
import {
  buildMonthGrid,
  monthOf,
  parseDate,
  shiftMonth,
} from "@/lib/task-board"

const DAY_INITIAL_KEYS: TranslationKey[] = [
  "day.mon",
  "day.tue",
  "day.wed",
  "day.thu",
  "day.fri",
  "day.sat",
  "day.sun",
]

/**
 * The board's day heading, doubling as the trigger for a month calendar so a
 * user can jump to any day instead of stepping through them one arrow at a time.
 */
function TaskDayPicker({
  date,
  today,
  onSelect,
}: {
  date: string
  today: string | null
  onSelect: (date: string) => void
}) {
  const { t } = useI18n()

  const [open, setOpen] = React.useState(false)
  const [visibleMonth, setVisibleMonth] = React.useState(() => monthOf(date))

  // Opening always lands on the selected day's month, however far the user
  // browsed away the last time the calendar was open.
  const [lastOpenedDate, setLastOpenedDate] = React.useState<string | null>(
    null
  )
  if (open && lastOpenedDate !== date) {
    setLastOpenedDate(date)
    setVisibleMonth(monthOf(date))
  }
  if (!open && lastOpenedDate !== null) {
    setLastOpenedDate(null)
  }

  const weeks = React.useMemo(
    () => buildMonthGrid(visibleMonth),
    [visibleMonth]
  )

  function select(next: string) {
    setOpen(false)
    if (next !== date) {
      onSelect(next)
    }
  }

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger
        render={
          <button
            type="button"
            title={t("task.pickDay")}
            // `w-full` keeps the button from sizing to its intrinsic content
            // (the browser default for form controls) and overflowing the
            // heading, which is what lets the label truncate on narrow screens.
            className="flex w-full min-w-0 cursor-pointer items-center gap-1.5 rounded-md text-left underline-offset-4 hover:underline focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-ring"
          />
        }
      >
        <DayLabel date={date} />
        <HugeiconsIcon
          icon={ArrowDown01Icon}
          size={20}
          className="shrink-0 text-muted-foreground"
        />
      </PopoverTrigger>
      <PopoverContent align="start" className="w-auto gap-3 p-3">
        <div className="flex items-center justify-between gap-2">
          <Button
            type="button"
            variant="ghost"
            size="icon-sm"
            onClick={() => setVisibleMonth(shiftMonth(visibleMonth, -1))}
          >
            <HugeiconsIcon icon={ArrowLeft01Icon} />
            <span className="sr-only">{t("task.monthPrev")}</span>
          </Button>
          <p className="text-sm font-medium capitalize">
            <MonthLabel {...visibleMonth} />
          </p>
          <Button
            type="button"
            variant="ghost"
            size="icon-sm"
            onClick={() => setVisibleMonth(shiftMonth(visibleMonth, 1))}
          >
            <HugeiconsIcon icon={ArrowRight01Icon} />
            <span className="sr-only">{t("task.monthNext")}</span>
          </Button>
        </div>

        <div>
          <div className="grid grid-cols-7 gap-1">
            {DAY_INITIAL_KEYS.map((key, index) => (
              <div
                key={index}
                className="flex h-6 w-9 items-center justify-center text-xs font-medium text-muted-foreground"
              >
                {t(key)}
              </div>
            ))}
          </div>
          <div className="mt-1 flex flex-col gap-1">
            {weeks.map((week) => (
              <div key={week[0].date} className="grid grid-cols-7 gap-1">
                {week.map((cell) => {
                  const selected = cell.date === date
                  const isToday = cell.date === today
                  return (
                    <button
                      key={cell.date}
                      type="button"
                      aria-current={selected ? "date" : undefined}
                      onClick={() => select(cell.date)}
                      className={cn(
                        "flex size-9 cursor-pointer items-center justify-center rounded-full text-sm tabular-nums transition-colors",
                        !cell.inMonth && "text-muted-foreground/50",
                        selected
                          ? "bg-primary font-medium text-primary-foreground"
                          : "hover:bg-muted",
                        !selected &&
                          isToday &&
                          "font-medium ring-1 ring-primary/50"
                      )}
                    >
                      {cell.day}
                    </button>
                  )
                })}
              </div>
            ))}
          </div>
        </div>

        {today && today !== date && (
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={() => select(today)}
          >
            {t("task.goToday")}
          </Button>
        )}
      </PopoverContent>
    </Popover>
  )
}

/** Renders `YYYY-MM-DD` as a long weekday + day + month label, anchored at UTC. */
function DayLabel({ date }: { date: string }) {
  const { locale } = useI18n()
  const label = React.useMemo(
    () =>
      new Intl.DateTimeFormat(locale === "pt" ? "pt-BR" : "en-US", {
        weekday: "long",
        day: "numeric",
        month: "long",
        timeZone: "UTC",
      }).format(parseDate(date)),
    [locale, date]
  )
  return <span className="min-w-0 truncate capitalize">{label}</span>
}

function MonthLabel({ year, month }: { year: number; month: number }) {
  const { locale } = useI18n()
  const label = React.useMemo(
    () =>
      new Intl.DateTimeFormat(locale === "pt" ? "pt-BR" : "en-US", {
        month: "long",
        year: "numeric",
        timeZone: "UTC",
      }).format(new Date(Date.UTC(year, month - 1, 1))),
    [locale, year, month]
  )
  return <>{label}</>
}

export { TaskDayPicker }
