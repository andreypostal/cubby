import * as React from "react"
import { toast } from "sonner"

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
import { useAuth } from "@/lib/auth-context"
import { useI18n } from "@/lib/i18n/context"
import type { Task } from "@/lib/api"

/**
 * Confirmation for deleting a task. A recurring task that is still running can
 * be stopped instead — that keeps everything already recorded and can be
 * resumed later — so the softer way out is offered alongside the delete.
 */
function TaskDeleteDialog({
  task,
  open,
  onOpenChange,
  onDeleted,
  onStopped,
}: {
  task: Task
  open: boolean
  onOpenChange: (open: boolean) => void
  onDeleted: (taskId: number) => void
  /** Stopping rewrites the day's occurrences, so the caller reloads the board. */
  onStopped: (task: Task) => void
}) {
  const { deleteTask, stopTask } = useAuth()
  const { t } = useI18n()

  const [busy, setBusy] = React.useState(false)

  // Onetime tasks have no recurrence to stop, and a stopped one is already
  // there — for both, deleting is the only thing left to offer.
  const stoppable = task.recurrence_type !== "once" && !task.stopped

  async function onConfirmDelete() {
    setBusy(true)
    try {
      await deleteTask(task.id)
      onOpenChange(false)
      onDeleted(task.id)
      toast.success(t("task.deleteSuccess"))
    } catch {
      toast.error(t("task.deleteError"))
    } finally {
      setBusy(false)
    }
  }

  async function onConfirmStop() {
    setBusy(true)
    try {
      const updated = await stopTask(task.id)
      onOpenChange(false)
      onStopped(updated)
      toast.success(t("task.stopSuccess"))
    } catch {
      toast.error(t("task.stopError"))
    } finally {
      setBusy(false)
    }
  }

  return (
    <AlertDialog open={open} onOpenChange={onOpenChange}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>
            {stoppable
              ? t("task.deleteOrStopTitle")
              : t("task.deleteDialogTitle")}
          </AlertDialogTitle>
          <AlertDialogDescription>
            {stoppable
              ? t("task.deleteOrStopDescription", { name: task.name })
              : t("task.deleteDialogDescription", { name: task.name })}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel disabled={busy}>
            {t("task.cancel")}
          </AlertDialogCancel>
          {stoppable && (
            <AlertDialogAction
              variant="secondary"
              disabled={busy}
              onClick={onConfirmStop}
            >
              {t("task.stopInstead")}
            </AlertDialogAction>
          )}
          <AlertDialogAction
            variant={stoppable ? "destructive" : "default"}
            disabled={busy}
            onClick={onConfirmDelete}
          >
            {stoppable ? t("task.deleteAnyway") : t("task.deleteConfirm")}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}

export { TaskDeleteDialog }
