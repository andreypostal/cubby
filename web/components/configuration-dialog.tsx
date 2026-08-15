import * as React from "react"
import { useNavigate } from "react-router"
import { useTheme } from "next-themes"
import { toast } from "sonner"
import { HugeiconsIcon } from "@hugeicons/react"
import {
  Building05Icon,
  CogIcon,
  ComputerIcon,
  Delete02Icon,
  Logout05Icon,
  Moon02Icon,
  Sun03Icon,
  UserAdd01Icon,
  UserRemove01Icon,
} from "@hugeicons/core-free-icons"

import { useResetOnOpen } from "@/hooks/use-reset-on-open"
import { Alert, AlertDescription } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { ConfirmDestructiveDialog } from "@/components/confirm-destructive-dialog"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogTitle,
} from "@/components/ui/dialog"
import { Field, FieldGroup, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Spinner } from "@/components/ui/spinner"
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group"
import { ApiError, useAuth } from "@/lib/auth-context"
import { useI18n } from "@/lib/i18n/context"
import {
  locales,
  localeNames,
  type Locale,
  type TranslationKey,
} from "@/lib/i18n/translations"
import type { Workspace, WorkspaceMember } from "@/lib/api"

const SECTIONS = [
  { id: "preferences", labelKey: "config.preferences", icon: CogIcon },
  { id: "workspace", labelKey: "config.workspace", icon: Building05Icon },
] as const

type SectionId = (typeof SECTIONS)[number]["id"]

const THEMES = [
  { value: "light", labelKey: "theme.light", icon: Sun03Icon },
  { value: "dark", labelKey: "theme.dark", icon: Moon02Icon },
  { value: "system", labelKey: "theme.system", icon: ComputerIcon },
] as const

const TOGGLE_SELECTED =
  "aria-pressed:border-primary aria-pressed:bg-primary aria-pressed:text-primary-foreground aria-pressed:hover:bg-primary/90"

function ConfigurationDialog({
  open,
  onOpenChange,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const { t } = useI18n()
  const { workspace } = useAuth()

  const [section, setSection] = React.useState<SectionId>("preferences")

  useResetOnOpen(open, () => {
    setSection("preferences")
  })

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        forceOverlay
        className="gap-0 overflow-hidden p-0 sm:max-w-2xl"
      >
        <div className="flex min-h-[26rem] flex-col sm:flex-row">
          <nav className="shrink-0 bg-muted/40 p-3 sm:w-52 sm:border-r sm:border-foreground/10">
            <div className="px-2 pt-1 pb-3">
              <DialogTitle>{t("config.title")}</DialogTitle>
              <DialogDescription className="mt-1 text-xs/relaxed">
                {t("config.description")}
              </DialogDescription>
            </div>
            <div className="flex gap-1 sm:flex-col">
              {SECTIONS.map((item) => (
                <button
                  key={item.id}
                  type="button"
                  onClick={() => setSection(item.id)}
                  data-active={section === item.id}
                  className="flex flex-1 items-center gap-2 rounded-lg px-2.5 py-2 text-left text-xs font-medium text-muted-foreground transition-colors hover:bg-muted hover:text-foreground data-[active=true]:bg-background data-[active=true]:text-foreground data-[active=true]:ring-1 data-[active=true]:ring-foreground/10 sm:flex-none"
                >
                  <HugeiconsIcon icon={item.icon} className="size-4 shrink-0" />
                  {t(item.labelKey)}
                </button>
              ))}
            </div>
          </nav>

          <div className="max-h-[70svh] min-w-0 flex-1 overflow-y-auto p-5">
            {section === "preferences" && <PreferencesSection />}
            {section === "workspace" && workspace && (
              // Keyed so a different cubby starts the section from scratch,
              // members and the name field included
              <WorkspaceSection
                key={workspace.id}
                workspace={workspace}
                onClose={() => onOpenChange(false)}
              />
            )}
          </div>
        </div>
      </DialogContent>
    </Dialog>
  )
}

