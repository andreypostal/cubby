import * as React from "react"
import { toast } from "sonner"
import { HugeiconsIcon } from "@hugeicons/react"
import {
  Add01Icon,
  ArrowLeft01Icon,
  ArrowRight01Icon,
  Cancel01Icon,
  Delete02Icon,
  InformationCircleIcon,
  ListViewIcon,
  MoreHorizontalIcon,
  PinIcon,
  Tick02Icon,
} from "@hugeicons/core-free-icons"

import { cn } from "@/lib/utils"
import { Alert, AlertDescription } from "@/components/ui/alert"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { CreateTaskDrawer } from "@/components/create-task-drawer"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Skeleton } from "@/components/ui/skeleton"
import { TaskDayPicker } from "@/components/task-day-picker"
import { TaskDeleteDialog } from "@/components/task-delete-dialog"
import { TaskDetailsDrawer } from "@/components/task-details-drawer"
import { TaskListDrawer } from "@/components/task-list-drawer"
import { useAuth } from "@/lib/auth-context"
import { useI18n } from "@/lib/i18n/context"
import { getTaskIcon } from "@/lib/task-icons"
import {
  buildSections,
  isSettled,
  SECTION_LABEL_KEYS,
  shiftDate,
  STATUS_BADGE_CLASS,
  STATUS_LABEL_KEYS,
} from "@/lib/task-board"
import type {
  Task,
  TaskBoard,
  TaskDayStatus,
  TaskDayView,
  TaskOccurrence,
} from "@/lib/api"

export default function TasksBoardPage() {
  const { workspace } = useAuth()

  // Remounts (and re-fetches from a clean state) whenever the workspace changes.
  return <TasksBoard key={workspace?.id} />
}

