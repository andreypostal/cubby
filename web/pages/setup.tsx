import * as React from "react"
import { useNavigate } from "react-router"
import { HugeiconsIcon } from "@hugeicons/react"
import { Alert01Icon } from "@hugeicons/core-free-icons"

import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Field, FieldGroup, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Alert, AlertDescription } from "@/components/ui/alert"
import { Spinner } from "@/components/ui/spinner"
import { LanguageSwitcher } from "@/components/language-switcher"
import { ThemeToggle } from "@/components/theme-toggle"
import { ApiError, useAuth } from "@/lib/auth-context"
import { useI18n } from "@/lib/i18n/context"
import type { TranslationKey } from "@/lib/i18n/translations"

/** Matches the minimum the API enforces, so the server never has to say no. */
const MIN_PASSWORD_LENGTH = 8

/**
 * The first-run screen. A server that has never been set up prints a token to
 * its log; whoever can read that log proves they own the deployment and gets to
 * create the first administrator here.
 */
export default function SetupPage() {
  const navigate = useNavigate()
  const { status, setupRequired, completeSetup } = useAuth()
  const { t } = useI18n()

  const [token, setToken] = React.useState("")
  const [name, setName] = React.useState("")
  const [username, setUsername] = React.useState("")
  const [password, setPassword] = React.useState("")
  const [confirmation, setConfirmation] = React.useState("")
  const [error, setError] = React.useState<TranslationKey | null>(null)
  const [submitting, setSubmitting] = React.useState(false)

  React.useEffect(() => {
    if (status !== "loading" && !setupRequired) {
      navigate("/login", { replace: true })
    }
  }, [status, setupRequired, navigate])

  async function onSubmit(event: React.FormEvent) {
    event.preventDefault()
    setError(null)

    if (!token.trim() || !name.trim() || !username.trim()) {
      setError("setup.missingFields")
      return
    }

    if (password.length < MIN_PASSWORD_LENGTH) {
      setError("setup.passwordTooShort")
      return
    }

    if (password !== confirmation) {
      setError("setup.passwordMismatch")
      return
    }

    setSubmitting(true)
    try {
      await completeSetup(token.trim(), name.trim(), username.trim(), password)
      navigate("/", { replace: true })
    } catch (err) {
      if (err instanceof ApiError && err.status === 403) {
        setError("setup.invalidToken")
      } else if (err instanceof ApiError && err.status === 409) {
        setError("setup.alreadyDone")
      } else {
        setError("setup.error")
      }
    } finally {
      setSubmitting(false)
    }
  }

  if (status === "loading" || !setupRequired) {
    return (
      <div className="flex min-h-svh items-center justify-center">
        <Spinner className="size-6" />
      </div>
    )
  }

  return (
    <div className="flex min-h-svh items-center justify-center p-6">
      <div className="absolute top-4 right-4 flex gap-2">
        <LanguageSwitcher />
        <ThemeToggle />
      </div>

      <Card className="w-full max-w-sm">
        <CardHeader>
          <CardTitle>{t("setup.title")}</CardTitle>
          <CardDescription>{t("setup.description")}</CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={onSubmit} noValidate>
            <FieldGroup>
              {error && (
                <Alert variant="destructive">
                  <HugeiconsIcon icon={Alert01Icon} />
                  <AlertDescription>{t(error)}</AlertDescription>
                </Alert>
              )}

              <Field>
                <FieldLabel htmlFor="token">{t("setup.token")}</FieldLabel>
                <Input
                  id="token"
                  name="token"
                  autoComplete="off"
                  spellCheck={false}
                  className="font-mono"
                  placeholder={t("setup.tokenPlaceholder")}
                  value={token}
                  onChange={(event) => setToken(event.target.value)}
                  disabled={submitting}
                  autoFocus
                />
              </Field>

              <Field>
                <FieldLabel htmlFor="name">{t("setup.name")}</FieldLabel>
                <Input
                  id="name"
                  name="name"
                  autoComplete="name"
                  placeholder={t("setup.namePlaceholder")}
                  value={name}
                  onChange={(event) => setName(event.target.value)}
                  disabled={submitting}
                />
              </Field>

              <Field>
                <FieldLabel htmlFor="username">{t("setup.username")}</FieldLabel>
                <Input
                  id="username"
                  name="username"
                  autoComplete="username"
                  placeholder={t("setup.usernamePlaceholder")}
                  value={username}
                  onChange={(event) => setUsername(event.target.value)}
                  disabled={submitting}
                />
              </Field>

              <Field>
                <FieldLabel htmlFor="password">{t("setup.password")}</FieldLabel>
                <Input
                  id="password"
                  name="password"
                  type="password"
                  autoComplete="new-password"
                  placeholder={t("setup.passwordPlaceholder")}
                  value={password}
                  onChange={(event) => setPassword(event.target.value)}
                  disabled={submitting}
                />
              </Field>

              <Field>
                <FieldLabel htmlFor="confirmation">
                  {t("setup.confirmation")}
                </FieldLabel>
                <Input
                  id="confirmation"
                  name="confirmation"
                  type="password"
                  autoComplete="new-password"
                  placeholder={t("setup.confirmationPlaceholder")}
                  value={confirmation}
                  onChange={(event) => setConfirmation(event.target.value)}
                  disabled={submitting}
                />
              </Field>

              <Button type="submit" className="w-full" disabled={submitting}>
                {submitting && <Spinner data-icon="inline-start" />}
                {submitting ? t("setup.submitting") : t("setup.submit")}
              </Button>
            </FieldGroup>
          </form>
        </CardContent>
      </Card>
    </div>
  )
}