function PreferencesSection() {
  const { locale, setLocale, t } = useI18n()
  const { theme, setTheme } = useTheme()

  const currentTheme =
    THEMES.find((option) => option.value === theme)?.value ?? "system"

  return (
    <FieldGroup>
      <SectionTitle titleKey="config.preferences" />
      <Field>
        <FieldLabel>{t("language.label")}</FieldLabel>
        <ToggleGroup
          value={[locale]}
          onValueChange={(value) => {
            const next = value[0] as Locale | undefined
            if (next) {
              setLocale(next)
            }
          }}
          variant="outline"
          className="flex-wrap"
        >
          {locales.map((code) => (
            <ToggleGroupItem
              key={code}
              value={code}
              className={TOGGLE_SELECTED}
            >
              {localeNames[code]}
            </ToggleGroupItem>
          ))}
        </ToggleGroup>
      </Field>

      <Field>
        <FieldLabel>{t("config.theme")}</FieldLabel>
        <ToggleGroup
          value={[currentTheme]}
          onValueChange={(value) => {
            const next = value[0]
            if (next) {
              setTheme(next)
            }
          }}
          variant="outline"
          className="flex-wrap"
        >
          {THEMES.map((option) => (
            <ToggleGroupItem
              key={option.value}
              value={option.value}
              className={TOGGLE_SELECTED}
            >
              <HugeiconsIcon icon={option.icon} data-icon="inline-start" />
              {t(option.labelKey)}
            </ToggleGroupItem>
          ))}
        </ToggleGroup>
      </Field>
    </FieldGroup>
  )
}

