import * as React from "react"
import { HugeiconsIcon } from "@hugeicons/react"
import { Certificate01Icon, SquareUnlock02Icon } from "@hugeicons/core-free-icons"

import { Alert, AlertDescription } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { Spinner } from "@/components/ui/spinner"
import { useAuth } from "@/lib/auth-context"
import { useI18n } from "@/lib/i18n/context"
import type { TLSSettings as TLSSettingsValue } from "@/lib/api"

/**
 * Reports the certificate the API is serving with. TLS is set up in the
 * server's configuration, so there is nothing to change from here.
 */
function TLSSettings({ dateFormatter }: { dateFormatter: Intl.DateTimeFormat }) {
  const { getTLSSettings } = useAuth()
  const { t } = useI18n()

  const [settings, setSettings] = React.useState<TLSSettingsValue | null>(null)
  const [loadError, setLoadError] = React.useState(false)

  const load = React.useCallback(async () => {
    try {
      setSettings(await getTLSSettings())
      setLoadError(false)
    } catch {
      setLoadError(true)
    }
  }, [getTLSSettings])

  React.useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- load only sets state once the fetch it awaits resolves
    load()
  }, [load])

  const cert = settings?.certificate
  const expired = cert?.expired ?? false

  return (
    <section className="mt-10">
      <div className="mb-4">
        <h2 className="font-heading text-base font-medium">
          {t("admin.tlsTitle")}
        </h2>
        <p className="mt-1 text-xs/relaxed text-muted-foreground">
          {t("admin.tlsDescription")}
        </p>
      </div>

      {loadError && (
        <Alert variant="destructive">
          <AlertDescription className="flex items-center justify-between gap-4">
            {t("admin.tlsLoadError")}
            <Button variant="outline" size="sm" onClick={load}>
              {t("admin.retry")}
            </Button>
          </AlertDescription>
        </Alert>
      )}

      {settings === null && !loadError && (
        <div className="flex justify-center py-10">
          <Spinner className="size-6" />
        </div>
      )}

      {settings !== null && !cert && (
        <div className="flex items-center gap-3 rounded-lg bg-card p-3 ring-1 ring-foreground/10">
          <div className="flex size-9 shrink-0 items-center justify-center rounded-md bg-muted text-muted-foreground">
            <HugeiconsIcon icon={SquareUnlock02Icon} className="size-4" />
          </div>

          <div className="min-w-0 flex-1">
            <p className="truncate text-sm font-medium">
              {t("admin.tlsDisabled")}
            </p>
            <p className="mt-0.5 text-xs text-muted-foreground">
              {t("admin.tlsDisabledDescription")}
            </p>
          </div>
        </div>
      )}

      {cert && (
        <div className="flex items-center gap-3 rounded-lg bg-card p-3 ring-1 ring-foreground/10">
          <div className="flex size-9 shrink-0 items-center justify-center rounded-md bg-muted text-muted-foreground">
            <HugeiconsIcon icon={Certificate01Icon} className="size-4" />
          </div>

          <div className="min-w-0 flex-1">
            <p className="truncate text-sm font-medium">{cert.subject}</p>
            <p
              className={`mt-0.5 text-xs ${expired ? "text-destructive" : "text-muted-foreground"}`}
            >
              {t("admin.tlsValidUntil", {
                date: dateFormatter.format(new Date(cert.not_after)),
              })}
              {expired && ` · ${t("admin.tlsExpired")}`}
            </p>
          </div>
        </div>
      )}
    </section>
  )
}

export { TLSSettings }
