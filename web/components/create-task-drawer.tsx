import * as React from "react"

import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet"
import { useResetOnOpen } from "@/hooks/use-reset-on-open"
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
import { MarkdownEditor } from "@/components/markdown-editor"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Spinner } from "@/components/ui/spinner"
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group"
import { useAuth } from "@/lib/auth-context"
import { useI18n } from "@/lib/i18n/context"
import type { TranslationKey } from "@/lib/i18n/translations"
import { monthNames } from "@/lib/task-board"
import { DEFAULT_TASK_ICON, getTaskIcon, TASK_ICONS } from "@/lib/task-icons"
import type {
  ApiError,
  Task,
  TaskBusinessDayStrategy,
  TaskRecurrenceType,
} from "@/lib/api"

const RECURRENCES = [
  { value: "once", labelKey: "task.recurrence.once" },
  { value: "annual", labelKey: "task.recurrence.annual" },
  { value: "monthly", labelKey: "task.recurrence.monthly" },
  { value: "weekly", labelKey: "task.recurrence.weekly" },
  { value: "daily", labelKey: "task.recurrence.daily" },
] as const satisfies ReadonlyArray<{
  value: TaskRecurrenceType
  labelKey: TranslationKey
}>

const STRATEGIES = [
  { value: "previous", labelKey: "task.strategy.previous" },
  { value: "next", labelKey: "task.strategy.next" },
  { value: "closest", labelKey: "task.strategy.closest" },
  { value: "none", labelKey: "task.strategy.none" },
] as const satisfies ReadonlyArray<{
  value: TaskBusinessDayStrategy
  labelKey: TranslationKey
}>

// API weekdays are 0 = Sunday .. 6 = Saturday, but the picker reads Monday-first.
const WEEKDAYS = [
  { value: 1, initialKey: "day.mon", labelKey: "day.monday" },
  { value: 2, initialKey: "day.tue", labelKey: "day.tuesday" },
  { value: 3, initialKey: "day.wed", labelKey: "day.wednesday" },
  { value: 4, initialKey: "day.thu", labelKey: "day.thursday" },
  { value: 5, initialKey: "day.fri", labelKey: "day.friday" },
  { value: 6, initialKey: "day.sat", labelKey: "day.saturday" },
  { value: 0, initialKey: "day.sun", labelKey: "day.sunday" },
] as const satisfies ReadonlyArray<{
  value: number
  initialKey: TranslationKey
  labelKey: TranslationKey
}>

/** The month a `YYYY-MM-DD` string falls in, as the API's 1-based number. */
function monthOf(date: string): string {
  return String(Number(date.slice(5, 7)))
}

