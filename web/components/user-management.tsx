import * as React from "react"
import { toast } from "sonner"
import { HugeiconsIcon } from "@hugeicons/react"
import {
  Key01Icon,
  PencilEdit02Icon,
  UserAdd01Icon,
  UserRemove01Icon,
} from "@hugeicons/core-free-icons"

import { Alert, AlertDescription } from "@/components/ui/alert"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Field, FieldGroup, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Spinner } from "@/components/ui/spinner"
import { ConfirmDestructiveDialog } from "@/components/confirm-destructive-dialog"
import { useResetOnOpen } from "@/hooks/use-reset-on-open"
import { ApiError, useAuth } from "@/lib/auth-context"
import { useI18n } from "@/lib/i18n/context"
import type { User } from "@/lib/api"
import type { TranslationKey } from "@/lib/i18n/translations"

/** Matches the minimum the API enforces. */
const MIN_PASSWORD_LENGTH = 8

/**
 * Everyone who can sign in to this server. Creating and removing accounts is
 * the only way in, the application has no sign-up.
 */
function UserManagement() {
  const { t } = useI18n()
  const { user: currentUser, listUsers, deleteUser } = useAuth()

  const [users, setUsers] = React.useState<User[] | null>(null)
  const [loadError, setLoadError] = React.useState(false)

  const [creating, setCreating] = React.useState(false)
  const [editing, setEditing] = React.useState<User | null>(null)
  const [changingPassword, setChangingPassword] = React.useState<User | null>(
    null
  )
  const [pendingDelete, setPendingDelete] = React.useState<User | null>(null)
  const [deleteError, setDeleteError] = React.useState<string | null>(null)

  const load = React.useCallback(async () => {
    try {
      setUsers(await listUsers())
      setLoadError(false)
    } catch {
      setLoadError(true)
    }
  }, [listUsers])

  React.useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- load only sets state once the fetch it awaits resolves
    load()
  }, [load])

  async function onDelete(target: User) {
    setDeleteError(null)
    try {
      await deleteUser(target.id)
      setUsers((prev) => (prev ?? []).filter((u) => u.id !== target.id))
      setPendingDelete(null)
      toast.success(t("admin.userDeleteSuccess", { name: target.name }))
    } catch (error) {
      // The cubbies the user created are named in the refusal, so the message
      // can say what has to be dealt with first
      if (error instanceof ApiError && error.status === 409) {
        const names = error.details?.workspaces
        setDeleteError(
          Array.isArray(names) && names.length > 0
            ? t("admin.userDeleteOwnsCubbies", { names: names.join(", ") })
            : t("admin.userDeleteLastAdmin")
        )
      } else {
        setDeleteError(t("admin.userDeleteError"))
      }
      throw error
    }
  }

  return (
    <section className="mt-10">
      <div className="mb-4 flex items-end justify-between gap-4">
        <div>
          <h2 className="font-heading text-base font-medium">
            {t("admin.usersTitle")}
          </h2>
          <p className="mt-1 text-xs/relaxed text-muted-foreground">
            {t("admin.usersDescription")}
          </p>
        </div>
        <Button onClick={() => setCreating(true)} className="shrink-0">
          <HugeiconsIcon icon={UserAdd01Icon} data-icon="inline-start" />
          {t("admin.userCreate")}
        </Button>
      </div>

      {loadError && (
        <Alert variant="destructive">
          <AlertDescription className="flex items-center justify-between gap-4">
            {t("admin.usersLoadError")}
            <Button variant="outline" size="sm" onClick={load}>
              {t("admin.retry")}
            </Button>
          </AlertDescription>
        </Alert>
      )}

      {users === null && !loadError && (
        <div className="flex justify-center py-10">
          <Spinner className="size-6" />
        </div>
      )}

      <div className="flex flex-col gap-2">
        {users?.map((user) => (
          <div
            key={user.id}
            className="flex items-center gap-3 rounded-lg bg-card p-3 ring-1 ring-foreground/10"
          >
            <div className="flex size-9 shrink-0 items-center justify-center rounded-md bg-primary text-sm font-medium text-primary-foreground">
              {user.name.charAt(0).toUpperCase()}
            </div>

            <div className="min-w-0 flex-1">
              <p className="flex items-center gap-2 truncate text-sm font-medium">
                {user.name}
                {user.is_system_admin && (
                  <Badge variant="secondary">{t("admin.userAdminBadge")}</Badge>
                )}
              </p>
              <p className="mt-0.5 truncate text-xs text-muted-foreground">
                {user.username}
                {user.id === currentUser?.id && ` · ${t("admin.userIsYou")}`}
              </p>
            </div>

            <div className="flex items-center gap-1">
              <Button
                variant="ghost"
                size="icon-sm"
                aria-label={t("admin.userEdit")}
                onClick={() => setEditing(user)}
              >
                <HugeiconsIcon icon={PencilEdit02Icon} />
              </Button>

              <Button
                variant="ghost"
                size="icon-sm"
                aria-label={t("admin.userPassword")}
                onClick={() => setChangingPassword(user)}
              >
                <HugeiconsIcon icon={Key01Icon} />
              </Button>

              <Button
                variant="ghost"
                size="icon-sm"
                className="text-muted-foreground hover:text-destructive"
                aria-label={t("admin.userDelete")}
                // Deleting yourself is refused by the server, and offering it
                // here would only be a button that always fails
                disabled={user.id === currentUser?.id}
                onClick={() => {
                  setDeleteError(null)
                  setPendingDelete(user)
                }}
              >
                <HugeiconsIcon icon={UserRemove01Icon} />
              </Button>
            </div>
          </div>
        ))}
      </div>

      <CreateUserDialog
        open={creating}
        onOpenChange={setCreating}
        onCreated={(user) => setUsers((prev) => [...(prev ?? []), user])}
      />

      <EditUserDialog
        user={editing}
        onOpenChange={(next) => {
          if (!next) {
            setEditing(null)
          }
        }}
        onUpdated={(updated) =>
          setUsers((prev) =>
            (prev ?? []).map((u) => (u.id === updated.id ? updated : u))
          )
        }
      />

      <ChangePasswordDialog
        user={changingPassword}
        onOpenChange={(next) => {
          if (!next) {
            setChangingPassword(null)
          }
        }}
      />

      <ConfirmDestructiveDialog
        open={pendingDelete !== null}
        onOpenChange={(next) => {
          if (!next) {
            setPendingDelete(null)
          }
        }}
        title={t("admin.userDeleteTitle")}
        description={t("admin.userDeleteDescription", {
          name: pendingDelete?.name ?? "",
        })}
        errorMessage={deleteError ?? t("admin.userDeleteError")}
        cancelLabel={t("admin.cancel")}
        confirmLabel={t("admin.userDelete")}
        onConfirm={async () => {
          if (pendingDelete) {
            await onDelete(pendingDelete)
          }
        }}
      />
    </section>
  )
}

