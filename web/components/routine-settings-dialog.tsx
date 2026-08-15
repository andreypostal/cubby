import * as React from "react"
import { toast } from "sonner"
import { HugeiconsIcon } from "@hugeicons/react"
import { Delete02Icon } from "@hugeicons/core-free-icons"

import { useResetOnOpen } from "@/hooks/use-reset-on-open"
import { Alert, AlertDescription } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { ConfirmDestructiveDialog } from "@/components/confirm-destructive-dialog"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Field, FieldDescription, FieldGroup, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Separator } from "@/components/ui/separator"
import { Spinner } from "@/components/ui/spinner"
import { Textarea } from "@/components/ui/textarea"
import { useAuth } from "@/lib/auth-context"
import { useI18n } from "@/lib/i18n/context"
import type { Routine } from "@/lib/api"

function RoutineSettingsDialog({
  open,
  onOpenChange,
  routine,
  onSaved,
  onDeleted,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  routine: Routine
  onSaved: (routine: Routine) => void
  onDeleted: () => void
}) {
  const { updateRoutine, deleteRoutine } = useAuth()
  const { t } = useI18n()

  const [name, setName] = React.useState(routine.name)
  const [description, setDescription] = React.useState(routine.description)
  const [submitting, setSubmitting] = React.useState(false)
  const [hasError, setHasError] = React.useState(false)
  const [deleteOpen, setDeleteOpen] = React.useState(false)

  useResetOnOpen(open, () => {
    setName(routine.name)
    setDescription(routine.description)
    setHasError(false)
  })

  const hasChanges =
    name.trim() !== routine.name || description.trim() !== routine.description

  function handleOpenChange(next: boolean) {
    if (!submitting) {
      onOpenChange(next)
    }
  }

  function onCancel() {
    setName(routine.name)
    setDescription(routine.description)
    setHasError(false)
  }

  async function onSubmit(event: React.FormEvent) {
    event.preventDefault()
    if (!name.trim() || !hasChanges) {
      return
    }

    setSubmitting(true)
    setHasError(false)
    try {
      const updated = await updateRoutine(routine.id, {
        name: name.trim(),
        description: description.trim(),
        pinned: routine.pinned,
        active: routine.active,
        on_monday: routine.on_monday,
        on_tuesday: routine.on_tuesday,
        on_wednesday: routine.on_wednesday,
        on_thursday: routine.on_thursday,
        on_friday: routine.on_friday,
        on_saturday: routine.on_saturday,
        on_sunday: routine.on_sunday,
      })
      onSaved(updated)
      onOpenChange(false)
      toast.success(t("goal.saveSuccess"))
    } catch {
      setHasError(true)
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <>
      <Dialog open={open} onOpenChange={handleOpenChange}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("goal.settingsTitle")}</DialogTitle>
            <DialogDescription>{t("goal.settingsDescription")}</DialogDescription>
          </DialogHeader>
          <form onSubmit={onSubmit} noValidate>
            <FieldGroup>
              {hasError && (
                <Alert variant="destructive">
                  <AlertDescription>{t("goal.saveError")}</AlertDescription>
                </Alert>
              )}
              <Field>
                <Input
                  autoFocus
                  placeholder={t("goal.namePlaceholder")}
                  value={name}
                  onChange={(event) => setName(event.target.value)}
                  disabled={submitting}
                />
              </Field>
              <Field>
                <FieldLabel>{t("goal.descriptionLabel")}</FieldLabel>
                <Textarea
                  placeholder={t("goal.descriptionPlaceholder")}
                  value={description}
                  onChange={(event) => setDescription(event.target.value)}
                  disabled={submitting}
                />
              </Field>
              <FieldDescription>{t("goal.daysLockedNote")}</FieldDescription>
            </FieldGroup>
            <DialogFooter className="mt-4">
              <Button
                type="button"
                variant="outline"
                disabled={submitting || !hasChanges}
                onClick={onCancel}
              >
                {t("goal.cancel")}
              </Button>
              {hasChanges && (
                <Button type="submit" disabled={submitting}>
                  {submitting && <Spinner data-icon="inline-start" />}
                  {submitting ? t("goal.saving") : t("goal.save")}
                </Button>
              )}
            </DialogFooter>
          </form>

          <Separator className="my-1" />

          <div className="flex flex-col gap-2 rounded-lg border border-destructive/30 p-3">
            <p className="text-xs/relaxed text-muted-foreground">
              {t("goal.dangerZoneDescription")}
            </p>
            <Button
              type="button"
              variant="destructive"
              size="sm"
              className="self-start"
              disabled={submitting}
              onClick={() => setDeleteOpen(true)}
            >
              <HugeiconsIcon icon={Delete02Icon} data-icon="inline-start" />
              {t("goal.deleteGoal")}
            </Button>
          </div>
        </DialogContent>
      </Dialog>

      <ConfirmDestructiveDialog
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
        title={t("goal.deleteDialogTitle")}
        description={t("goal.deleteDialogDescription", { name: routine.name })}
        errorMessage={t("goal.deleteError")}
        cancelLabel={t("goal.cancel")}
        confirmLabel={t("goal.deleteConfirmAction")}
        busyLabel={t("goal.deleteDeleting")}
        confirmPlaceholder={t("goal.deleteConfirmPlaceholder")}
        onConfirm={async () => {
          await deleteRoutine(routine.id)
          toast.success(t("goal.deleteSuccess"))
          setDeleteOpen(false)
          onOpenChange(false)
          onDeleted()
        }}
      />
    </>
  )
}

export { RoutineSettingsDialog }
