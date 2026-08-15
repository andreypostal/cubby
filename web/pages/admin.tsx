import * as React from "react"
import { useNavigate } from "react-router"
import { toast } from "sonner"
import { HugeiconsIcon } from "@hugeicons/react"
import {
  Add01Icon,
  ArrowLeft01Icon,
  DatabaseBackupIcon,
  DatabaseImportIcon,
  Delete02Icon,
  Download04Icon,
} from "@hugeicons/core-free-icons"

import { Alert, AlertDescription } from "@/components/ui/alert"
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
import { Button } from "@/components/ui/button"
import { Spinner } from "@/components/ui/spinner"
import { LanguageSwitcher } from "@/components/language-switcher"
import { SessionCleanup } from "@/components/session-cleanup"
import { ThemeToggle } from "@/components/theme-toggle"
import { TLSSettings } from "@/components/tls-settings"
import { UserManagement } from "@/components/user-management"
import { useAuth } from "@/lib/auth-context"
import { useI18n } from "@/lib/i18n/context"
import { formatBytes, saveBlob } from "@/lib/utils"
import type { BackupArchive } from "@/lib/api"

/**
 * Server administration, reachable only by system admins. Backups are the only
 * thing it manages today, so the page is a single list with its actions.
 */
export default function AdminPage() {
  const navigate = useNavigate()
  const {
    status,
    user,
    listBackups,
    generateBackup,
    downloadBackup,
    deleteBackup,
    restoreBackup,
  } = useAuth()
  const { locale, t } = useI18n()

  const [backups, setBackups] = React.useState<BackupArchive[] | null>(null)
  const [keep, setKeep] = React.useState(0)
  const [loadError, setLoadError] = React.useState(false)

  const [generating, setGenerating] = React.useState(false)
  const [downloading, setDownloading] = React.useState<string | null>(null)
  const [pendingDelete, setPendingDelete] = React.useState<BackupArchive | null>(
    null
  )
  const [deleting, setDeleting] = React.useState(false)
  const [pendingRestore, setPendingRestore] =
    React.useState<BackupArchive | null>(null)
  const [restoring, setRestoring] = React.useState(false)

  const allowed = status === "authenticated" && user?.is_system_admin === true

  React.useEffect(() => {
    if (status === "unauthenticated") {
      navigate("/login", { replace: true })
    } else if (status === "authenticated" && !user?.is_system_admin) {
      navigate("/", { replace: true })
    }
  }, [status, user, navigate])

  const load = React.useCallback(async () => {
    try {
      const res = await listBackups()
      setBackups(res.backups)
      setKeep(res.keep)
      setLoadError(false)
    } catch {
      setLoadError(true)
    }
  }, [listBackups])

  React.useEffect(() => {
    if (!allowed) {
      return
    }
    // eslint-disable-next-line react-hooks/set-state-in-effect -- load only sets state once the fetch it awaits resolves
    load()
  }, [allowed, load])

  const dateFormatter = React.useMemo(
    () =>
      new Intl.DateTimeFormat(locale === "pt" ? "pt-BR" : "en-US", {
        day: "numeric",
        month: "short",
        year: "numeric",
        hour: "2-digit",
        minute: "2-digit",
      }),
    [locale]
  )

  async function onGenerate() {
    setGenerating(true)
    try {
      const archive = await generateBackup()
      saveBlob(archive.blob, archive.filename)
      toast.success(t("admin.generateSuccess"))
      // Generating also prunes, so the whole list is refetched rather than
      // having the new archive pushed onto it.
      await load()
    } catch {
      toast.error(t("admin.generateError"))
    } finally {
      setGenerating(false)
    }
  }

  async function onDownload(backup: BackupArchive) {
    setDownloading(backup.name)
    try {
      const archive = await downloadBackup(backup.name)
      saveBlob(archive.blob, archive.filename)
      toast.success(t("admin.downloadSuccess"))
    } catch {
      toast.error(t("admin.downloadError"))
    } finally {
      setDownloading(null)
    }
  }

  async function onConfirmDelete() {
    if (!pendingDelete) {
      return
    }

    setDeleting(true)
    try {
      await deleteBackup(pendingDelete.name)
      setBackups((prev) =>
        (prev ?? []).filter((item) => item.name !== pendingDelete.name)
      )
      setPendingDelete(null)
      toast.success(t("admin.deleteSuccess"))
    } catch {
      toast.error(t("admin.deleteError"))
    } finally {
      setDeleting(false)
    }
  }

  async function onConfirmRestore() {
    if (!pendingRestore) {
      return
    }

    setRestoring(true)
    try {
      await restoreBackup(pendingRestore.name)
      setPendingRestore(null)
      toast.success(t("admin.restoreSuccess"))
      // Everything held in memory came from the database that was just
      // replaced, so the whole app is reloaded rather than reconciled.
      window.location.reload()
    } catch {
      toast.error(t("admin.restoreError"))
    } finally {
      setRestoring(false)
    }
  }

  if (!allowed) {
    return (
      <div className="flex min-h-svh items-center justify-center">
        <Spinner className="size-6" />
      </div>
    )
  }

  return (
    <div className="flex min-h-svh justify-center p-6">
      <div className="absolute top-4 right-4 flex gap-2">
        <LanguageSwitcher />
        <ThemeToggle />
      </div>

      <div className="w-full max-w-2xl">
        <Button
          variant="link"
          size="sm"
          className="mb-6 px-0"
          onClick={() => navigate("/workspace")}
        >
          <HugeiconsIcon icon={ArrowLeft01Icon} data-icon="inline-start" />
          {t("admin.back")}
        </Button>

        <div className="mb-8">
          <h1 className="font-heading text-xl font-semibold">
            {t("admin.title")}
          </h1>
          <p className="mt-1 text-xs/relaxed text-muted-foreground">
            {t("admin.description")}
          </p>
        </div>

        <div className="mb-4 flex items-end justify-between gap-4">
          <div>
            <h2 className="font-heading text-base font-medium">
              {t("admin.backupsTitle")}
            </h2>
            {keep > 0 && (
              <p className="mt-1 text-xs/relaxed text-muted-foreground">
                {t("admin.retention", { keep })}
              </p>
            )}
          </div>
          <Button onClick={onGenerate} disabled={generating} className="shrink-0">
            {generating ? (
              <Spinner data-icon="inline-start" />
            ) : (
              <HugeiconsIcon icon={Add01Icon} data-icon="inline-start" />
            )}
            {generating ? t("admin.generating") : t("admin.generate")}
          </Button>
        </div>

        <p className="mb-4 text-xs/relaxed text-muted-foreground">
          {t("admin.generateHint")}
        </p>

        {loadError && (
          <Alert variant="destructive" className="mb-4">
            <AlertDescription className="flex items-center justify-between gap-4">
              {t("admin.loadError")}
              <Button variant="outline" size="sm" onClick={load}>
                {t("admin.retry")}
              </Button>
            </AlertDescription>
          </Alert>
        )}

        {backups === null && !loadError && (
          <div className="flex justify-center py-10">
            <Spinner className="size-6" />
          </div>
        )}

        {backups?.length === 0 && (
          <div className="flex flex-col items-center gap-2 rounded-lg border border-dashed border-foreground/15 px-6 py-10 text-center">
            <HugeiconsIcon
              icon={DatabaseBackupIcon}
              className="size-6 text-muted-foreground"
            />
            <p className="text-sm font-medium">{t("admin.empty")}</p>
            <p className="text-xs/relaxed text-muted-foreground">
              {t("admin.emptyDescription")}
            </p>
          </div>
        )}

        <div className="flex flex-col gap-2">
          {backups?.map((backup) => (
            <div
              key={backup.name}
              className="flex flex-col gap-3 rounded-lg bg-card p-3 ring-1 ring-foreground/10 sm:flex-row sm:items-center"
            >
              <div className="flex min-w-0 flex-1 items-center gap-3">
                <div className="flex size-9 shrink-0 items-center justify-center rounded-md bg-muted text-muted-foreground">
                  <HugeiconsIcon icon={DatabaseBackupIcon} className="size-4" />
                </div>

                <div className="min-w-0 flex-1">
                  <p className="truncate font-mono text-xs">{backup.name}</p>
                  <p className="mt-0.5 text-xs text-muted-foreground">
                    {dateFormatter.format(new Date(backup.created_at))} ·{" "}
                    {formatBytes(backup.size)}
                  </p>
                </div>
              </div>

              <div className="flex items-center gap-3">
                <Button
                  variant="outline"
                  size="sm"
                  className="flex-1 sm:flex-none"
                  disabled={downloading === backup.name}
                  onClick={() => onDownload(backup)}
                >
                  {downloading === backup.name ? (
                    <Spinner data-icon="inline-start" />
                  ) : (
                    <HugeiconsIcon
                      icon={Download04Icon}
                      data-icon="inline-start"
                    />
                  )}
                  {t("admin.download")}
                </Button>

                <Button
                  variant="outline"
                  size="sm"
                  className="flex-1 sm:flex-none"
                  onClick={() => setPendingRestore(backup)}
                >
                  <HugeiconsIcon
                    icon={DatabaseImportIcon}
                    data-icon="inline-start"
                  />
                  {t("admin.restore")}
                </Button>

                <Button
                  variant="ghost"
                  size="icon-sm"
                  className="shrink-0 text-muted-foreground hover:text-destructive"
                  aria-label={t("admin.delete")}
                  onClick={() => setPendingDelete(backup)}
                >
                  <HugeiconsIcon icon={Delete02Icon} />
                </Button>
              </div>
            </div>
          ))}
        </div>

        <UserManagement />

        <SessionCleanup dateFormatter={dateFormatter} />

        <TLSSettings dateFormatter={dateFormatter} />
      </div>

      <AlertDialog
        open={pendingRestore !== null}
        onOpenChange={(open) => {
          if (!open && !restoring) {
            setPendingRestore(null)
          }
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("admin.restoreTitle")}</AlertDialogTitle>
            <AlertDialogDescription>
              {t("admin.restoreDescription", {
                name: pendingRestore?.name ?? "",
              })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={restoring}>
              {t("admin.cancel")}
            </AlertDialogCancel>
            <AlertDialogAction
              variant="destructive"
              disabled={restoring}
              onClick={(event) => {
                // The dialog closes on click, the restore needs it to stay up
                // while the server is busy swapping the data
                event.preventDefault()
                onConfirmRestore()
              }}
            >
              {restoring && <Spinner data-icon="inline-start" />}
              {restoring ? t("admin.restoring") : t("admin.restoreConfirm")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AlertDialog
        open={pendingDelete !== null}
        onOpenChange={(open) => {
          if (!open) {
            setPendingDelete(null)
          }
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("admin.deleteTitle")}</AlertDialogTitle>
            <AlertDialogDescription>
              {t("admin.deleteDescription", { name: pendingDelete?.name ?? "" })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={deleting}>
              {t("admin.cancel")}
            </AlertDialogCancel>
            <AlertDialogAction
              variant="destructive"
              disabled={deleting}
              onClick={onConfirmDelete}
            >
              {t("admin.deleteConfirm")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
