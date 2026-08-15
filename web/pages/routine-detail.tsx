import * as React from "react"
import { useNavigate, useParams } from "react-router"
import { AnimatePresence, motion } from "motion/react"
import { HugeiconsIcon } from "@hugeicons/react"
import {
  Award01Icon,
  ArrowLeft01Icon,
  ArrowRight01Icon,
  Cancel01Icon,
  CogIcon,
  Tick02Icon,
} from "@hugeicons/core-free-icons"

import { cn } from "@/lib/utils"
import { Alert, AlertDescription } from "@/components/ui/alert"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent } from "@/components/ui/card"
import { MarkRoutineDayDialog } from "@/components/mark-routine-day-dialog"
import { RoutineSettingsDialog } from "@/components/routine-settings-dialog"
import { Separator } from "@/components/ui/separator"
import { Spinner } from "@/components/ui/spinner"
import { useAuth } from "@/lib/auth-context"
import { useI18n } from "@/lib/i18n/context"
import type { TranslationKey } from "@/lib/i18n/translations"
import {
  buildCalendarGrid,
  isDayClickable,
  monthIndex,
  WEEKDAY_KEYS,
  type CalendarCell,
} from "@/lib/routine-calendar"
import type { Routine, RoutineDayStatus, RoutineMonth } from "@/lib/api"

const DAY_INITIAL_KEYS: TranslationKey[] = [
  "day.mon",
  "day.tue",
  "day.wed",
  "day.thu",
  "day.fri",
  "day.sat",
  "day.sun",
]

const DAY_LABEL_KEYS: TranslationKey[] = [
  "day.monday",
  "day.tuesday",
  "day.wednesday",
  "day.thursday",
  "day.friday",
  "day.saturday",
  "day.sunday",
]

const STATUS_LABEL_KEYS: Record<RoutineDayStatus, TranslationKey> = {
  achieved: "goal.status.achieved",
  failed: "goal.status.failed",
  waiting: "goal.status.waiting",
  future: "goal.status.future",
  disabled: "goal.status.disabled",
}

export default function RoutineDetailPage() {
  const params = useParams<{ routineID: string }>()
  const routineId = Number(params.routineID)

  return (
    <AnimatePresence mode="wait">
      <RoutineView key={routineId} routineId={routineId} />
    </AnimatePresence>
  )
}