function WorkspaceSection({
  workspace,
  onClose,
}: {
  workspace: Workspace
  onClose: () => void
}) {
  const navigate = useNavigate()
  const { t } = useI18n()
  const {
    renameWorkspace,
    deleteWorkspace,
    leaveWorkspace,
    listWorkspaceMembers,
  } = useAuth()

  const [name, setName] = React.useState(workspace.name)
  const [renaming, setRenaming] = React.useState(false)
  const [renameError, setRenameError] = React.useState(false)

  const [members, setMembers] = React.useState<WorkspaceMember[] | null>(null)
  const [membersError, setMembersError] = React.useState(false)

  const [leaveOpen, setLeaveOpen] = React.useState(false)
  const [deleteOpen, setDeleteOpen] = React.useState(false)

  React.useEffect(() => {
    let active = true

    listWorkspaceMembers(workspace.id)
      .then((list) => {
        if (active) {
          setMembers(list)
        }
      })
      .catch(() => {
        if (active) {
          setMembersError(true)
        }
      })

    return () => {
      active = false
    }
  }, [workspace.id, listWorkspaceMembers])

  const hasNameChange = name.trim() !== workspace.name && name.trim() !== ""

  async function onRename(event: React.FormEvent) {
    event.preventDefault()
    if (!hasNameChange) {
      return
    }

    setRenaming(true)
    setRenameError(false)
    try {
      await renameWorkspace(workspace.id, name.trim())
      toast.success(t("config.saveSuccess"))
    } catch {
      setRenameError(true)
    } finally {
      setRenaming(false)
    }
  }

  // Both the creator's delete and a shared user's leave end with the workspace
  // gone from under the session, so the app goes back to picking one
  function onLeftWorkspace() {
    onClose()
    navigate("/workspace", { replace: true })
  }

  return (
    <FieldGroup>
      <SectionTitle titleKey="config.workspace" />

      {workspace.is_creator ? (
        <form onSubmit={onRename} noValidate>
          <FieldGroup>
            {renameError && (
              <Alert variant="destructive">
                <AlertDescription>{t("config.saveError")}</AlertDescription>
              </Alert>
            )}
            <Field>
              <FieldLabel htmlFor="workspace-name">
                {t("config.workspaceName")}
              </FieldLabel>
              <div className="flex gap-2">
                <Input
                  id="workspace-name"
                  value={name}
                  onChange={(event) => setName(event.target.value)}
                  disabled={renaming}
                />
                <Button type="submit" disabled={!hasNameChange || renaming}>
                  {renaming && <Spinner data-icon="inline-start" />}
                  {renaming ? t("config.saving") : t("config.save")}
                </Button>
              </div>
            </Field>
          </FieldGroup>
        </form>
      ) : (
        <Field>
          <FieldLabel>{t("config.workspaceName")}</FieldLabel>
          <Input value={workspace.name} readOnly disabled />
          <p className="text-xs/relaxed text-muted-foreground">
            {t("config.guestNotice")}
          </p>
        </Field>
      )}

      <MembersSection
        workspace={workspace}
        members={members}
        hasError={membersError}
        onMembersChange={setMembers}
      />

      <div className="flex flex-col gap-2 rounded-lg border border-destructive/30 p-3">
        <p className="text-xs/relaxed text-muted-foreground">
          {workspace.is_creator
            ? t("config.deleteDescription")
            : t("config.leaveDescription")}
        </p>
        {workspace.is_creator ? (
          <Button
            type="button"
            variant="destructive"
            size="sm"
            className="self-start"
            onClick={() => setDeleteOpen(true)}
          >
            <HugeiconsIcon icon={Delete02Icon} data-icon="inline-start" />
            {t("config.delete")}
          </Button>
        ) : (
          <Button
            type="button"
            variant="destructive"
            size="sm"
            className="self-start"
            onClick={() => setLeaveOpen(true)}
          >
            <HugeiconsIcon icon={Logout05Icon} data-icon="inline-start" />
            {t("config.leave")}
          </Button>
        )}
      </div>

      <ConfirmDestructiveDialog
        forceOverlay
        open={leaveOpen}
        onOpenChange={setLeaveOpen}
        title={t("config.leaveTitle")}
        description={t("config.leaveConfirmDescription", {
          name: workspace.name,
        })}
        errorMessage={t("config.leaveError")}
        cancelLabel={t("config.cancel")}
        confirmLabel={t("config.leave")}
        onConfirm={async () => {
          await leaveWorkspace(workspace.id)
          toast.success(t("config.leaveSuccess"))
          onLeftWorkspace()
        }}
      />

      <ConfirmDestructiveDialog
        forceOverlay
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
        title={t("config.deleteTitle")}
        description={t("config.deleteConfirmDescription", {
          name: workspace.name,
        })}
        errorMessage={t("config.deleteError")}
        cancelLabel={t("config.cancel")}
        confirmLabel={t("config.delete")}
        confirmPlaceholder={t("config.deleteConfirmPlaceholder")}
        onConfirm={async () => {
          await deleteWorkspace(workspace.id)
          toast.success(t("config.deleteSuccess"))
          onLeftWorkspace()
        }}
      />
    </FieldGroup>
  )
}

/**
 * The people the workspace is shared with. The creator adds and removes them
 * here; everyone else only gets to see who else is in.
 */
