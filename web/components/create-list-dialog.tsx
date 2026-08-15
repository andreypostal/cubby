import * as React from "react"

import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { useResetOnOpen } from "@/hooks/use-reset-on-open"
import { Alert, AlertDescription } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { Field, FieldGroup } from "@/components/ui/field"
import { IconPicker } from "@/components/icon-picker"
import { Input } from "@/components/ui/input"
import { Spinner } from "@/components/ui/spinner"
import { useAuth } from "@/lib/auth-context"
import { useI18n } from "@/lib/i18n/context"
import { DEFAULT_LIST_ICON } from "@/lib/list-icons"
import type { ShopList } from "@/lib/api"

function CreateListDialog({
  open,
  onOpenChange,
  onCreated,
  defaultName = "",
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  onCreated: (list: ShopList) => void
  /** Suggested name, filled in each time the dialog opens. */
  defaultName?: string
}) {
  const { createList } = useAuth()
  const { t } = useI18n()

  const [name, setName] = React.useState(defaultName)
  const [icon, setIcon] = React.useState(DEFAULT_LIST_ICON)
  const [submitting, setSubmitting] = React.useState(false)
  const [hasError, setHasError] = React.useState(false)

  useResetOnOpen(open, () => {
    setName(defaultName)
    setIcon(DEFAULT_LIST_ICON)
    setHasError(false)
  })

  async function onSubmit(event: React.FormEvent) {
    event.preventDefault()
    if (!name.trim()) {
      return
    }

    setSubmitting(true)
    setHasError(false)
    try {
      const list = await createList(name.trim(), icon)
      onCreated(list)
      onOpenChange(false)
    } catch {
      setHasError(true)
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent forceOverlay>
        <DialogHeader>
          <DialogTitle>{t("list.createTitle")}</DialogTitle>
          <DialogDescription>{t("list.createDescription")}</DialogDescription>
        </DialogHeader>
        <form onSubmit={onSubmit} noValidate>
          <FieldGroup>
            {hasError && (
              <Alert variant="destructive">
                <AlertDescription>{t("list.createError")}</AlertDescription>
              </Alert>
            )}
            <Field orientation="horizontal">
              <IconPicker
                value={icon}
                onChange={setIcon}
                disabled={submitting}
              />
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
              disabled={submitting}
              onClick={() => onOpenChange(false)}
            >
              {t("list.cancel")}
            </Button>
            <Button type="submit" disabled={submitting}>
              {submitting && <Spinner data-icon="inline-start" />}
              {submitting ? t("list.createSubmitting") : t("list.createSubmit")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

export { CreateListDialog }
