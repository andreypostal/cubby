import * as React from "react"
import { toast } from "sonner"
import { HugeiconsIcon } from "@hugeicons/react"
import {
  Delete02Icon,
  PauseIcon,
  PencilEdit02Icon,
  PinIcon,
  PinOffIcon,
  PlayIcon,
} from "@hugeicons/core-free-icons"

import { useResetOnOpen } from "@/hooks/use-reset-on-open"
import { cn } from "@/lib/utils"
import { Alert, AlertDescription } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import {
  Field,
  FieldDescription,
  FieldGroup,
  FieldLabel,
} from "@/components/ui/field"
import { IconPicker } from "@/components/icon-picker"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { MarkdownEditor, MarkdownView } from "@/components/markdown-editor"
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet"
import { Spinner } from "@/components/ui/spinner"
import { TaskDeleteDialog } from "@/components/task-delete-dialog"
import {
  TaskResumeDialog,
  TaskStopDialog,
} from "@/components/task-schedule-dialogs"
import { useAuth } from "@/lib/auth-context"
import { useI18n } from "@/lib/i18n/context"
import type { TranslationKey } from "@/lib/i18n/translations"
import { getTaskIcon, TASK_ICONS } from "@/lib/task-icons"
import {
  describeSchedule,
  formatTaskDate,
  STATUS_BADGE_CLASS,
  STATUS_LABEL_KEYS,
} from "@/lib/task-board"
import type { ApiError, Task, TaskOccurrence } from "@/lib/api"

const STRATEGY_LABEL_KEYS: Record<string, TranslationKey> = {
  previous: "task.strategy.previous",
  next: "task.strategy.next",
  closest: "task.strategy.closest",
  none: "task.strategy.none",
}

/**
 * Right-side drawer with everything about a single task: the day's status, its
 * schedule, the full instructions, and the actions that act on the definition
 * (edit, pin, stop/resume, delete) rather than on one day.
 */
function TaskDetailsDrawer({
  occurrence,
  open,
  today,
  onOpenChange,
  onUpdated,
  onReload,
  onDeleted,
}: {
  occurrence: TaskOccurrence | null
  open: boolean
  today: string | null
  onOpenChange: (open: boolean) => void
  onUpdated: (task: Task) => void
  /** Stopping or resuming rewrites the day's occurrences server-side. */
  onReload: () => void
  onDeleted: (taskId: number) => void
}) {
  const { t } = useI18n()

  // The board drops the occurrence as soon as the drawer starts closing, so the
  // last one is kept around to render through the exit animation.
  const [snapshot, setSnapshot] = React.useState(occurrence)
  if (occurrence && occurrence !== snapshot) {
    setSnapshot(occurrence)
  }

  const [editing, setEditing] = React.useState(false)

  useResetOnOpen(open, () => {
    setEditing(false)
  })

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent
        side="right"
        className="data-[side=right]:w-full data-[side=right]:sm:max-w-xl"
      >
        {snapshot && (
          <DrawerBody
            key={snapshot.task.id}
            occurrence={snapshot}
            editing={editing}
            today={today}
            onEditingChange={setEditing}
            onClose={() => onOpenChange(false)}
            onUpdated={onUpdated}
            onReload={onReload}
            onDeleted={onDeleted}
            t={t}
          />
        )}
      </SheetContent>
    </Sheet>
  )
}

