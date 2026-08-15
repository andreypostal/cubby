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
import { Field, FieldGroup } from "@/components/ui/field"
import { IconPicker } from "@/components/icon-picker"
import { Input } from "@/components/ui/input"
import { Separator } from "@/components/ui/separator"
import { Spinner } from "@/components/ui/spinner"
import { useAuth } from "@/lib/auth-context"
import { useI18n } from "@/lib/i18n/context"
import type { ShopList } from "@/lib/api"

function ListSettingsDialog({
  open,
  onOpenChange,
  list,
  onSaved,
  onDeleted,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  list: { id: number; name: string; icon: string }
  onSaved: (list: ShopList) => void
  onDeleted: () => void
}) {
  const { updateList, deleteList } = useAuth()
  const { t } = useI18n()

  const [name, setName] = React.useState(list.name)
  const [icon, setIcon] = React.useState(list.icon)
  const [submitting, setSubmitting] = React.useState(false)
  const [hasError, setHasError] = React.useState(false)
  const [deleteOpen, setDeleteOpen] = React.useState(false)

  useResetOnOpen(open, () => {
    setName(list.name)
    setIcon(list.icon)
    setHasError(false)
  })

  const hasChanges = name.trim() !== list.name || icon !== list.icon

  function handleOpenChange(next: boolean) {
    if (!submitting) {
      onOpenChange(next)
    }
  }

  function onCancel() {
    setName(list.name)
    setIcon(list.icon)
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
      const updated = await updateList(list.id, name.trim(), icon)
      onSaved(updated)
      onOpenChange(false)
      toast.success(t("list.saveSuccess"))
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
            <DialogTitle>{t("list.settingsTitle")}</DialogTitle>
            <DialogDescription>
              {t("list.settingsDescription")}
            </DialogDescription>
          </DialogHeader>
          <form onSubmit={onSubmit} noValidate>
            <FieldGroup>
              {hasError && (
                <Alert variant="destructive">
                  <AlertDescription>{t("list.saveError")}</AlertDescription>
                </Alert>
              )}
              <Field orientation="horizontal">
                <IconPicker value={icon} onChange={setIcon} disabled={submitting} />
                <Input
                  autoFocus
                  placeholder={t("list.namePlaceholder")}
                  value={name}
                  onChange={(event) => setName(event.target.value)}
                  disabled={submitting}
                />
              </Field>
            </FieldGroup>
            <DialogFooter className="mt-4">
              <Button
                type="button"
                variant="outline"
                disabled={submitting || !hasChanges}
                onClick={onCancel}
              >
                {t("list.cancel")}
              </Button>
              {hasChanges && (
                <Button type="submit" disabled={submitting}>
                  {submitting && <Spinner data-icon="inline-start" />}
                  {submitting ? t("list.saving") : t("list.save")}
                </Button>
              )}
            </DialogFooter>
          </form>

          <Separator className="my-1" />

          <div className="flex flex-col gap-2 rounded-lg border border-destructive/30 p-3">
            <p className="text-xs/relaxed text-muted-foreground">
              {t("list.dangerZoneDescription")}
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
              {t("list.deleteList")}
            </Button>
          </div>
        </DialogContent>
      </Dialog>

      <ConfirmDestructiveDialog
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
        title={t("list.deleteDialogTitle")}
        description={t("list.deleteDialogDescription", { name: list.name })}
        errorMessage={t("list.deleteError")}
        cancelLabel={t("list.cancel")}
        confirmLabel={t("list.deleteConfirmAction")}
        busyLabel={t("list.deleteDeleting")}
        confirmPlaceholder={t("list.deleteConfirmPlaceholder")}
        onConfirm={async () => {
          await deleteList(list.id)
          toast.success(t("list.deleteSuccess"))
          setDeleteOpen(false)
          onOpenChange(false)
          onDeleted()
        }}
      />
    </>
  )
}

export { ListSettingsDialog }
