import * as React from "react"
import { toast } from "sonner"
import { HugeiconsIcon } from "@hugeicons/react"
import {
  Delete02Icon,
  MoreHorizontalIcon,
  PauseIcon,
  PinIcon,
  PlayIcon,
  Search01Icon,
} from "@hugeicons/core-free-icons"

import { useResetOnOpen } from "@/hooks/use-reset-on-open"
import { cn } from "@/lib/utils"
import { Alert, AlertDescription } from "@/components/ui/alert"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Input } from "@/components/ui/input"
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet"
import { Skeleton } from "@/components/ui/skeleton"
import { TaskDeleteDialog } from "@/components/task-delete-dialog"
import {
  TaskResumeDialog,
  TaskStopDialog,
} from "@/components/task-schedule-dialogs"
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group"
import { useAuth } from "@/lib/auth-context"
import { useI18n } from "@/lib/i18n/context"
import type { Locale, TranslationKey } from "@/lib/i18n/translations"
import { getTaskIcon } from "@/lib/task-icons"
import { describeSchedule, formatTaskDate } from "@/lib/task-board"
import type { TaskListFilter, TaskListResponse, TaskSummary } from "@/lib/api"

/**
 * Workspace-wide overview of task definitions, split into what is running and
 * what was stopped. The board answers "what do I do today?"; this answers "what
 * did I sign myself up for?", and is where a task gets stopped or resumed.
 */