function CreateUserDialog({
  open,
  onOpenChange,
  onCreated,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  onCreated: (user: User) => void
}) {
  const { t } = useI18n()
  const { createUser } = useAuth()

  const [name, setName] = React.useState("")
  const [username, setUsername] = React.useState("")
  const [password, setPassword] = React.useState("")
  const [isAdmin, setIsAdmin] = React.useState(false)
  const [error, setError] = React.useState<TranslationKey | null>(null)
  const [submitting, setSubmitting] = React.useState(false)

  useResetOnOpen(open, () => {
    setName("")
    setUsername("")
    setPassword("")
    setIsAdmin(false)
    setError(null)
    setSubmitting(false)
  })

  async function onSubmit(event: React.FormEvent) {
    event.preventDefault()
    setError(null)

    if (!name.trim() || !username.trim()) {
      setError("admin.userMissingFields")
      return
    }

    if (password.length < MIN_PASSWORD_LENGTH) {
      setError("admin.userPasswordTooShort")
      return
    }

    setSubmitting(true)
    try {
      const created = await createUser({
        name: name.trim(),
        username: username.trim(),
        password,
        is_system_admin: isAdmin,
      })
      onCreated(created)
      onOpenChange(false)
      toast.success(t("admin.userCreateSuccess", { name: created.name }))
    } catch (err) {
      setError(
        err instanceof ApiError && err.status === 409
          ? "admin.userDuplicated"
          : "admin.userCreateError"
      )
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("admin.userCreateTitle")}</DialogTitle>
          <DialogDescription>
            {t("admin.userCreateDescription")}
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={onSubmit} noValidate>
          <FieldGroup>
            {error && (
              <Alert variant="destructive">
                <AlertDescription>{t(error)}</AlertDescription>
              </Alert>
            )}

            <Field>
              <FieldLabel htmlFor="new-user-name">
                {t("admin.userName")}
              </FieldLabel>
              <Input
                id="new-user-name"
                value={name}
                onChange={(event) => setName(event.target.value)}
                disabled={submitting}
                autoFocus
              />
            </Field>

            <Field>
              <FieldLabel htmlFor="new-user-username">
                {t("admin.userUsername")}
              </FieldLabel>
              <Input
                id="new-user-username"
                autoComplete="off"
                value={username}
                onChange={(event) => setUsername(event.target.value)}
                disabled={submitting}
              />
            </Field>

            <Field>
              <FieldLabel htmlFor="new-user-password">
                {t("admin.userPassword")}
              </FieldLabel>
              <Input
                id="new-user-password"
                type="password"
                autoComplete="new-password"
                placeholder={t("admin.userPasswordPlaceholder")}
                value={password}
                onChange={(event) => setPassword(event.target.value)}
                disabled={submitting}
              />
            </Field>

            <Field orientation="horizontal">
              <Checkbox
                id="new-user-admin"
                checked={isAdmin}
                onCheckedChange={(checked) => setIsAdmin(checked === true)}
                disabled={submitting}
              />
              <Label htmlFor="new-user-admin" className="font-normal">
                {t("admin.userAdminLabel")}
              </Label>
            </Field>

            <Button type="submit" className="w-full" disabled={submitting}>
              {submitting && <Spinner data-icon="inline-start" />}
              {submitting ? t("admin.userSaving") : t("admin.userCreate")}
            </Button>
          </FieldGroup>
        </form>
      </DialogContent>
    </Dialog>
  )
}

