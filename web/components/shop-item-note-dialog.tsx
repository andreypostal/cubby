import * as React from "react"

import { useResetOnOpen } from "@/hooks/use-reset-on-open"
import { Alert, AlertDescription } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Field, FieldGroup } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Spinner } from "@/components/ui/spinner"
import { useI18n } from "@/lib/i18n/context"
import type { ShopListItem } from "@/lib/api"

const NOTE_MAX_LENGTH = 255

/**
 * Writes the short note shown under an item name. The input starts on the
 * current note so an existing one can be edited, and saving it empty clears it.
 *
 * A bought item hides its note in the row, so there the dialog only reads it
 * back — nothing left to change once the thing is in the basket.
 */
function ShopItemNoteDialog({
  open,
  onOpenChange,
  item,
  onSave,
  readOnly = false,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  item: ShopListItem
  onSave: (notes: string) => Promise<void>
  readOnly?: boolean
}) {
  const { t } = useI18n()

  const [notes, setNotes] = React.useState(item.notes)
  const [saving, setSaving] = React.useState(false)
  const [hasError, setHasError] = React.useState(false)

  useResetOnOpen(open, () => {
    setNotes(item.notes)
    setHasError(false)
  })

  function handleOpenChange(next: boolean) {
    if (!saving) {
      onOpenChange(next)
    }
  }

  async function onSubmit(event: React.FormEvent) {
    event.preventDefault()

    const trimmed = notes.trim()
    if (trimmed === item.notes) {
      onOpenChange(false)
      return
    }

    setSaving(true)
    setHasError(false)
    try {
      await onSave(trimmed)
      onOpenChange(false)
    } catch {
      setHasError(true)
    } finally {
      setSaving(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>
            {readOnly ? t("list.noteViewTitle") : t("list.noteDialogTitle")}
          </DialogTitle>
          <DialogDescription>
            {readOnly
              ? t("list.noteViewDescription", { name: item.name })
              : t("list.noteDialogDescription", { name: item.name })}
          </DialogDescription>
        </DialogHeader>

        {readOnly ? (
          <>
            <p className="text-sm wrap-anywhere">{item.notes}</p>
            <DialogFooter className="mt-4">
              <Button type="button" onClick={() => onOpenChange(false)}>
                {t("list.close")}
              </Button>
            </DialogFooter>
          </>
        ) : (
          <form onSubmit={onSubmit} noValidate>
            <FieldGroup>
              {hasError && (
                <Alert variant="destructive">
                  <AlertDescription>{t("list.noteSaveError")}</AlertDescription>
                </Alert>
              )}
              <Field>
                <Input
                  autoFocus
                  maxLength={NOTE_MAX_LENGTH}
                  placeholder={t("list.notePlaceholder")}
                  value={notes}
                  onChange={(event) => setNotes(event.target.value)}
                  disabled={saving}
                />
              </Field>
            </FieldGroup>
            <DialogFooter className="mt-4">
              <Button
                type="button"
                variant="outline"
                disabled={saving}
                onClick={() => onOpenChange(false)}
              >
                {t("list.cancel")}
              </Button>
              <Button type="submit" disabled={saving}>
                {saving && <Spinner data-icon="inline-start" />}
                {saving ? t("list.saving") : t("list.save")}
              </Button>
            </DialogFooter>
          </form>
        )}
      </DialogContent>
    </Dialog>
  )
}

export { ShopItemNoteDialog }