function TaskListDrawer({
  open,
  onOpenChange,
  today,
  onTasksChanged,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  today: string | null
  /** Stopping, resuming or deleting rewrites the day the board is showing. */
  onTasksChanged: () => void
}) {
  const { listTasks } = useAuth()
  const { t } = useI18n()

  const [filter, setFilter] = React.useState<TaskListFilter>("active")
  // The filter the loaded slice belongs to is kept with it, so switching tabs
  // shows a skeleton instead of the previous tab's rows for a frame.
  const [view, setView] = React.useState<{
    filter: TaskListFilter
    data: TaskListResponse
  } | null>(null)
  const [loadError, setLoadError] = React.useState(false)
  const [reloadKey, setReloadKey] = React.useState(0)
  const [query, setQuery] = React.useState("")

  // Fetching on open (rather than once) keeps the list honest: the board itself
  // creates and stops tasks while the drawer sits closed.
  React.useEffect(() => {
    if (!open) {
      return
    }

    let cancelled = false
    listTasks(filter)
      .then((loaded) => {
        if (!cancelled) {
          setView({ filter, data: loaded })
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
  }, [open, filter, listTasks, reloadKey])

  useResetOnOpen(open, () => {
    setQuery("")
  })

  function reload() {
    setReloadKey((current) => current + 1)
  }

  function onMutated() {
    reload()
    onTasksChanged()
  }

  // Counts cover the whole board, so the previous slice still labels the tabs
  // correctly while the next one is on its way.
  const data = view?.data ?? null
  const loading = !loadError && view?.filter !== filter

  const normalizedQuery = query.trim().toLowerCase()
  const tasks = (data?.tasks ?? []).filter((task) =>
    normalizedQuery === ""
      ? true
      : `${task.name} ${task.description}`
          .toLowerCase()
          .includes(normalizedQuery)
  )

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent
        side="right"
        className="data-[side=right]:w-full data-[side=right]:sm:max-w-xl"
      >
        <SheetHeader className="gap-3 border-b border-border/70 pr-14">
          <div className="flex flex-col gap-1.5">
            <SheetTitle>{t("task.listTitle")}</SheetTitle>
            <SheetDescription>{t("task.listDescription")}</SheetDescription>
          </div>

          <ToggleGroup
            value={[filter]}
            onValueChange={(value) => {
              const next = value[0] as TaskListFilter | undefined
              if (next) {
                setFilter(next)
              }
            }}
            variant="outline"
            className="w-full"
          >
            <FilterTab
              value="active"
              label={t("task.filterActive")}
              count={data?.active_count}
            />
            <FilterTab
              value="stopped"
              label={t("task.filterStopped")}
              count={data?.stopped_count}
            />
          </ToggleGroup>

          <div className="relative">
            <HugeiconsIcon
              icon={Search01Icon}
              size={16}
              className="pointer-events-none absolute top-1/2 left-3 -translate-y-1/2 text-muted-foreground"
            />
            <Input
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              placeholder={t("task.listSearchPlaceholder")}
              className="pl-9"
            />
          </div>
        </SheetHeader>

        <div className="flex min-h-0 flex-1 flex-col gap-2 overflow-y-auto px-6 py-5">
          {loadError && (
            <Alert variant="destructive">
              <AlertDescription className="flex flex-wrap items-center gap-3">
                {t("task.listError")}
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  onClick={reload}
                >
                  {t("task.listRetry")}
                </Button>
              </AlertDescription>
            </Alert>
          )}

          {loading &&
            !loadError &&
            Array.from({ length: 4 }).map((_, index) => (
              <Skeleton key={index} className="h-20 w-full rounded-2xl" />
            ))}

          {!loading && !loadError && tasks.length === 0 && (
            <p className="rounded-2xl border border-dashed border-border px-6 py-12 text-center text-sm text-muted-foreground">
              {normalizedQuery !== ""
                ? t("task.listNoMatches", { query: query.trim() })
                : filter === "stopped"
                  ? t("task.listEmptyStopped")
                  : t("task.listEmptyActive")}
            </p>
          )}

          {!loading && !loadError && tasks.length > 0 && (
            <ul className="flex flex-col gap-2">
              {tasks.map((task) => (
                <TaskListRow
                  key={task.id}
                  task={task}
                  today={today}
                  onMutated={onMutated}
                />
              ))}
            </ul>
          )}
        </div>
      </SheetContent>
    </Sheet>
  )
}

function FilterTab({
  value,
  label,
  count,
}: {
  value: TaskListFilter
  label: string
  count?: number
}) {
  return (
    <ToggleGroupItem
      value={value}
      className="flex-1 aria-pressed:border-primary aria-pressed:bg-primary aria-pressed:text-primary-foreground aria-pressed:hover:bg-primary/90"
    >
      {label}
      {count !== undefined && (
        <span className="ml-1.5 tabular-nums opacity-70">{count}</span>
      )}
    </ToggleGroupItem>
  )
}

function TaskListRow({
  task,
  today,
  onMutated,
}: {
  task: TaskSummary
  today: string | null
  onMutated: () => void
}) {
  const { stopTask, resumeTask } = useAuth()
  const { t, locale } = useI18n()

  const [busy, setBusy] = React.useState(false)
  const [stopOpen, setStopOpen] = React.useState(false)
  const [resumeOpen, setResumeOpen] = React.useState(false)
  const [deleteOpen, setDeleteOpen] = React.useState(false)

  // Onetime tasks have no recurrence to stop, and an older stopped run always
  // overlaps the one that continued it — the API refuses both.
  const stoppable = task.recurrence_type !== "once" && !task.stopped
  const resumable = task.resumable

  async function onConfirmStop() {
    setBusy(true)
    try {
      await stopTask(task.id)
      setStopOpen(false)
      onMutated()
      toast.success(t("task.stopSuccess"))
    } catch {
      toast.error(t("task.stopError"))
    } finally {
      setBusy(false)
    }
  }

  async function onConfirmResume(from: string) {
    setBusy(true)
    try {
      await resumeTask(task.id, from)
      setResumeOpen(false)
      onMutated()
      toast.success(t("task.resumeSuccess"))
    } catch {
      toast.error(t("task.resumeError"))
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
      <div className="flex items-start gap-3 p-3">
        <span
          className={cn(
            "flex size-9 shrink-0 items-center justify-center rounded-full ring-1 ring-foreground/10",
            task.stopped
              ? "bg-muted/50 text-muted-foreground/70"
              : "bg-muted text-muted-foreground"
          )}
        >
          <HugeiconsIcon icon={getTaskIcon(task.icon)} size={18} />
        </span>

        <div className="flex min-w-0 flex-1 flex-col gap-1">
          <div className="flex min-w-0 items-center gap-1.5">
            {task.pinned && (
              <HugeiconsIcon
                icon={PinIcon}
                size={14}
                className="shrink-0 text-muted-foreground"
                aria-label={t("task.pinned")}
              />
            )}
            <span className="min-w-0 truncate text-sm font-medium">
              {task.name}
            </span>
          </div>

          {task.description && (
            <span className="truncate text-xs text-muted-foreground">
              {task.description}
            </span>
          )}

          <div className="flex flex-wrap items-center gap-1.5">
            <Badge variant="secondary" className="text-[0.65rem] font-medium">
              {describeSchedule(task, t, locale, { shortWeekdays: true })}
            </Badge>
            <span className="text-xs text-muted-foreground tabular-nums">
              <TaskTiming task={task} today={today} locale={locale} t={t} />
            </span>
          </div>
        </div>

        <div className="flex shrink-0 items-center gap-1">
          {stoppable && (
            <Button
              type="button"
              variant="outline"
              size="icon-sm"
              disabled={busy}
              title={t("task.stop")}
              onClick={() => setStopOpen(true)}
            >
              <HugeiconsIcon icon={PauseIcon} />
              <span className="sr-only">{t("task.stop")}</span>
            </Button>
          )}
          {resumable && (
            <Button
              type="button"
              size="icon-sm"
              disabled={busy}
              title={t("task.resume")}
              onClick={() => setResumeOpen(true)}
            >
              <HugeiconsIcon icon={PlayIcon} />
              <span className="sr-only">{t("task.resume")}</span>
            </Button>
          )}

          <DropdownMenu>
            <DropdownMenuTrigger
              render={
                <Button
                  type="button"
                  variant="ghost"
                  size="icon-sm"
                  className="text-muted-foreground"
                />
              }
            >
              <HugeiconsIcon icon={MoreHorizontalIcon} />
              <span className="sr-only">{t("task.moreActions")}</span>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="w-auto min-w-48">
              {stoppable && (
                <DropdownMenuItem onClick={() => setStopOpen(true)}>
                  <HugeiconsIcon icon={PauseIcon} />
                  {t("task.stop")}
                </DropdownMenuItem>
              )}
              {resumable && (
                <DropdownMenuItem onClick={() => setResumeOpen(true)}>
                  <HugeiconsIcon icon={PlayIcon} />
                  {t("task.resume")}
                </DropdownMenuItem>
              )}
              {(stoppable || resumable) && <DropdownMenuSeparator />}
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
      </div>

      <TaskStopDialog
        open={stopOpen}
        onOpenChange={setStopOpen}
        taskName={task.name}
        busy={busy}
        onConfirm={onConfirmStop}
      />

      <TaskResumeDialog
        open={resumeOpen}
        onOpenChange={setResumeOpen}
        task={task}
        today={today}
        fallbackDay={task.starts_on}
        busy={busy}
        onConfirm={onConfirmResume}
      />

      <TaskDeleteDialog
        task={task}
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
        onDeleted={onMutated}
        onStopped={onMutated}
      />
    </li>
  )
}

/**
 * The one line that says where a task stands: when it fires next while it runs,
 * and when it was stopped once it does not.
 */
function TaskTiming({
  task,
  today,
  locale,
  t,
}: {
  task: TaskSummary
  today: string | null
  locale: Locale
  t: (key: TranslationKey, vars?: Record<string, string | number>) => string
}) {
  // The year is noise while it matches the one the board is on.
  function label(date: string) {
    const sameYear = today?.slice(0, 4) === date.slice(0, 4)
    return formatTaskDate(date, locale, sameYear ? { year: undefined } : {})
  }

  if (task.stopped) {
    return (
      <>
        {task.ends_on
          ? t("task.stoppedOn", { date: label(task.ends_on) })
          : t("task.stoppedLabel")}
        {!task.resumable &&
          task.recurrence_type !== "once" &&
          ` · ${t("task.superseded")}`}
      </>
    )
  }

  if (!task.next_occurrence) {
    return <>{t("task.noNextOccurrence")}</>
  }

  if (today && task.next_occurrence === today) {
    return <>{t("task.nextToday")}</>
  }

  return <>{t("task.nextOn", { date: label(task.next_occurrence) })}</>

}

export { TaskListDrawer }