function EditUserDialog({
  user,
  onOpenChange,
  onUpdated,
}: {
  user: User | null
  onOpenChange: (open: boolean) => void
  onUpdated: (user: User) => void
}) {
  const { t } = useI18n()
  const { user: currentUser, updateUser } = useAuth()

  const open = user !== null

  const [name, setName] = React.useState("")
  const [isAdmin, setIsAdmin] = React.useState(false)
  const [error, setError] = React.useState<TranslationKey | null>(null)
  const [submitting, setSubmitting] = React.useState(false)

  useResetOnOpen(open, () => {
    setName(user?.name ?? "")
    setIsAdmin(user?.is_system_admin ?? false)
    setError(null)
    setSubmitting(false)
  })

  async function onSubmit(event: React.FormEvent) {
    event.preventDefault()
    setError(null)

    if (!user) {
      return
    }

    if (!name.trim()) {
      setError("admin.userMissingFields")
      return
    }

    setSubmitting(true)
    try {
      const updated = await updateUser(user.id, {
        name: name.trim(),
        is_system_admin: isAdmin,
      })
      onUpdated(updated)
      onOpenChange(false)
      toast.success(t("admin.userUpdateSuccess", { name: updated.name }))
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) {
        setError("admin.userDeleteLastAdmin")
      } else if (err instanceof ApiError && err.status === 403) {
        setError("admin.userCannotDemoteSelf")
      } else {
        setError("admin.userUpdateError")
      }
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("admin.userEditTitle")}</DialogTitle>
          <DialogDescription>
            {t("admin.userEditDescription", { username: user?.username ?? "" })}
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={onSubmit} noValidate>
          <FieldGroup>
            {error && (
              <Alert variant="destructive">
                <AlertDescription>{t(error)}</AlertDescription>
              </Alert>
            )}

            <Field>
              <FieldLabel htmlFor="edit-user-name">
                {t("admin.userName")}
              </FieldLabel>
              <Input
                id="edit-user-name"
                value={name}
                onChange={(event) => setName(event.target.value)}
                disabled={submitting}
                autoFocus
              />
            </Field>

            <Field orientation="horizontal">
              <Checkbox
                id="edit-user-admin"
                checked={isAdmin}
                onCheckedChange={(checked) => setIsAdmin(checked === true)}
                // An administrator demoting themselves loses this page mid-edit
                disabled={submitting || user?.id === currentUser?.id}
              />
              <Label htmlFor="edit-user-admin" className="font-normal">
                {t("admin.userAdminLabel")}
              </Label>
            </Field>

            <Button type="submit" className="w-full" disabled={submitting}>
              {submitting && <Spinner data-icon="inline-start" />}
              {submitting ? t("admin.userSaving") : t("admin.userSave")}
            </Button>
          </FieldGroup>
        </form>
      </DialogContent>
    </Dialog>
  )
}