function CreateTaskDrawer({
  open,
  onOpenChange,
  today,
  onCreated,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  today: string
  onCreated: (task: Task) => void
}) {
  const { createTask } = useAuth()
  const { t, locale } = useI18n()

  // Dropping index 0 lines the options up with their values, which are the
  // API's own 1-based months.
  const monthItems = React.useMemo(
    () =>
      monthNames(locale)
        .slice(1)
        .map((label, index) => ({ label, value: String(index + 1) })),
    [locale]
  )

  const [name, setName] = React.useState("")
  const [icon, setIcon] = React.useState(DEFAULT_TASK_ICON)
  const [description, setDescription] = React.useState("")
  const [instructions, setInstructions] = React.useState("")
  const [pinned, setPinned] = React.useState(false)
  const [recurrence, setRecurrence] = React.useState<TaskRecurrenceType>("once")
  const [weekdays, setWeekdays] = React.useState<string[]>(["1"])
  const [dayOfMonth, setDayOfMonth] = React.useState("1")
  const [monthOfYear, setMonthOfYear] = React.useState(monthOf(today))
  const [strategy, setStrategy] =
    React.useState<TaskBusinessDayStrategy>("previous")
  const [startsOn, setStartsOn] = React.useState(today)
  const [submitting, setSubmitting] = React.useState(false)
  const [error, setError] = React.useState<string | null>(null)
  // Remounts the editor on reset: tiptap owns its document once mounted, so a
  // prop change alone would not clear it.
  const [formKey, setFormKey] = React.useState(0)

  useResetOnOpen(open, () => {
    setName("")
    setIcon(DEFAULT_TASK_ICON)
    setDescription("")
    setInstructions("")
    setPinned(false)
    setRecurrence("once")
    setWeekdays(["1"])
    setDayOfMonth("1")
    setMonthOfYear(monthOf(today))
    setStrategy("previous")
    setStartsOn(today)
    setError(null)
    setFormKey((current) => current + 1)
  })

  const canSubmit = name.trim().length >= 3 && !submitting

  async function onSubmit(event: React.FormEvent) {
    event.preventDefault()
    if (!canSubmit) {
      return
    }

    setSubmitting(true)
    setError(null)
    try {
      const task = await createTask({
        name: name.trim(),
        icon,
        description: description.trim(),
        instructions: instructions.trim(),
        pinned,
        recurrence_type: recurrence,
        starts_on: startsOn || undefined,
        // The API rejects fields that don't belong to the chosen recurrence.
        ...(recurrence === "weekly"
          ? { weekdays: weekdays.map(Number).sort((a, b) => a - b) }
          : {}),
        ...(recurrence === "monthly"
          ? {
              day_of_month: Number(dayOfMonth),
              business_day_strategy: strategy,
            }
          : {}),
        ...(recurrence === "annual"
          ? {
              month_of_year: Number(monthOfYear),
              day_of_month: Number(dayOfMonth),
              business_day_strategy: strategy,
            }
          : {}),
      })
      onCreated(task)
      onOpenChange(false)
    } catch (err) {
      setError((err as ApiError)?.message ?? t("task.createError"))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent
        side="right"
        className="data-[side=right]:w-full data-[side=right]:sm:max-w-xl"
      >
        <SheetHeader className="border-b border-border/70">
          <SheetTitle>{t("task.createTitle")}</SheetTitle>
          <SheetDescription>{t("task.createDescription")}</SheetDescription>
        </SheetHeader>

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
                <FieldLabel htmlFor="task-name">
                  {t("task.nameLabel")}
                </FieldLabel>
                <div className="flex items-center gap-2">
                  <IconPicker
                    value={icon}
                    onChange={setIcon}
                    disabled={submitting}
                    icons={TASK_ICONS}
                    resolveIcon={getTaskIcon}
                  />
                  <Input
                    id="task-name"
                    autoFocus
                    placeholder={t("task.namePlaceholder")}
                    value={name}
                    onChange={(event) => setName(event.target.value)}
                    disabled={submitting}
                  />
                </div>
              </Field>

              <Field>
                <FieldLabel htmlFor="task-description">
                  {t("task.descriptionLabel")}
                </FieldLabel>
                <Input
                  id="task-description"
                  placeholder={t("task.descriptionPlaceholder")}
                  value={description}
                  maxLength={250}
                  onChange={(event) => setDescription(event.target.value)}
                  disabled={submitting}
                />
              </Field>

              <Field>
                <FieldLabel>{t("task.recurrenceLabel")}</FieldLabel>
                <ToggleGroup
                  value={[recurrence]}
                  onValueChange={(value) => {
                    const next = value[0] as TaskRecurrenceType | undefined
                    if (next) {
                      setRecurrence(next)
                    }
                  }}
                  variant="outline"
                  disabled={submitting}
                  className="flex-wrap"
                >
                  {RECURRENCES.map((option) => (
                    <ToggleGroupItem
                      key={option.value}
                      value={option.value}
                      className="aria-pressed:border-primary aria-pressed:bg-primary aria-pressed:text-primary-foreground aria-pressed:hover:bg-primary/90"
                    >
                      {t(option.labelKey)}
                    </ToggleGroupItem>
                  ))}
                </ToggleGroup>
              </Field>

              {recurrence === "weekly" && (
                <Field>
                  <FieldLabel>{t("task.weekdaysLabel")}</FieldLabel>
                  <ToggleGroup
                    multiple
                    value={weekdays}
                    onValueChange={(value) => setWeekdays(value as string[])}
                    variant="outline"
                    disabled={submitting}
                    className="flex-wrap"
                  >
                    {WEEKDAYS.map((day) => (
                      <ToggleGroupItem
                        key={day.value}
                        value={String(day.value)}
                        aria-label={t(day.labelKey)}
                        className="aria-pressed:border-primary aria-pressed:bg-primary aria-pressed:text-primary-foreground aria-pressed:hover:bg-primary/90"
                      >
                        {t(day.initialKey)}
                      </ToggleGroupItem>
                    ))}
                  </ToggleGroup>
                  {weekdays.length === 0 && (
                    <FieldDescription className="text-destructive">
                      {t("task.weekdaysRequired")}
                    </FieldDescription>
                  )}
                </Field>
              )}

              {recurrence === "monthly" && (
                <Field>
                  <FieldLabel htmlFor="task-day-of-month">
                    {t("task.dayOfMonthLabel")}
                  </FieldLabel>
                  <Input
                    id="task-day-of-month"
                    type="number"
                    inputMode="numeric"
                    min={1}
                    max={31}
                    value={dayOfMonth}
                    onChange={(event) => setDayOfMonth(event.target.value)}
                    disabled={submitting}
                  />
                  <FieldDescription>
                    {t("task.dayOfMonthHint")}
                  </FieldDescription>
                </Field>
              )}

              {recurrence === "annual" && (
                <Field>
                  <FieldLabel htmlFor="task-day-of-month">
                    {t("task.annualDateLabel")}
                  </FieldLabel>
                  {/* Day and month are one date, so they read as one line. The
                      day only ever holds two digits, hence the narrow column. */}
                  <div className="flex items-center gap-2">
                    <Input
                      id="task-day-of-month"
                      type="number"
                      inputMode="numeric"
                      min={1}
                      max={31}
                      aria-label={t("task.dayOfMonthLabel")}
                      value={dayOfMonth}
                      onChange={(event) => setDayOfMonth(event.target.value)}
                      disabled={submitting}
                      className="w-20 shrink-0"
                    />
                    <Select
                      items={monthItems}
                      value={monthOfYear}
                      onValueChange={(value) => {
                        if (value) {
                          setMonthOfYear(String(value))
                        }
                      }}
                      disabled={submitting}
                    >
                      <SelectTrigger
                        id="task-month-of-year"
                        aria-label={t("task.monthOfYearLabel")}
                        className="w-full flex-1"
                      >
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        {monthItems.map((month) => (
                          <SelectItem key={month.value} value={month.value}>
                            {month.label}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                  <FieldDescription>
                    {t("task.dayOfMonthHint")}
                  </FieldDescription>
                </Field>
              )}

              {(recurrence === "monthly" || recurrence === "annual") && (
                <Field>
                  <FieldLabel>{t("task.strategyLabel")}</FieldLabel>
                  <ToggleGroup
                    value={[strategy]}
                    onValueChange={(value) => {
                      const next = value[0] as
                        TaskBusinessDayStrategy | undefined
                      if (next) {
                        setStrategy(next)
                      }
                    }}
                    variant="outline"
                    disabled={submitting}
                    className="flex-wrap"
                  >
                    {STRATEGIES.map((option) => (
                      <ToggleGroupItem
                        key={option.value}
                        value={option.value}
                        className="aria-pressed:border-primary aria-pressed:bg-primary aria-pressed:text-primary-foreground aria-pressed:hover:bg-primary/90"
                      >
                        {t(option.labelKey)}
                      </ToggleGroupItem>
                    ))}
                  </ToggleGroup>
                  <FieldDescription>{t("task.strategyHint")}</FieldDescription>
                </Field>
              )}

              <Field>
                <FieldLabel htmlFor="task-starts-on">
                  {recurrence === "once"
                    ? t("task.dueOnLabel")
                    : t("task.startsOnLabel")}
                </FieldLabel>
                <Input
                  id="task-starts-on"
                  type="date"
                  value={startsOn}
                  onChange={(event) => setStartsOn(event.target.value)}
                  disabled={submitting}
                />
              </Field>

              <Field>
                <FieldLabel>{t("task.instructionsLabel")}</FieldLabel>
                <MarkdownEditor
                  key={formKey}
                  value={instructions}
                  onChange={setInstructions}
                  disabled={submitting}
                  placeholder={t("task.instructionsPlaceholder")}
                />
                <FieldDescription>
                  {t("task.instructionsHint")}
                </FieldDescription>
              </Field>

              <Field orientation="horizontal">
                <Checkbox
                  id="task-pinned"
                  checked={pinned}
                  onCheckedChange={(checked) => setPinned(checked === true)}
                  disabled={submitting}
                />
                <div className="grid gap-1">
                  <Label htmlFor="task-pinned">{t("task.pinnedLabel")}</Label>
                  <FieldDescription>{t("task.pinnedHint")}</FieldDescription>
                </div>
              </Field>
            </FieldGroup>
          </div>

          <SheetFooter className="flex-row justify-end border-t border-border/70">
            <Button
              type="button"
              variant="outline"
              disabled={submitting}
              onClick={() => onOpenChange(false)}
            >
              {t("task.cancel")}
            </Button>
            <Button type="submit" disabled={!canSubmit}>
              {submitting && <Spinner data-icon="inline-start" />}
              {submitting ? t("task.createSubmitting") : t("task.createSubmit")}
            </Button>
          </SheetFooter>
        </form>
      </SheetContent>
    </Sheet>
  )
}

export { CreateTaskDrawer }
