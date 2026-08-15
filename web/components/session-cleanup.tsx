import * as React from "react"
import { HugeiconsIcon } from "@hugeicons/react"
import {
  Alert01Icon,
  CheckmarkCircle02Icon,
  ClockIcon,
  PauseIcon,
} from "@hugeicons/core-free-icons"

import { Alert, AlertDescription } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { Spinner } from "@/components/ui/spinner"
import { useAuth } from "@/lib/auth-context"
import { useI18n } from "@/lib/i18n/context"
import type { SessionCleanupReport, SessionCleanupRun } from "@/lib/api"

const runIcons = {
  ok: CheckmarkCircle02Icon,
  skipped: PauseIcon,
  failed: Alert01Icon,
} as const

/**
 * Reports the last runs of the job that sweeps expired sessions. It runs on the
 * server's own schedule, so there is nothing to trigger from here.
 */
function SessionCleanup({
  dateFormatter,
}: {
  dateFormatter: Intl.DateTimeFormat
}) {
  const { getSessionCleanupReport } = useAuth()
  const { t } = useI18n()

  const [report, setReport] = React.useState<SessionCleanupReport | null>(null)
  const [loadError, setLoadError] = React.useState(false)

  const load = React.useCallback(async () => {
    try {
      setReport(await getSessionCleanupReport())
      setLoadError(false)
    } catch {
      setLoadError(true)
    }
  }, [getSessionCleanupReport])

  React.useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- load only sets state once the fetch it awaits resolves
    load()
  }, [load])

  function summary(run: SessionCleanupRun) {
    if (run.status === "skipped") {
      return t("admin.cleanupSkipped")
    }

    if (run.status === "failed") {
      // The reason comes from the database driver, untranslated but useful
      return run.error ?? t("admin.cleanupFailed")
    }

    if (run.removed === 0) {
      return t("admin.cleanupRemovedNone")
    }

    return run.removed === 1
      ? t("admin.cleanupRemovedOne")
      : t("admin.cleanupRemoved", { count: run.removed })
  }

  return (
    <section className="mt-10">
      <div className="mb-4">
        <h2 className="font-heading text-base font-medium">
          {t("admin.cleanupTitle")}
        </h2>
        <p className="mt-1 text-xs/relaxed text-muted-foreground">
          {t("admin.cleanupDescription", {
            hours: Math.round((report?.interval_seconds ?? 3600) / 3600),
          })}
        </p>
      </div>

      {loadError && (
        <Alert variant="destructive">
          <AlertDescription className="flex items-center justify-between gap-4">
            {t("admin.cleanupLoadError")}
            <Button variant="outline" size="sm" onClick={load}>
              {t("admin.retry")}
            </Button>
          </AlertDescription>
        </Alert>
      )}

      {report === null && !loadError && (
        <div className="flex justify-center py-10">
          <Spinner className="size-6" />
        </div>
      )}

      {report?.runs.length === 0 && (
        <div className="flex flex-col items-center gap-2 rounded-lg border border-dashed border-foreground/15 px-6 py-10 text-center">
          <HugeiconsIcon icon={ClockIcon} className="size-6 text-muted-foreground" />
          <p className="text-sm font-medium">{t("admin.cleanupEmpty")}</p>
          <p className="text-xs/relaxed text-muted-foreground">
            {t("admin.cleanupEmptyDescription")}
          </p>
        </div>
      )}

      <div className="flex flex-col gap-2">
        {report?.runs.map((run) => (
          <div
            key={run.started_at}
            className="flex items-center gap-3 rounded-lg bg-card p-3 ring-1 ring-foreground/10"
          >
            <div
              className={`flex size-9 shrink-0 items-center justify-center rounded-md bg-muted ${
                run.status === "failed"
                  ? "text-destructive"
                  : "text-muted-foreground"
              }`}
            >
              <HugeiconsIcon icon={runIcons[run.status]} className="size-4" />
            </div>

            <div className="min-w-0 flex-1">
              <p className="truncate text-sm font-medium">{summary(run)}</p>
              <p className="mt-0.5 text-xs text-muted-foreground">
                {dateFormatter.format(new Date(run.started_at))} ·{" "}
                {t("admin.cleanupDuration", { ms: run.duration_ms })}
              </p>
            </div>
          </div>
        ))}
      </div>

      {report?.next_run && (
        <p className="mt-3 text-xs/relaxed text-muted-foreground">
          {t("admin.cleanupNextRun", {
            date: dateFormatter.format(new Date(report.next_run)),
          })}
        </p>
      )}
    </section>
  )
}

export { SessionCleanup }