function ChangePasswordDialog({
  user,
  onOpenChange,
}: {
  user: User | null
  onOpenChange: (open: boolean) => void
}) {
  const { t } = useI18n()
  const { changeUserPassword } = useAuth()

  const open = user !== null

  const [password, setPassword] = React.useState("")
  const [confirmation, setConfirmation] = React.useState("")
  const [error, setError] = React.useState<TranslationKey | null>(null)
  const [submitting, setSubmitting] = React.useState(false)

  useResetOnOpen(open, () => {
    setPassword("")
    setConfirmation("")
    setError(null)
    setSubmitting(false)
  })

  async function onSubmit(event: React.FormEvent) {
    event.preventDefault()
    setError(null)

    if (!user) {
      return
    }

    if (password.length < MIN_PASSWORD_LENGTH) {
      setError("admin.userPasswordTooShort")
      return
    }

    if (password !== confirmation) {
      setError("admin.userPasswordMismatch")
      return
    }

    setSubmitting(true)
    try {
      await changeUserPassword(user.id, password)
      onOpenChange(false)
      toast.success(t("admin.userPasswordSuccess", { name: user.name }))
    } catch {
      setError("admin.userPasswordError")
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("admin.userPasswordTitle")}</DialogTitle>
          <DialogDescription>
            {t("admin.userPasswordDescription", { name: user?.name ?? "" })}
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={onSubmit} noValidate>
          <FieldGroup>
            {error && (
              <Alert variant="destructive">
                <AlertDescription>{t(error)}</AlertDescription>
              </Alert>
            )}

            <Field>
              <FieldLabel htmlFor="reset-password">
                {t("admin.userPassword")}
              </FieldLabel>
              <Input
                id="reset-password"
                type="password"
                autoComplete="new-password"
                placeholder={t("admin.userPasswordPlaceholder")}
                value={password}
                onChange={(event) => setPassword(event.target.value)}
                disabled={submitting}
                autoFocus
              />
            </Field>

            <Field>
              <FieldLabel htmlFor="reset-password-confirmation">
                {t("admin.userPasswordConfirmation")}
              </FieldLabel>
              <Input
                id="reset-password-confirmation"
                type="password"
                autoComplete="new-password"
                value={confirmation}
                onChange={(event) => setConfirmation(event.target.value)}
                disabled={submitting}
              />
            </Field>

            <Button type="submit" className="w-full" disabled={submitting}>
              {submitting && <Spinner data-icon="inline-start" />}
              {submitting ? t("admin.userSaving") : t("admin.userSave")}
            </Button>
          </FieldGroup>
        </form>
      </DialogContent>
    </Dialog>
  )
}

export { UserManagement }