function MembersSection({
  workspace,
  members,
  hasError,
  onMembersChange,
}: {
  workspace: Workspace
  members: WorkspaceMember[] | null
  hasError: boolean
  onMembersChange: (members: WorkspaceMember[]) => void
}) {
  const { t } = useI18n()
  const { shareWorkspace, revokeWorkspaceAccess } = useAuth()

  const [username, setUsername] = React.useState("")
  const [sharing, setSharing] = React.useState(false)
  const [shareError, setShareError] = React.useState<TranslationKey | null>(null)
  const [pendingRevoke, setPendingRevoke] =
    React.useState<WorkspaceMember | null>(null)

  async function onShare(event: React.FormEvent) {
    event.preventDefault()

    const target = username.trim()
    if (!target) {
      return
    }

    setSharing(true)
    setShareError(null)
    try {
      const member = await shareWorkspace(workspace.id, target)
      onMembersChange([...(members ?? []), member])
      setUsername("")
      toast.success(t("config.shareSuccess", { username: member.username }))
    } catch (error) {
      if (error instanceof ApiError && error.status === 404) {
        setShareError("config.shareNotFound")
      } else if (error instanceof ApiError && error.status === 409) {
        setShareError("config.shareDuplicated")
      } else {
        setShareError("config.shareError")
      }
    } finally {
      setSharing(false)
    }
  }

  async function onRevoke(member: WorkspaceMember) {
    await revokeWorkspaceAccess(workspace.id, member.id)
    onMembersChange((members ?? []).filter((m) => m.id !== member.id))
    toast.success(t("config.revokeSuccess", { name: member.name }))
  }

  return (
    <Field>
      <FieldLabel>{t("config.membersTitle")}</FieldLabel>
      <p className="text-xs/relaxed text-muted-foreground">
        {workspace.is_creator
          ? t("config.membersDescription")
          : t("config.membersGuestDescription")}
      </p>

      {workspace.is_creator && (
        <form onSubmit={onShare} noValidate className="flex flex-col gap-2">
          {shareError && (
            <Alert variant="destructive">
              <AlertDescription>{t(shareError)}</AlertDescription>
            </Alert>
          )}
          <div className="flex gap-2">
            <Input
              placeholder={t("config.sharePlaceholder")}
              value={username}
              onChange={(event) => setUsername(event.target.value)}
              disabled={sharing}
              autoComplete="off"
            />
            <Button type="submit" disabled={sharing || !username.trim()}>
              {sharing ? (
                <Spinner data-icon="inline-start" />
              ) : (
                <HugeiconsIcon icon={UserAdd01Icon} data-icon="inline-start" />
              )}
              {sharing ? t("config.sharing") : t("config.share")}
            </Button>
          </div>
        </form>
      )}

      {hasError && (
        <Alert variant="destructive">
          <AlertDescription>{t("config.membersLoadError")}</AlertDescription>
        </Alert>
      )}

      {members === null && !hasError && (
        <div className="flex justify-center py-3">
          <Spinner className="size-4" />
        </div>
      )}

      {members?.length === 0 && (
        <p className="py-1 text-xs/relaxed text-muted-foreground">
          {t("config.membersEmpty")}
        </p>
      )}

      {members && members.length > 0 && (
        <ul className="flex flex-col gap-1">
          {members.map((member) => (
            <li
              key={member.id}
              className="flex items-center gap-3 rounded-lg bg-card p-2 ring-1 ring-foreground/10"
            >
              <div className="flex size-7 shrink-0 items-center justify-center rounded-md bg-primary text-xs font-medium text-primary-foreground">
                {member.name.charAt(0).toUpperCase()}
              </div>
              <div className="min-w-0 flex-1">
                <p className="truncate text-xs font-medium">{member.name}</p>
                <p className="truncate text-[0.65rem] text-muted-foreground">
                  {member.username}
                </p>
              </div>
              {workspace.is_creator && (
                <Button
                  type="button"
                  variant="ghost"
                  size="icon-sm"
                  aria-label={t("config.revoke")}
                  onClick={() => setPendingRevoke(member)}
                >
                  <HugeiconsIcon icon={UserRemove01Icon} />
                </Button>
              )}
            </li>
          ))}
        </ul>
      )}

      <ConfirmDestructiveDialog
        forceOverlay
        open={pendingRevoke !== null}
        onOpenChange={(next) => {
          if (!next) {
            setPendingRevoke(null)
          }
        }}
        title={t("config.revokeTitle")}
        description={t("config.revokeConfirmDescription", {
          name: pendingRevoke?.name ?? "",
        })}
        errorMessage={t("config.revokeError")}
        cancelLabel={t("config.cancel")}
        confirmLabel={t("config.revoke")}
        onConfirm={async () => {
          if (pendingRevoke) {
            await onRevoke(pendingRevoke)
            setPendingRevoke(null)
          }
        }}
      />
    </Field>
  )
}

function SectionTitle({ titleKey }: { titleKey: TranslationKey }) {
  const { t } = useI18n()

  return (
    <h2 className="font-heading text-sm font-medium sm:hidden">{t(titleKey)}</h2>
  )
}

export { ConfigurationDialog }
