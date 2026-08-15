import * as React from "react"
import { HugeiconsIcon } from "@hugeicons/react"
import { Cancel01Icon, Tick02Icon } from "@hugeicons/core-free-icons"

import { useResetOnOpen } from "@/hooks/use-reset-on-open"
import { cn } from "@/lib/utils"
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
import { Spinner } from "@/components/ui/spinner"
import { useAuth } from "@/lib/auth-context"
import { useI18n } from "@/lib/i18n/context"
import type { RoutineDayStatus } from "@/lib/api"

type PendingAction = "achieved" | "failed" | "clear" | null

function MarkRoutineDayDialog({
  open,
  onOpenChange,
  routineId,
  dateStr,
  status,
  onChanged,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  routineId: number
  dateStr: string | null
  status: RoutineDayStatus | null
  onChanged: (dateStr: string, status: RoutineDayStatus, points: number) => void
}) {
  const { markRoutineDay, unmarkRoutineDay } = useAuth()
  const { t, locale } = useI18n()

  const [pending, setPending] = React.useState<PendingAction>(null)
  const [hasError, setHasError] = React.useState(false)

  useResetOnOpen(open, () => {
    setHasError(false)
    setPending(null)
  })

  const dateLabel = React.useMemo(() => {
    if (!dateStr) {
      return ""
    }
    const [year, month, day] = dateStr.split("-").map(Number)
    const date = new Date(year, month - 1, day)
    return new Intl.DateTimeFormat(locale === "pt" ? "pt-BR" : "en-US", {
      weekday: "long",
      day: "numeric",
      month: "long",
      year: "numeric",
    }).format(date)
  }, [dateStr, locale])

  function handleOpenChange(next: boolean) {
    if (!pending) {
      onOpenChange(next)
    }
  }

  async function onMark(achieved: boolean) {
    if (!dateStr) {
      return
    }
    setPending(achieved ? "achieved" : "failed")
    setHasError(false)
    try {
      const result = await markRoutineDay(routineId, dateStr, achieved)
      onChanged(dateStr, result.day.status, result.routine.points)
      onOpenChange(false)
    } catch {
      setHasError(true)
    } finally {
      setPending(null)
    }
  }

  async function onClear() {
    if (!dateStr) {
      return
    }
    setPending("clear")
    setHasError(false)
    try {
      const routine = await unmarkRoutineDay(routineId, dateStr)
      onChanged(dateStr, "waiting", routine.points)
      onOpenChange(false)
    } catch {
      setHasError(true)
    } finally {
      setPending(null)
    }
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle className="capitalize">
            {dateLabel || t("goal.markTitle", { date: "" })}
          </DialogTitle>
          <DialogDescription>{t("goal.markDescription")}</DialogDescription>
        </DialogHeader>

        {hasError && (
          <Alert variant="destructive">
            <AlertDescription>{t("goal.markError")}</AlertDescription>
          </Alert>
        )}

        <div className="grid grid-cols-2 gap-3">
          <Button
            type="button"
            variant="outline"
            disabled={pending !== null || status === "achieved"}
            onClick={() => onMark(true)}
            className={cn(
              "h-16 flex-row gap-2",
              status === "achieved" &&
                "border-emerald-500/50 bg-emerald-500/10 text-emerald-600 dark:text-emerald-400"
            )}
          >
            {pending === "achieved" ? (
              <Spinner data-icon="inline-start" />
            ) : (
              <HugeiconsIcon icon={Tick02Icon} data-icon="inline-start" />
            )}
            {t("goal.markSucceeded")}
          </Button>
          <Button
            type="button"
            variant="outline"
            disabled={pending !== null || status === "failed"}
            onClick={() => onMark(false)}
            className={cn(
              "h-16 flex-row gap-2",
              status === "failed" &&
                "border-red-500/50 bg-red-500/10 text-red-600 dark:text-red-400"
            )}
          >
            {pending === "failed" ? (
              <Spinner data-icon="inline-start" />
            ) : (
              <HugeiconsIcon icon={Cancel01Icon} data-icon="inline-start" />
            )}
            {t("goal.markFailed")}
          </Button>
        </div>

        {status !== "waiting" && (
          <DialogFooter>
            <Button
              type="button"
              variant="ghost"
              size="sm"
              disabled={pending !== null}
              onClick={onClear}
              className="text-muted-foreground hover:text-foreground"
            >
              {pending === "clear" && <Spinner data-icon="inline-start" />}
              {t("goal.markClear")}
            </Button>
          </DialogFooter>
        )}
      </DialogContent>
    </Dialog>
  )
}

export { MarkRoutineDayDialog }
