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
import { useAuth } from "@/lib/auth-context"
import { useI18n } from "@/lib/i18n/context"

export default function LoginPage() {
  const navigate = useNavigate()
  const { status, setupRequired, login } = useAuth()
  const { t } = useI18n()

  const [username, setUsername] = React.useState("")
  const [password, setPassword] = React.useState("")
  const [hasError, setHasError] = React.useState(false)
  const [submitting, setSubmitting] = React.useState(false)

  React.useEffect(() => {
    if (setupRequired) {
      navigate("/setup", { replace: true })
    } else if (status === "authenticated") {
      navigate("/", { replace: true })
    }
  }, [status, setupRequired, navigate])

  async function onSubmit(event: React.FormEvent) {
    event.preventDefault()
    setHasError(false)

    if (!username.trim() || !password) {
      setHasError(true)
      return
    }

    setSubmitting(true)
    try {
      await login(username, password)
      navigate("/", { replace: true })
    } catch {
      setHasError(true)
    } finally {
      setSubmitting(false)
    }
  }

  // setupRequired is on its way to /setup, showing a sign-in form nobody has an
  // account for would only be a flash of the wrong screen
  if (status === "loading" || status === "authenticated" || setupRequired) {
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
          <CardTitle>{t("login.title")}</CardTitle>
          <CardDescription>{t("login.description")}</CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={onSubmit} noValidate>
            <FieldGroup>
              {hasError && (
                <Alert variant="destructive">
                  <HugeiconsIcon icon={Alert01Icon} />
                  <AlertDescription>{t("login.error")}</AlertDescription>
                </Alert>
              )}

              <Field>
                <FieldLabel htmlFor="username">{t("login.username")}</FieldLabel>
                <Input
                  id="username"
                  name="username"
                  autoComplete="username"
                  placeholder={t("login.usernamePlaceholder")}
                  value={username}
                  onChange={(event) => setUsername(event.target.value)}
                  disabled={submitting}
                  autoFocus
                />
              </Field>

              <Field>
                <FieldLabel htmlFor="password">{t("login.password")}</FieldLabel>
                <Input
                  id="password"
                  name="password"
                  type="password"
                  autoComplete="current-password"
                  placeholder={t("login.passwordPlaceholder")}
                  value={password}
                  onChange={(event) => setPassword(event.target.value)}
                  disabled={submitting}
                />
              </Field>

              <Button type="submit" className="w-full" disabled={submitting}>
                {submitting && <Spinner data-icon="inline-start" />}
                {submitting ? t("login.submitting") : t("login.submit")}
              </Button>
            </FieldGroup>
          </form>
        </CardContent>
      </Card>
    </div>
  )
}
