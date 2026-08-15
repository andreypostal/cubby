import * as React from "react"

import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog"
import { useResetOnOpen } from "@/hooks/use-reset-on-open"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Field, FieldDescription, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Spinner } from "@/components/ui/spinner"
import { useI18n } from "@/lib/i18n/context"
import { shiftDate } from "@/lib/task-board"
import type { Task } from "@/lib/api"

/** Confirms ending a recurrence today. Onetime tasks never get here. */
function TaskStopDialog({
  open,
  onOpenChange,
  taskName,
  busy,
  onConfirm,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  taskName: string
  busy: boolean
  onConfirm: () => void
}) {
  const { t } = useI18n()

  return (
    <AlertDialog open={open} onOpenChange={onOpenChange}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{t("task.stopDialogTitle")}</AlertDialogTitle>
          <AlertDialogDescription>
            {t("task.stopDialogDescription", { name: taskName })}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel disabled={busy}>
            {t("task.cancel")}
          </AlertDialogCancel>
          <AlertDialogAction disabled={busy} onClick={onConfirm}>
            {t("task.stopConfirm")}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}

/**
 * Picks the day a stopped recurrence continues from. The API refuses a resume
 * that overlaps the run that was stopped, so the picker can never go earlier
 * than the day after it ended.
 */
function TaskResumeDialog({
  open,
  onOpenChange,
  task,
  today,
  /** Where to start counting when the task never recorded an end. */
  fallbackDay,
  busy,
  onConfirm,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  task: Task
  today: string | null
  fallbackDay: string
  busy: boolean
  onConfirm: (from: string) => void
}) {
  const { t } = useI18n()
  const inputId = React.useId()

  const earliest = task.ends_on ? shiftDate(task.ends_on, 1) : (today ?? fallbackDay)
  const defaultFrom = today && today > earliest ? today : earliest

  const [from, setFrom] = React.useState(defaultFrom)

  useResetOnOpen(open, () => {
    setFrom(defaultFrom)
  })

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("task.resumeDialogTitle")}</DialogTitle>
          <DialogDescription>
            {t("task.resumeDialogDescription", { name: task.name })}
          </DialogDescription>
        </DialogHeader>

        <Field>
          <FieldLabel htmlFor={inputId}>{t("task.resumeFromLabel")}</FieldLabel>
          <Input
            id={inputId}
            type="date"
            value={from}
            min={earliest}
            onChange={(event) => setFrom(event.target.value)}
            disabled={busy}
          />
          <FieldDescription>{t("task.resumeFromHint")}</FieldDescription>
        </Field>

        <DialogFooter>
          <Button
            type="button"
            variant="outline"
            disabled={busy}
            onClick={() => onOpenChange(false)}
          >
            {t("task.cancel")}
          </Button>
          <Button
            type="button"
            disabled={busy || from < earliest}
            onClick={() => onConfirm(from)}
          >
            {busy && <Spinner data-icon="inline-start" />}
            {busy ? t("task.resumeSubmitting") : t("task.resumeConfirm")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

export { TaskResumeDialog, TaskStopDialog }