function RoutineView({ routineId }: { routineId: number }) {
  const { getRoutine, getRoutineMonth, subscribeRoutinesChanged } = useAuth()
  const { t } = useI18n()
  const navigate = useNavigate()

  const today = React.useMemo(() => new Date(), [])

  const [routine, setRoutine] = React.useState<Routine | null>(null)
  const [loadError, setLoadError] = React.useState(false)
  const [year, setYear] = React.useState(today.getFullYear())
  const [month, setMonth] = React.useState(today.getMonth() + 1)
  const [monthData, setMonthData] = React.useState<RoutineMonth | null>(null)
  const [monthError, setMonthError] = React.useState(false)
  const monthLoading = monthData === null && !monthError
  const [settingsOpen, setSettingsOpen] = React.useState(false)
  const [markState, setMarkState] = React.useState<{
    dateStr: string
    status: RoutineDayStatus
  } | null>(null)

  React.useEffect(() => {
    getRoutine(routineId)
      .then(setRoutine)
      .catch(() => setLoadError(true))
  }, [getRoutine, routineId])

  React.useEffect(() => {
    return subscribeRoutinesChanged((event) => {
      if (event.type === "deleted") {
        if (event.routineId === routineId) {
          navigate("/")
        }
      } else if (event.routine.id === routineId) {
        setRoutine(event.routine)
      }
    })
  }, [subscribeRoutinesChanged, routineId, navigate])

  React.useEffect(() => {
    let cancelled = false
    getRoutineMonth(routineId, year, month)
      .then((data) => {
        if (!cancelled) {
          setMonthData(data)
          setMonthError(false)
        }
      })
      .catch(() => {
        if (!cancelled) {
          setMonthError(true)
        }
      })
    return () => {
      cancelled = true
    }
  }, [getRoutineMonth, routineId, year, month])

  const statusByDate = React.useMemo(() => {
    const map = new Map<string, RoutineDayStatus>()
    monthData?.days.forEach((day) => map.set(day.day, day.status))
    return map
  }, [monthData])

  const weeks = React.useMemo(
    () => buildCalendarGrid(year, month, statusByDate),
    [year, month, statusByDate]
  )

  const currentIndex = monthIndex(year, month)
  const maxIndex = monthIndex(today.getFullYear(), today.getMonth() + 1)
  const minIndex = routine
    ? monthIndex(
        new Date(routine.created_at).getFullYear(),
        new Date(routine.created_at).getMonth() + 1
      )
    : null

  const canGoPrev = minIndex !== null && currentIndex > minIndex
  const canGoNext = currentIndex < maxIndex

  function goPrevMonth() {
    if (!canGoPrev) {
      return
    }
    setMonthData(null)
    setMonthError(false)
    setMonth((current) => {
      if (current === 1) {
        setYear((y) => y - 1)
        return 12
      }
      return current - 1
    })
  }

  function goNextMonth() {
    if (!canGoNext) {
      return
    }
    setMonthData(null)
    setMonthError(false)
    setMonth((current) => {
      if (current === 12) {
        setYear((y) => y + 1)
        return 1
      }
      return current + 1
    })
  }

  function onDayClick(cell: CalendarCell) {
    if (!isDayClickable(cell) || !cell.status) {
      return
    }
    setMarkState({ dateStr: cell.dateStr, status: cell.status })
  }

  function onMarkChanged(
    dateStr: string,
    status: RoutineDayStatus,
    points: number
  ) {
    setMonthData((current) => {
      if (!current) {
        return current
      }
      const exists = current.days.some((day) => day.day === dateStr)
      const days = exists
        ? current.days.map((day) =>
            day.day === dateStr ? { ...day, status } : day
          )
        : [...current.days, { day: dateStr, status }]
      return { ...current, days }
    })
    setRoutine((current) => (current ? { ...current, points } : current))
  }

  if (loadError) {
    return (
      <Alert variant="destructive">
        <AlertDescription>{t("goal.loadError")}</AlertDescription>
      </Alert>
    )
  }

  if (!routine) {
    return (
      <motion.div
        key="loading"
        initial={{ opacity: 0 }}
        animate={{ opacity: 1 }}
        exit={{ opacity: 0 }}
        transition={{ duration: 0.15 }}
        className="flex flex-1 items-center justify-center"
      >
        <Spinner className="size-6" />
      </motion.div>
    )
  }

  return (
    <motion.div
      key="content"
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      exit={{ opacity: 0 }}
      transition={{ duration: 0.2 }}
      className="mx-auto flex w-full max-w-2xl flex-col gap-6"
    >
      <div className="min-w-0">
        <p className="text-xs font-medium tracking-wide text-muted-foreground uppercase">
          {t("goal.eyebrow")}
        </p>
        <div className="flex items-center gap-2">
          <h1 className="min-w-0 truncate font-heading text-2xl font-semibold">
            {routine.name}
          </h1>
          <Button
            type="button"
            variant="ghost"
            size="icon-lg"
            className="shrink-0 text-muted-foreground hover:text-foreground"
            onClick={() => setSettingsOpen(true)}
          >
            <HugeiconsIcon icon={CogIcon} />
            <span className="sr-only">{t("goal.settingsLabel")}</span>
          </Button>
        </div>
        {routine.description && (
          <p className="mt-1 text-sm/relaxed text-muted-foreground">
            {routine.description}
          </p>
        )}

        <div className="mt-4 flex flex-wrap items-center gap-4">
          <div className="flex flex-col gap-1.5">
            <span className="text-xs font-medium tracking-wide text-muted-foreground uppercase">
              {t("goal.scheduledDaysLabel")}
            </span>
            <div className="flex gap-1">
              {WEEKDAY_KEYS.map((key, index) => {
                const scheduled = Boolean(routine[key])
                return (
                  <span
                    key={key}
                    title={t(DAY_LABEL_KEYS[index])}
                    className={cn(
                      "flex size-6 items-center justify-center rounded-full text-[0.65rem] font-semibold",
                      scheduled
                        ? "bg-primary text-primary-foreground"
                        : "bg-muted text-muted-foreground/50"
                    )}
                  >
                    {t(DAY_INITIAL_KEYS[index])}
                  </span>
                )
              })}
            </div>
          </div>

          <div className="flex flex-col gap-1.5">
            <span className="text-xs font-medium tracking-wide text-muted-foreground uppercase">
              {t("goal.score")}
            </span>
            <Badge variant="secondary" className="h-6 px-2.5 text-xs">
              <HugeiconsIcon icon={Award01Icon} data-icon="inline-start" />
              {t("goal.points", { points: routine.points })}
            </Badge>
          </div>
        </div>
      </div>

      <Card>
        <CardContent className="flex flex-col gap-4">
          <div className="flex items-center justify-between">
            <Button
              type="button"
              variant="ghost"
              size="icon-sm"
              disabled={!canGoPrev}
              onClick={goPrevMonth}
            >
              <HugeiconsIcon icon={ArrowLeft01Icon} />
              <span className="sr-only">{t("goal.monthPrev")}</span>
            </Button>
            <p className="text-sm font-medium capitalize">
              <MonthLabel year={year} month={month} />
            </p>
            <Button
              type="button"
              variant="ghost"
              size="icon-sm"
              disabled={!canGoNext}
              onClick={goNextMonth}
            >
              <HugeiconsIcon icon={ArrowRight01Icon} />
              <span className="sr-only">{t("goal.monthNext")}</span>
            </Button>
          </div>

          {monthError && (
            <Alert variant="destructive">
              <AlertDescription>{t("goal.monthLoadError")}</AlertDescription>
            </Alert>
          )}

          <div className="relative">
            {monthLoading && (
              <div className="absolute inset-0 z-10 flex items-center justify-center rounded-xl bg-card/60">
                <Spinner className="size-5" />
              </div>
            )}

            <div className="hidden sm:block">
              <div className="grid grid-cols-7 gap-1.5">
                {DAY_INITIAL_KEYS.map((key, index) => (
                  <div
                    key={key}
                    className="flex h-6 items-center justify-center text-xs font-medium text-muted-foreground"
                  >
                    {t(DAY_INITIAL_KEYS[index])}
                  </div>
                ))}
              </div>
              <div className="mt-1 flex flex-col gap-1.5">
                {weeks.map((week, weekIndex) => (
                  <div key={weekIndex} className="grid grid-cols-7 gap-1.5">
                    {week.map((cell) => (
                      <DayCell key={cell.dateStr} cell={cell} onClick={onDayClick} />
                    ))}
                  </div>
                ))}
              </div>
            </div>

            <div className="flex flex-col gap-1.5 sm:hidden">
              {weeks
                .flat()
                .filter((cell) => cell.inMonth)
                .map((cell) => (
                  <DayRow key={cell.dateStr} cell={cell} onClick={onDayClick} />
                ))}
            </div>
          </div>

          <Legend />
        </CardContent>
      </Card>

      <RoutineSettingsDialog
        open={settingsOpen}
        onOpenChange={setSettingsOpen}
        routine={routine}
        onSaved={setRoutine}
        onDeleted={() => navigate("/")}
      />

      <MarkRoutineDayDialog
        open={markState !== null}
        onOpenChange={(open) => {
          if (!open) {
            setMarkState(null)
          }
        }}
        routineId={routineId}
        dateStr={markState?.dateStr ?? null}
        status={markState?.status ?? null}
        onChanged={onMarkChanged}
      />
    </motion.div>
  )
}