function DrawerBody({
  occurrence,
  editing,
  today,
  onEditingChange,
  onClose,
  onUpdated,
  onReload,
  onDeleted,
  t,
}: {
  occurrence: TaskOccurrence
  editing: boolean
  today: string | null
  onEditingChange: (editing: boolean) => void
  onClose: () => void
  onUpdated: (task: Task) => void
  onReload: () => void
  onDeleted: (taskId: number) => void
  t: (key: TranslationKey, vars?: Record<string, string | number>) => string
}) {
  const { updateTask, stopTask, resumeTask } = useAuth()

  const { task, day, status } = occurrence

  const [busy, setBusy] = React.useState(false)
  const [error, setError] = React.useState<string | null>(null)
  const [deleteOpen, setDeleteOpen] = React.useState(false)
  const [stopOpen, setStopOpen] = React.useState(false)
  const [resumeOpen, setResumeOpen] = React.useState(false)

  const [name, setName] = React.useState(task.name)
  const [icon, setIcon] = React.useState(task.icon)
  const [description, setDescription] = React.useState(task.description)
  const [instructions, setInstructions] = React.useState(task.instructions)
  const [pinned, setPinned] = React.useState(task.pinned)
  // tiptap owns its document once mounted, so the editor is remounted whenever
  // edition restarts from the stored instructions.
  const [formKey, setFormKey] = React.useState(0)

  useResetOnOpen(editing, () => {
    setName(task.name)
    setIcon(task.icon)
    setDescription(task.description)
    setInstructions(task.instructions)
    setPinned(task.pinned)
    setError(null)
    setFormKey((current) => current + 1)
  })

  // Onetime tasks have no recurrence to stop or continue.
  const stoppable = task.recurrence_type !== "once"

  const canSubmit = name.trim().length >= 3 && !busy

  async function onSubmit(event: React.FormEvent) {
    event.preventDefault()
    if (!canSubmit) {
      return
    }

    setBusy(true)
    setError(null)
    try {
      const updated = await updateTask(task.id, {
        name: name.trim(),
        icon,
        description: description.trim(),
        instructions: instructions.trim(),
        pinned,
      })
      onUpdated(updated)
      onEditingChange(false)
      toast.success(t("task.updateSuccess"))
    } catch (err) {
      setError((err as ApiError)?.message ?? t("task.updateError"))
    } finally {
      setBusy(false)
    }
  }

  async function onTogglePin() {
    if (busy) {
      return
    }
    setBusy(true)
    setError(null)
    try {
      const updated = await updateTask(task.id, {
        name: task.name,
        icon: task.icon,
        description: task.description,
        instructions: task.instructions,
        pinned: !task.pinned,
      })
      onUpdated(updated)
      toast.success(
        updated.pinned ? t("task.pinSuccess") : t("task.unpinSuccess")
      )
    } catch (err) {
      setError((err as ApiError)?.message ?? t("task.updateError"))
    } finally {
      setBusy(false)
    }
  }

  async function onConfirmStop() {
    setBusy(true)
    try {
      const updated = await stopTask(task.id)
      onUpdated(updated)
      onReload()
      setStopOpen(false)
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
      // Resuming creates a brand new task, so the drawer's own task is gone as
      // the active one — reload the day and step out of the way.
      onReload()
      setResumeOpen(false)
      onClose()
      toast.success(t("task.resumeSuccess"))
    } catch {
      toast.error(t("task.resumeError"))
    } finally {
      setBusy(false)
    }
  }

  return (
    <>
      <SheetHeader className="border-b border-border/70 pr-14">
        <p className="text-xs font-medium tracking-wide text-muted-foreground uppercase">
          {editing ? t("task.editTitle") : t("task.detailsEyebrow")}
        </p>
        <div className="flex items-center gap-2.5">
          <span className="flex size-9 shrink-0 items-center justify-center rounded-full bg-muted text-muted-foreground ring-1 ring-foreground/10">
            <HugeiconsIcon icon={getTaskIcon(task.icon)} size={18} />
          </span>
          <SheetTitle className="min-w-0 flex-1 text-base">
            <span className="flex min-w-0 items-center gap-1.5">
              {task.pinned && (
                <HugeiconsIcon
                  icon={PinIcon}
                  size={14}
                  className="shrink-0 text-muted-foreground"
                  aria-label={t("task.pinned")}
                />
              )}
              <span className="min-w-0 truncate">{task.name}</span>
            </span>
          </SheetTitle>
        </div>
        <SheetDescription>
          {editing ? t("task.editDescription") : t("task.detailsDescription")}
        </SheetDescription>
      </SheetHeader>

      {editing ? (
        <form
          onSubmit={onSubmit}
          noValidate
          className="flex min-h-0 flex-1 flex-col"
        >
          <div className="min-h-0 flex-1 overflow-y-auto px-6 py-5">
            <FieldGroup className="gap-5">
              {error && (
                <Alert variant="destructive">
                  <AlertDescription>{error}</AlertDescription>
                </Alert>
              )}

              <Field>
                <FieldLabel htmlFor="task-edit-name">
                  {t("task.nameLabel")}
                </FieldLabel>
                <div className="flex items-center gap-2">
                  <IconPicker
                    value={icon}
                    onChange={setIcon}
                    disabled={busy}
                    icons={TASK_ICONS}
                    resolveIcon={getTaskIcon}
                  />
                  <Input
                    id="task-edit-name"
                    autoFocus
                    placeholder={t("task.namePlaceholder")}
                    value={name}
                    onChange={(event) => setName(event.target.value)}
                    disabled={busy}
                  />
                </div>
              </Field>

              <Field>
                <FieldLabel htmlFor="task-edit-description">
                  {t("task.descriptionLabel")}
                </FieldLabel>
                <Input
                  id="task-edit-description"
                  placeholder={t("task.descriptionPlaceholder")}
                  value={description}
                  maxLength={250}
                  onChange={(event) => setDescription(event.target.value)}
                  disabled={busy}
                />
              </Field>

              <Field>
                <FieldLabel>{t("task.instructionsLabel")}</FieldLabel>
                <MarkdownEditor
                  key={formKey}
                  value={instructions}
                  onChange={setInstructions}
                  disabled={busy}
                  placeholder={t("task.instructionsPlaceholder")}
                />
                <FieldDescription>
                  {t("task.instructionsHint")}
                </FieldDescription>
              </Field>

              <Field orientation="horizontal">
                <Checkbox
                  id="task-edit-pinned"
                  checked={pinned}
                  onCheckedChange={(checked) => setPinned(checked === true)}
                  disabled={busy}
                />
                <div className="grid gap-1">
                  <Label htmlFor="task-edit-pinned">
                    {t("task.pinnedLabel")}
                  </Label>
                  <FieldDescription>{t("task.pinnedHint")}</FieldDescription>
                </div>
              </Field>
            </FieldGroup>
          </div>

          <SheetFooter className="flex-row justify-end border-t border-border/70">
            <Button
              type="button"
              variant="outline"
              disabled={busy}
              onClick={() => onEditingChange(false)}
            >
              {t("task.cancel")}
            </Button>
            <Button type="submit" disabled={!canSubmit}>
              {busy && <Spinner data-icon="inline-start" />}
              {busy ? t("task.saveSubmitting") : t("task.saveSubmit")}
            </Button>
          </SheetFooter>
        </form>
      ) : (
        <div className="flex min-h-0 flex-1 flex-col gap-5 overflow-y-auto px-6 py-5">
          {error && (
            <Alert variant="destructive">
              <AlertDescription>{error}</AlertDescription>
            </Alert>
          )}

          <div className="flex flex-wrap gap-2">
            <Button
              type="button"
              variant="outline"
              size="sm"
              disabled={busy}
              onClick={() => onEditingChange(true)}
            >
              <HugeiconsIcon icon={PencilEdit02Icon} data-icon="inline-start" />
              {t("task.edit")}
            </Button>
            <Button
              type="button"
              variant="outline"
              size="sm"
              disabled={busy}
              onClick={onTogglePin}
            >
              <HugeiconsIcon
                icon={task.pinned ? PinOffIcon : PinIcon}
                data-icon="inline-start"
              />
              {task.pinned ? t("task.unpin") : t("task.pin")}
            </Button>
            {stoppable && (
              <Button
                type="button"
                variant="outline"
                size="sm"
                disabled={busy}
                onClick={() =>
                  task.stopped ? setResumeOpen(true) : setStopOpen(true)
                }
              >
                <HugeiconsIcon
                  icon={task.stopped ? PlayIcon : PauseIcon}
                  data-icon="inline-start"
                />
                {task.stopped ? t("task.resume") : t("task.stop")}
              </Button>
            )}
            <Button
              type="button"
              variant="outline"
              size="sm"
              disabled={busy}
              className="text-destructive hover:bg-destructive/10 hover:text-destructive"
              onClick={() => setDeleteOpen(true)}
            >
              <HugeiconsIcon icon={Delete02Icon} data-icon="inline-start" />
              {t("task.delete")}
            </Button>
          </div>

          <p
            className={cn(
              "text-sm",
              task.description ? "text-foreground" : "text-muted-foreground"
            )}
          >
            {task.description || t("task.noDescription")}
          </p>

          <dl className="grid grid-cols-1 gap-x-4 gap-y-3 rounded-2xl bg-muted/40 p-4 sm:grid-cols-2">
            <InfoRow label={t("task.statusLabel")}>
              <span
                className={cn(
                  "inline-flex rounded-full px-2 py-0.5 text-[0.7rem] font-medium",
                  STATUS_BADGE_CLASS[status]
                )}
              >
                {t(STATUS_LABEL_KEYS[status])}
              </span>
            </InfoRow>
            <InfoRow label={t("task.dayLabel")}>
              <DateText date={day} />
            </InfoRow>
            <InfoRow label={t("task.scheduleLabel")}>
              <ScheduleSummary task={task} />
            </InfoRow>
            {(task.recurrence_type === "monthly" ||
              task.recurrence_type === "annual") && (
              <InfoRow label={t("task.strategyLabel")}>
                {t(
                  STRATEGY_LABEL_KEYS[task.business_day_strategy] ??
                    "task.strategy.none"
                )}
              </InfoRow>
            )}
            <InfoRow
              label={
                task.recurrence_type === "once"
                  ? t("task.dueOnLabel")
                  : t("task.startsOnLabel")
              }
            >
              <DateText date={task.starts_on} />
            </InfoRow>
            {task.ends_on && (
              <InfoRow label={t("task.endsOnLabel")}>
                <DateText date={task.ends_on} />
              </InfoRow>
            )}
            {task.stopped && (
              <InfoRow label={t("task.stoppedLabel")}>
                {t("task.stoppedYes")}
              </InfoRow>
            )}
            <InfoRow label={t("task.createdAtLabel")}>
              <DateText date={task.created_at.slice(0, 10)} />
            </InfoRow>
          </dl>

          <div className="flex flex-col gap-2">
            <h3 className="font-heading text-sm font-semibold">
              {t("task.instructionsLabel")}
            </h3>
            {task.instructions ? (
              <MarkdownView value={task.instructions} />
            ) : (
              <p className="text-sm text-muted-foreground">
                {t("task.noInstructions")}
              </p>
            )}
          </div>
        </div>
      )}

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
        fallbackDay={day}
        busy={busy}
        onConfirm={onConfirmResume}
      />

      <TaskDeleteDialog
        task={task}
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
        onDeleted={(taskId) => {
          onDeleted(taskId)
          onClose()
        }}
        onStopped={(updated) => {
          onUpdated(updated)
          onReload()
        }}
      />
    </>
  )
}

function InfoRow({
  label,
  children,
}: {
  label: string
  children: React.ReactNode
}) {
  return (
    <div className="flex min-w-0 flex-col gap-1">
      <dt className="text-[0.7rem] font-medium tracking-wide text-muted-foreground uppercase">
        {label}
      </dt>
      <dd className="text-sm">{children}</dd>
    </div>
  )
}

function ScheduleSummary({ task }: { task: Task }) {
  const { t, locale } = useI18n()
  return <>{describeSchedule(task, t, locale)}</>
}

/** Renders a `YYYY-MM-DD` string in the active locale, anchored at UTC. */
function DateText({ date }: { date: string }) {
  const { locale } = useI18n()
  return (
    <span className="tabular-nums">{formatTaskDate(date, locale)}</span>
  )
}

export { TaskDetailsDrawer }
