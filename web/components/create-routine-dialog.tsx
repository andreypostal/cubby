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
import { Field, FieldGroup, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Spinner } from "@/components/ui/spinner"
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group"
import { useAuth } from "@/lib/auth-context"
import { useI18n } from "@/lib/i18n/context"
import type { TranslationKey } from "@/lib/i18n/translations"
import type { Routine } from "@/lib/api"

const DAYS = [
  { key: "on_monday", initialKey: "day.mon", labelKey: "day.monday" },
  { key: "on_tuesday", initialKey: "day.tue", labelKey: "day.tuesday" },
  { key: "on_wednesday", initialKey: "day.wed", labelKey: "day.wednesday" },
  { key: "on_thursday", initialKey: "day.thu", labelKey: "day.thursday" },
  { key: "on_friday", initialKey: "day.fri", labelKey: "day.friday" },
  { key: "on_saturday", initialKey: "day.sat", labelKey: "day.saturday" },
  { key: "on_sunday", initialKey: "day.sun", labelKey: "day.sunday" },
] as const satisfies ReadonlyArray<{
  key: string
  initialKey: TranslationKey
  labelKey: TranslationKey
}>

type DayKey = (typeof DAYS)[number]["key"]

const ALL_DAYS: DayKey[] = DAYS.map((day) => day.key)

function CreateRoutineDialog({
  open,
  onOpenChange,
  onCreated,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  onCreated: (routine: Routine) => void
}) {
  const { createRoutine } = useAuth()
  const { t } = useI18n()

  const [name, setName] = React.useState("")
  const [days, setDays] = React.useState<DayKey[]>(ALL_DAYS)
  const [submitting, setSubmitting] = React.useState(false)
  const [hasError, setHasError] = React.useState(false)

  useResetOnOpen(open, () => {
    setName("")
    setDays(ALL_DAYS)
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
      const routine = await createRoutine({
        name: name.trim(),
        description: "",
        pinned: false,
        on_monday: days.includes("on_monday"),
        on_tuesday: days.includes("on_tuesday"),
        on_wednesday: days.includes("on_wednesday"),
        on_thursday: days.includes("on_thursday"),
        on_friday: days.includes("on_friday"),
        on_saturday: days.includes("on_saturday"),
        on_sunday: days.includes("on_sunday"),
      })
      onCreated(routine)
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
          <DialogTitle>{t("goal.createTitle")}</DialogTitle>
          <DialogDescription>{t("goal.createDescription")}</DialogDescription>
        </DialogHeader>
        <form onSubmit={onSubmit} noValidate>
          <FieldGroup>
            {hasError && (
              <Alert variant="destructive">
                <AlertDescription>{t("goal.createError")}</AlertDescription>
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
              <FieldLabel>{t("goal.daysLabel")}</FieldLabel>
              <ToggleGroup
                multiple
                value={days}
                onValueChange={(value) => setDays(value as DayKey[])}
                variant="outline"
                disabled={submitting}
              >
                {DAYS.map((day) => (
                  <ToggleGroupItem
                    key={day.key}
                    value={day.key}
                    aria-label={t(day.labelKey)}
                    className="aria-pressed:border-primary aria-pressed:bg-primary aria-pressed:text-primary-foreground aria-pressed:hover:bg-primary/90"
                  >
                    {t(day.initialKey)}
                  </ToggleGroupItem>
                ))}
              </ToggleGroup>
            </Field>
          </FieldGroup>
          <DialogFooter className="mt-4">
            <Button
              type="button"
              variant="outline"
              disabled={submitting}
              onClick={() => onOpenChange(false)}
            >
              {t("goal.cancel")}
            </Button>
            <Button type="submit" disabled={submitting}>
              {submitting && <Spinner data-icon="inline-start" />}
              {submitting ? t("goal.createSubmitting") : t("goal.createSubmit")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

export { CreateRoutineDialog }