function TasksBoard() {
  const { getTaskBoard, updateTaskBoard, getTaskDay } = useAuth()
  const { t } = useI18n()

  const [board, setBoard] = React.useState<TaskBoard | null>(null)
  const [date, setDate] = React.useState<string | null>(null)
  const [day, setDay] = React.useState<TaskDayView | null>(null)
  const [loadError, setLoadError] = React.useState(false)
  const [createOpen, setCreateOpen] = React.useState(false)
  const [listOpen, setListOpen] = React.useState(false)
  const [detailsTaskId, setDetailsTaskId] = React.useState<number | null>(null)

  React.useEffect(() => {
    let cancelled = false

    getTaskBoard()
      .then(async (loaded) => {
        const browserTimezone = Intl.DateTimeFormat().resolvedOptions().timeZone
        // A fresh board defaults to UTC, which makes "today" wrong for most
        // users. Adopt the browser's zone once, then never touch it again so a
        // deliberate choice is not overwritten from another device.
        if (
          browserTimezone &&
          loaded.timezone === "UTC" &&
          browserTimezone !== "UTC"
        ) {
          return updateTaskBoard(browserTimezone).catch(() => loaded)
        }
        return loaded
      })
      .then((loaded) => {
        if (!cancelled) {
          setBoard(loaded)
          setDate(loaded.today)
        }
      })
      .catch(() => {
        if (!cancelled) {
          setLoadError(true)
        }
      })

    return () => {
      cancelled = true
    }
  }, [getTaskBoard, updateTaskBoard])

  const loadDay = React.useCallback(
    (target: string) => {
      return getTaskDay(target)
        .then((loaded) => {
          setDay(loaded)
          setLoadError(false)
        })
        .catch(() => setLoadError(true))
    },
    [getTaskDay]
  )

  React.useEffect(() => {
    if (!date) {
      return
    }
    let cancelled = false
    getTaskDay(date)
      .then((loaded) => {
        if (!cancelled) {
          setDay(loaded)
          setLoadError(false)
        }
      })
      .catch(() => {
        if (!cancelled) {
          setLoadError(true)
        }
      })
    return () => {
      cancelled = true
    }
  }, [getTaskDay, date])

  function goToDate(next: string) {
    setDay(null)
    setDetailsTaskId(null)
    setDate(next)
  }

  function onOccurrenceChanged(taskId: number, status: TaskDayStatus) {
    setDay((current) =>
      current
        ? {
            ...current,
            occurrences: current.occurrences.map((occurrence) =>
              occurrence.task.id === taskId
                ? { ...occurrence, status }
                : occurrence
            ),
          }
        : current
    )
  }

  function onTaskUpdated(task: Task) {
    setDay((current) =>
      current
        ? {
            ...current,
            occurrences: current.occurrences.map((occurrence) =>
              occurrence.task.id === task.id
                ? { ...occurrence, task }
                : occurrence
            ),
          }
        : current
    )
  }

  function onTaskDeleted(taskId: number) {
    setDetailsTaskId((current) => (current === taskId ? null : current))
    setDay((current) =>
      current
        ? {
            ...current,
            occurrences: current.occurrences.filter(
              (occurrence) => occurrence.task.id !== taskId
            ),
          }
        : current
    )
  }

  const sections = React.useMemo(
    () => buildSections(day?.occurrences ?? []),
    [day]
  )

  const detailsOccurrence =
    day?.occurrences.find(
      (occurrence) => occurrence.task.id === detailsTaskId
    ) ?? null

  // Stopping a task can drop it from the day being viewed, which would leave the
  // drawer showing something the board no longer has.
  if (day && detailsTaskId !== null && !detailsOccurrence) {
    setDetailsTaskId(null)
  }

  const total = day?.occurrences.length ?? 0
  const done = day?.occurrences.filter(isSettled).length ?? 0
  const isToday = Boolean(board && date === board.today)

  return (
    <div className="mx-auto flex w-full max-w-2xl flex-col gap-5">
      <div className="flex flex-col gap-3">
        {/* The day label is long enough in most locales that it needs the whole
            row on a phone, so the actions live in the controls row below it. */}
        <div className="min-w-0">
          <p className="text-xs font-medium tracking-wide text-muted-foreground uppercase">
            {t("task.boardEyebrow")}
          </p>
          <h1 className="min-w-0 font-heading text-2xl font-semibold">
            {date ? (
              <TaskDayPicker
                date={date}
                today={board?.today ?? null}
                onSelect={goToDate}
              />
            ) : (
              <Skeleton className="h-8 w-48" />
            )}
          </h1>
        </div>

        <div className="flex flex-wrap items-center gap-2">
          <div className="flex items-center gap-1">
            <Button
              type="button"
              variant="outline"
              size="icon-sm"
              disabled={!date}
              onClick={() => date && goToDate(shiftDate(date, -1))}
            >
              <HugeiconsIcon icon={ArrowLeft01Icon} />
              <span className="sr-only">{t("task.prevDay")}</span>
            </Button>
            <Button
              type="button"
              variant="outline"
              size="icon-sm"
              disabled={!date}
              onClick={() => date && goToDate(shiftDate(date, 1))}
            >
              <HugeiconsIcon icon={ArrowRight01Icon} />
              <span className="sr-only">{t("task.nextDay")}</span>
            </Button>
          </div>

          {board && !isToday && (
            <Button
              type="button"
              variant="ghost"
              size="sm"
              onClick={() => goToDate(board.today)}
            >
              {t("task.goToday")}
            </Button>
          )}

          {day && total > 0 && (
            <Badge variant="secondary" className="h-7 px-2.5 text-xs">
              {t("task.progress", { done, total })}
            </Badge>
          )}

          <div className="ml-auto flex shrink-0 items-center gap-2">
            <Button
              type="button"
              variant="outline"
              size="sm"
              // Icon-only below `sm`, so it stays square instead of keeping the
              // padding that only makes sense next to a label.
              className="max-sm:w-8 max-sm:px-0!"
              title={t("task.allTasks")}
              onClick={() => setListOpen(true)}
            >
              <HugeiconsIcon icon={ListViewIcon} data-icon="inline-start" />
              <span className="hidden sm:inline">{t("task.allTasks")}</span>
              <span className="sr-only sm:hidden">{t("task.allTasks")}</span>
            </Button>
            <Button
              type="button"
              size="sm"
              className="max-sm:w-8 max-sm:px-0!"
              title={t("task.newTask")}
              disabled={!date}
              onClick={() => setCreateOpen(true)}
            >
              <HugeiconsIcon icon={Add01Icon} data-icon="inline-start" />
              <span className="hidden sm:inline">{t("task.newTask")}</span>
              <span className="sr-only sm:hidden">{t("task.newTask")}</span>
            </Button>
          </div>
        </div>

        {day && total > 0 && (
          <div
            className="h-1.5 overflow-hidden rounded-full bg-muted"
            role="progressbar"
            aria-valuemin={0}
            aria-valuemax={total}
            aria-valuenow={done}
          >
            <div
              className="h-full rounded-full bg-primary transition-[width] duration-300"
              style={{ width: `${total === 0 ? 0 : (done / total) * 100}%` }}
            />
          </div>
        )}
      </div>

      {loadError && (
        <Alert variant="destructive">
          <AlertDescription>{t("task.loadError")}</AlertDescription>
        </Alert>
      )}

      {!day && !loadError && (
        <div className="flex flex-col gap-2">
          {Array.from({ length: 4 }).map((_, index) => (
            <Skeleton key={index} className="h-16 w-full rounded-2xl" />
          ))}
        </div>
      )}

      {day && total === 0 && !loadError && (
        <div className="flex flex-col items-center gap-3 rounded-2xl border border-dashed border-border px-6 py-12 text-center">
          <p className="text-sm text-muted-foreground">{t("task.empty")}</p>
          <Button
            type="button"
            variant="outline"
            onClick={() => setCreateOpen(true)}
          >
            <HugeiconsIcon icon={Add01Icon} data-icon="inline-start" />
            {t("task.newTask")}
          </Button>
        </div>
      )}

      {sections.map((section) => (
        <section key={section.type} className="flex flex-col gap-2">
          <div className="flex items-baseline gap-2">
            <h2 className="font-heading text-sm font-semibold">
              {t(SECTION_LABEL_KEYS[section.type])}
            </h2>
            <span className="text-xs text-muted-foreground tabular-nums">
              {section.occurrences.filter(isSettled).length}/
              {section.occurrences.length}
            </span>
          </div>
          <ul className="flex flex-col gap-2">
            {section.occurrences.map((occurrence) => (
              <TaskRow
                key={occurrence.task.id}
                occurrence={occurrence}
                onChanged={onOccurrenceChanged}
                onUpdated={onTaskUpdated}
                onDeleted={onTaskDeleted}
                onReload={() => date && loadDay(date)}
                onOpenDetails={() => setDetailsTaskId(occurrence.task.id)}
              />
            ))}
          </ul>
        </section>
      ))}

      <TaskListDrawer
        open={listOpen}
        onOpenChange={setListOpen}
        today={board?.today ?? null}
        onTasksChanged={() => date && loadDay(date)}
      />

      <TaskDetailsDrawer
        occurrence={detailsOccurrence}
        open={detailsTaskId !== null}
        today={board?.today ?? null}
        onOpenChange={(next) => {
          if (!next) {
            setDetailsTaskId(null)
          }
        }}
        onUpdated={onTaskUpdated}
        onReload={() => date && loadDay(date)}
        onDeleted={onTaskDeleted}
      />

      {date && (
        <CreateTaskDrawer
          open={createOpen}
          onOpenChange={setCreateOpen}
          today={board?.today ?? date}
          onCreated={() => {
            toast.success(t("task.createSuccess"))
            loadDay(date)
          }}
        />
      )}
    </div>
  )
}