function MonthLabel({ year, month }: { year: number; month: number }) {
  const { locale } = useI18n()
  const label = React.useMemo(
    () =>
      new Intl.DateTimeFormat(locale === "pt" ? "pt-BR" : "en-US", {
        month: "long",
        year: "numeric",
      }).format(new Date(year, month - 1, 1)),
    [locale, year, month]
  )
  return <>{label}</>
}

function DayCell({
  cell,
  onClick,
}: {
  cell: CalendarCell
  onClick: (cell: CalendarCell) => void
}) {
  const { t } = useI18n()
  const clickable = isDayClickable(cell)

  return (
    <button
      type="button"
      disabled={!clickable}
      onClick={() => onClick(cell)}
      title={cell.status ? t(STATUS_LABEL_KEYS[cell.status]) : undefined}
      className={cn(
        "relative flex aspect-square items-center justify-center rounded-xl text-sm font-medium transition-colors",
        !cell.inMonth && "text-muted-foreground/30",
        cell.inMonth && cell.status === "disabled" && "text-muted-foreground/40",
        cell.inMonth &&
          cell.status === "future" &&
          "border border-dashed border-primary/30 text-foreground",
        cell.inMonth &&
          cell.status === "waiting" &&
          (cell.isToday
            ? "cursor-pointer bg-blue-500/15 text-foreground ring-2 ring-blue-500 hover:bg-blue-500/25"
            : "cursor-pointer bg-amber-500/10 text-foreground ring-1 ring-amber-500/50 hover:bg-amber-500/20"),
        cell.inMonth &&
          cell.status === "achieved" &&
          (cell.isToday
            ? "cursor-pointer bg-emerald-500/15 text-emerald-700 ring-2 ring-emerald-600 hover:bg-emerald-500/25 dark:text-emerald-400 dark:ring-emerald-400"
            : "cursor-pointer bg-emerald-500/15 text-emerald-700 ring-1 ring-emerald-500/40 hover:bg-emerald-500/25 dark:text-emerald-400"),
        cell.inMonth &&
          cell.status === "failed" &&
          (cell.isToday
            ? "cursor-pointer bg-red-500/10 text-red-700 ring-2 ring-red-600 hover:bg-red-500/20 dark:text-red-400 dark:ring-red-400"
            : "cursor-pointer bg-red-500/10 text-red-700 ring-1 ring-red-500/30 hover:bg-red-500/20 dark:text-red-400")
      )}
    >
      {cell.status === "achieved" ? (
        <HugeiconsIcon icon={Tick02Icon} size={16} />
      ) : cell.status === "failed" ? (
        <HugeiconsIcon icon={Cancel01Icon} size={16} />
      ) : (
        cell.day
      )}
    </button>
  )
}

