import * as React from "react"
import { useNavigate } from "react-router"
import { HugeiconsIcon } from "@hugeicons/react"
import {
  Add01Icon,
  ArrowRight01Icon,
  CloudServerIcon,
  Logout05Icon,
} from "@hugeicons/core-free-icons"

import { Button } from "@/components/ui/button"
import { Card, CardTitle } from "@/components/ui/card"
import { Field, FieldGroup } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Alert, AlertDescription } from "@/components/ui/alert"
import { Spinner } from "@/components/ui/spinner"
import { LanguageSwitcher } from "@/components/language-switcher"
import { ThemeToggle } from "@/components/theme-toggle"
import { useAuth } from "@/lib/auth-context"
import { useI18n } from "@/lib/i18n/context"
import type { Workspace } from "@/lib/api"

export default function WorkspaceSelectionPage() {
  const navigate = useNavigate()
  const {
    status,
    setupRequired,
    user,
    workspace,
    logout,
    selectWorkspace,
    listWorkspaces,
    createWorkspace,
  } = useAuth()
  const { t } = useI18n()

  const [workspaces, setWorkspaces] = React.useState<Workspace[] | null>(null)
  const [loadError, setLoadError] = React.useState(false)
  const [selectingId, setSelectingId] = React.useState<number | null>(null)

  const [isCreating, setIsCreating] = React.useState(false)
  const [newName, setNewName] = React.useState("")
  const [createError, setCreateError] = React.useState(false)
  const [submitting, setSubmitting] = React.useState(false)

  React.useEffect(() => {
    if (setupRequired) {
      navigate("/setup", { replace: true })
    } else if (status === "unauthenticated") {
      navigate("/login", { replace: true })
    } else if (status === "authenticated" && workspace) {
      navigate("/", { replace: true })
    }
  }, [status, setupRequired, workspace, navigate])

  React.useEffect(() => {
    if (status !== "authenticated" || workspace) {
      return
    }
    listWorkspaces()
      .then(setWorkspaces)
      .catch(() => setLoadError(true))
  }, [status, workspace, listWorkspaces])

  async function onSelect(id: number) {
    setSelectingId(id)
    try {
      await selectWorkspace(id)
      navigate("/", { replace: true })
    } catch {
      setSelectingId(null)
      setLoadError(true)
    }
  }

  async function onCreate(event: React.FormEvent) {
    event.preventDefault()
    if (!newName.trim()) {
      return
    }

    setSubmitting(true)
    setCreateError(false)
    try {
      const created = await createWorkspace(newName.trim())
      setWorkspaces((prev) => [...(prev ?? []), created])
      setNewName("")
      setIsCreating(false)
    } catch {
      setCreateError(true)
    } finally {
      setSubmitting(false)
    }
  }

  if (status === "loading" || status === "unauthenticated" || (status === "authenticated" && workspace)) {
    return (
      <div className="flex min-h-svh items-center justify-center">
        <Spinner className="size-6" />
      </div>
    )
  }

  return (
    <div className="flex min-h-svh items-center justify-center p-6">
      <div className="absolute top-4 right-4 flex gap-2">
        {user?.is_system_admin && (
          <Button
            variant="outline"
            size="icon"
            aria-label={t("admin.open")}
            onClick={() => navigate("/admin")}
          >
            <HugeiconsIcon icon={CloudServerIcon} />
          </Button>
        )}
        <LanguageSwitcher />
        <ThemeToggle />
      </div>

      <div className="w-full max-w-md">
        <div className="mb-6 flex items-start justify-between gap-4">
          <div>
            <h1 className="font-heading text-xl font-semibold">{t("workspace.title")}</h1>
            <p className="mt-1 text-xs/relaxed text-muted-foreground">
              {t("workspace.description")}
            </p>
          </div>
          <Button variant="link" size="sm" className="shrink-0 px-0" onClick={logout}>
            <HugeiconsIcon icon={Logout05Icon} data-icon="inline-start" />
            {t("workspace.logout")}
          </Button>
        </div>

        {loadError && (
          <Alert variant="destructive" className="mb-4">
            <AlertDescription>{t("workspace.loadError")}</AlertDescription>
          </Alert>
        )}

        <div className="flex flex-col gap-2">
          {workspaces === null && !loadError && (
            <div className="flex justify-center py-6">
              <Spinner className="size-6" />
            </div>
          )}

          {workspaces?.length === 0 && (
            <p className="py-2 text-center text-xs/relaxed text-muted-foreground">
              {t("workspace.empty")}
            </p>
          )}

          {workspaces?.map((ws) => (
            <button
              key={ws.id}
              type="button"
              onClick={() => onSelect(ws.id)}
              disabled={selectingId !== null}
              className="group/button flex w-full items-center gap-3 rounded-lg bg-card p-3 text-left ring-1 ring-foreground/10 transition-all hover:bg-muted disabled:pointer-events-none disabled:opacity-50"
            >
              <div className="flex size-9 shrink-0 items-center justify-center rounded-md bg-primary text-sm font-medium text-primary-foreground">
                {ws.name.charAt(0).toUpperCase()}
              </div>
              <div className="min-w-0 flex-1">
                <CardTitle className="truncate">{ws.name}</CardTitle>
              </div>
              {selectingId === ws.id ? (
                <Spinner className="size-4 shrink-0" />
              ) : (
                <HugeiconsIcon
                  icon={ArrowRight01Icon}
                  className="size-4 shrink-0 text-muted-foreground"
                />
              )}
            </button>
          ))}

          {isCreating ? (
            <Card className="p-1">
              <form onSubmit={onCreate} className="p-2">
                <FieldGroup>
                  {createError && (
                    <Alert variant="destructive">
                      <AlertDescription>{t("workspace.createError")}</AlertDescription>
                    </Alert>
                  )}
                  <Field>
                    <Input
                      autoFocus
                      placeholder={t("workspace.namePlaceholder")}
                      value={newName}
                      onChange={(event) => setNewName(event.target.value)}
                      disabled={submitting}
                    />
                  </Field>
                  <div className="flex gap-2">
                    <Button type="submit" className="flex-1" disabled={submitting}>
                      {submitting && <Spinner data-icon="inline-start" />}
                      {submitting ? t("workspace.createSubmitting") : t("workspace.createSubmit")}
                    </Button>
                    <Button
                      type="button"
                      variant="outline"
                      className="flex-1"
                      disabled={submitting}
                      onClick={() => {
                        setIsCreating(false)
                        setNewName("")
                        setCreateError(false)
                      }}
                    >
                      {t("workspace.cancel")}
                    </Button>
                  </div>
                </FieldGroup>
              </form>
            </Card>
          ) : (
            <button
              type="button"
              onClick={() => setIsCreating(true)}
              className="flex w-full items-center justify-center gap-2 rounded-lg border border-dashed border-foreground/15 p-3 text-xs/relaxed font-medium text-muted-foreground transition-all hover:bg-muted hover:text-foreground"
            >
              <HugeiconsIcon icon={Add01Icon} className="size-4" />
              {t("workspace.createNew")}
            </button>
          )}
        </div>
      </div>
    </div>
  )
}