function TaskRow({
  occurrence,
  onChanged,
  onUpdated,
  onDeleted,
  onReload,
  onOpenDetails,
}: {
  occurrence: TaskOccurrence
  onChanged: (taskId: number, status: TaskDayStatus) => void
  onUpdated: (task: Task) => void
  onDeleted: (taskId: number) => void
  /** Stopping a task rewrites the day's occurrences server-side. */
  onReload: () => void
  onOpenDetails: () => void
}) {
  const { markTaskDay, clearTaskDay } = useAuth()
  const { t } = useI18n()

  const [busy, setBusy] = React.useState(false)
  const [deleteOpen, setDeleteOpen] = React.useState(false)

  const { task, day, status } = occurrence
  const markable = status !== "future"
  const completed = status === "completed"

  async function mark(next: "completed" | "skipped" | "failed") {
    if (!markable || busy) {
      return
    }
    setBusy(true)
    try {
      const result = await markTaskDay(task.id, day, next)
      onChanged(task.id, result.status)
    } catch {
      toast.error(t("task.markError"))
    } finally {
      setBusy(false)
    }
  }

  async function clear() {
    if (!markable || busy) {
      return
    }
    setBusy(true)
    try {
      const result = await clearTaskDay(task.id, day)
      onChanged(task.id, result.status)
    } catch {
      toast.error(t("task.markError"))
    } finally {
      setBusy(false)
    }
  }

  return (
    <li
      className={cn(
        "rounded-2xl bg-card ring-1 ring-foreground/10 transition-opacity",
        busy && "opacity-60"
      )}
    >
      <div className="flex items-center gap-3 p-3">
        <button
          type="button"
          disabled={!markable || busy}
          onClick={() => (completed ? clear() : mark("completed"))}
          title={
            markable
              ? completed
                ? t("task.clearMark")
                : t("task.markCompleted")
              : t("task.futureHint")
          }
          className={cn(
            "flex size-9 shrink-0 items-center justify-center rounded-full transition-colors",
            completed
              ? "bg-emerald-500/15 text-emerald-700 ring-1 ring-emerald-500/50 dark:text-emerald-400"
              : "bg-muted text-muted-foreground ring-1 ring-foreground/10",
            markable ? "cursor-pointer hover:bg-muted/70" : "opacity-50"
          )}
        >
          <HugeiconsIcon
            icon={completed ? Tick02Icon : getTaskIcon(task.icon)}
            size={18}
          />
        </button>

        <button
          type="button"
          onClick={onOpenDetails}
          title={t("task.openDetails")}
          className="min-w-0 flex-1 cursor-pointer text-left"
        >
          <div className="flex min-w-0 items-center gap-1.5">
            {task.pinned && (
              <HugeiconsIcon
                icon={PinIcon}
                size={14}
                className="shrink-0 text-muted-foreground"
                aria-label={t("task.pinned")}
              />
            )}
            <span
              className={cn(
                "min-w-0 truncate text-sm font-medium underline-offset-4 hover:underline",
                completed && "text-muted-foreground line-through"
              )}
            >
              {task.name}
            </span>
          </div>
          {task.description && (
            <span className="mt-0.5 block truncate text-xs text-muted-foreground">
              {task.description}
            </span>
          )}
        </button>

        <span
          className={cn(
            "hidden shrink-0 rounded-full px-2 py-0.5 text-[0.65rem] font-medium sm:inline",
            STATUS_BADGE_CLASS[status]
          )}
        >
          {t(STATUS_LABEL_KEYS[status])}
        </span>

        <DropdownMenu>
          <DropdownMenuTrigger
            render={
              <Button
                type="button"
                variant="ghost"
                size="icon-sm"
                className="shrink-0 text-muted-foreground"
              />
            }
          >
            <HugeiconsIcon icon={MoreHorizontalIcon} />
            <span className="sr-only">{t("task.moreActions")}</span>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" className="w-auto min-w-48">
            <DropdownMenuItem
              disabled={!markable}
              onClick={() => mark("completed")}
            >
              <HugeiconsIcon icon={Tick02Icon} />
              {t("task.markCompleted")}
            </DropdownMenuItem>
            <DropdownMenuItem
              disabled={!markable}
              onClick={() => mark("skipped")}
            >
              <HugeiconsIcon icon={ArrowRight01Icon} />
              {t("task.markSkipped")}
            </DropdownMenuItem>
            <DropdownMenuItem
              disabled={!markable}
              onClick={() => mark("failed")}
            >
              <HugeiconsIcon icon={Cancel01Icon} />
              {t("task.markFailed")}
            </DropdownMenuItem>
            <DropdownMenuItem disabled={!markable} onClick={clear}>
              {t("task.clearMark")}
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem onClick={onOpenDetails}>
              <HugeiconsIcon icon={InformationCircleIcon} />
              {t("task.openDetails")}
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem
              variant="destructive"
              onClick={() => setDeleteOpen(true)}
            >
              <HugeiconsIcon icon={Delete02Icon} />
              {t("task.delete")}
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>

      <TaskDeleteDialog
        task={task}
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
        onDeleted={onDeleted}
        onStopped={(updated) => {
          onUpdated(updated)
          onReload()
        }}
      />
    </li>
  )
}