function DayRow({
  cell,
  onClick,
}: {
  cell: CalendarCell
  onClick: (cell: CalendarCell) => void
}) {
  const { t, locale } = useI18n()
  const clickable = isDayClickable(cell)

  const weekdayLabel = React.useMemo(
    () =>
      new Intl.DateTimeFormat(locale === "pt" ? "pt-BR" : "en-US", {
        weekday: "short",
      }).format(cell.date),
    [cell.date, locale]
  )

  return (
    <button
      type="button"
      disabled={!clickable}
      onClick={() => onClick(cell)}
      className={cn(
        "flex items-center gap-3 rounded-lg bg-card px-3 py-2.5 text-left ring-1 ring-foreground/10 transition-colors",
        clickable && "hover:bg-muted"
      )}
    >
      <span
        className={cn(
          "flex size-9 shrink-0 items-center justify-center rounded-full text-sm font-semibold",
          cell.status === "disabled" && "bg-muted text-muted-foreground/50",
          cell.status === "future" &&
            "border border-dashed border-primary/30 text-foreground",
          cell.status === "waiting" &&
            (cell.isToday
              ? "bg-blue-500/15 text-foreground ring-2 ring-blue-500"
              : "bg-amber-500/10 text-foreground ring-1 ring-amber-500/50"),
          cell.status === "achieved" &&
            (cell.isToday
              ? "bg-emerald-500/15 text-emerald-700 ring-2 ring-emerald-600 dark:text-emerald-400 dark:ring-emerald-400"
              : "bg-emerald-500/15 text-emerald-700 ring-1 ring-emerald-500/40 dark:text-emerald-400"),
          cell.status === "failed" &&
            (cell.isToday
              ? "bg-red-500/10 text-red-700 ring-2 ring-red-600 dark:text-red-400 dark:ring-red-400"
              : "bg-red-500/10 text-red-700 ring-1 ring-red-500/30 dark:text-red-400")
        )}
      >
        {cell.status === "achieved" ? (
          <HugeiconsIcon icon={Tick02Icon} size={16} />
        ) : cell.status === "failed" ? (
          <HugeiconsIcon icon={Cancel01Icon} size={16} />
        ) : (
          cell.day
        )}
      </span>
      <div className="min-w-0 flex-1">
        <p className="truncate text-sm font-medium capitalize">{weekdayLabel}</p>
        {cell.status && (
          <p className="text-[0.65rem] text-muted-foreground">
            {t(STATUS_LABEL_KEYS[cell.status])}
          </p>
        )}
      </div>
    </button>
  )
}

function Legend() {
  const { t } = useI18n()
  const items: Array<{ status: RoutineDayStatus; dot: string }> = [
    { status: "waiting", dot: "bg-amber-500" },
    { status: "achieved", dot: "bg-emerald-500" },
    { status: "failed", dot: "bg-red-500" },
    { status: "future", dot: "bg-primary/40" },
  ]

  return (
    <>
      <Separator />
      <div className="flex flex-wrap items-center gap-x-4 gap-y-1.5">
        {items.map((item) => (
          <div key={item.status} className="flex items-center gap-1.5">
            <span className={cn("size-2 rounded-full", item.dot)} />
            <span className="text-xs text-muted-foreground">
              {t(STATUS_LABEL_KEYS[item.status])}
            </span>
          </div>
        ))}
      </div>
    </>
  )
}
