import * as React from "react"
import { HugeiconsIcon } from "@hugeicons/react"
import { Add01Icon, Delete02Icon } from "@hugeicons/core-free-icons"

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
import { useResetOnOpen } from "@/hooks/use-reset-on-open"
import { Alert, AlertDescription } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Spinner } from "@/components/ui/spinner"
import { useAuth } from "@/lib/auth-context"
import { useI18n } from "@/lib/i18n/context"
import { ApiError, type RecipeCategory } from "@/lib/api"

/**
 * Create, rename and delete the workspace's categories. Renaming is committed
 * on blur rather than behind a save button: the only editable field is the
 * name, so a separate confirmation would be a step with nothing to decide.
 */
function RecipeCategoriesDialog({
  open,
  onOpenChange,
  categories,
  onChanged,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  categories: RecipeCategory[]
  onChanged: (categories: RecipeCategory[]) => void
}) {
  const { createRecipeCategory, updateRecipeCategory, deleteRecipeCategory } =
    useAuth()
  const { t } = useI18n()

  const [name, setName] = React.useState("")
  const [creating, setCreating] = React.useState(false)
  const [error, setError] = React.useState<string | null>(null)
  const [pending, setPending] = React.useState<RecipeCategory | null>(null)
  const [deleting, setDeleting] = React.useState(false)

  useResetOnOpen(open, () => {
    setName("")
    setError(null)
  })

  async function create() {
    const trimmed = name.trim()
    if (trimmed.length < 2 || creating) {
      return
    }

    setCreating(true)
    setError(null)
    try {
      const created = await createRecipeCategory(trimmed)
      onChanged(
        [...categories, created].sort((a, b) => a.name.localeCompare(b.name))
      )
      setName("")
    } catch (err) {
      setError(
        (err as ApiError)?.status === 409
          ? t("recipe.categoryExists")
          : t("recipe.categoryCreateError")
      )
    } finally {
      setCreating(false)
    }
  }

  async function rename(category: RecipeCategory, next: string) {
    const trimmed = next.trim()
    if (trimmed.length < 2 || trimmed === category.name) {
      return
    }

    setError(null)
    try {
      const updated = await updateRecipeCategory(category.id, trimmed)
      onChanged(
        categories
          .map((item) => (item.id === updated.id ? updated : item))
          .sort((a, b) => a.name.localeCompare(b.name))
      )
    } catch (err) {
      setError(
        (err as ApiError)?.status === 409
          ? t("recipe.categoryExists")
          : t("recipe.categoryRenameError")
      )
    }
  }

  async function confirmDelete() {
    if (!pending) {
      return
    }

    setDeleting(true)
    setError(null)
    try {
      await deleteRecipeCategory(pending.id)
      onChanged(categories.filter((item) => item.id !== pending.id))
      setPending(null)
    } catch {
      setError(t("recipe.categoryDeleteError"))
    } finally {
      setDeleting(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{t("recipe.categoriesTitle")}</DialogTitle>
          <DialogDescription>
            {t("recipe.categoriesDialogDescription")}
          </DialogDescription>
        </DialogHeader>

        <div className="flex flex-col gap-4">
          {error && (
            <Alert variant="destructive">
              <AlertDescription>{error}</AlertDescription>
            </Alert>
          )}

          <div className="flex items-center gap-2">
            <Input
              value={name}
              maxLength={100}
              placeholder={t("recipe.newCategoryPlaceholder")}
              disabled={creating}
              onChange={(event) => setName(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === "Enter") {
                  event.preventDefault()
                  create()
                }
              }}
            />
            <Button
              type="button"
              disabled={creating || name.trim().length < 2}
              onClick={create}
              className="shrink-0"
            >
              {creating ? (
                <Spinner data-icon="inline-start" />
              ) : (
                <HugeiconsIcon icon={Add01Icon} data-icon="inline-start" />
              )}
              {t("recipe.addCategory")}
            </Button>
          </div>

          {categories.length === 0 ? (
            <p className="py-4 text-center text-sm text-muted-foreground">
              {t("recipe.noCategories")}
            </p>
          ) : (
            <ul className="flex max-h-72 flex-col gap-2 overflow-y-auto">
              {categories.map((category) => (
                <li key={category.id} className="flex items-center gap-2">
                  <Input
                    // Keyed on the name as well as the id: the field is
                    // uncontrolled, so a successful rename has to remount it
                    // rather than hand a live input a new defaultValue.
                    key={`${category.id}:${category.name}`}
                    defaultValue={category.name}
                    maxLength={100}
                    onBlur={(event) => rename(category, event.target.value)}
                    onKeyDown={(event) => {
                      if (event.key === "Enter") {
                        event.currentTarget.blur()
                      }
                    }}
                    className="min-w-0 flex-1"
                  />
                  <span className="w-20 shrink-0 text-right text-xs text-muted-foreground tabular-nums">
                    {category.recipe_count === 1
                      ? t("recipe.categoryRecipeCountOne")
                      : t("recipe.categoryRecipeCount", {
                          count: category.recipe_count,
                        })}
                  </span>
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon-sm"
                    onClick={() => setPending(category)}
                    className="shrink-0 text-muted-foreground"
                  >
                    <HugeiconsIcon icon={Delete02Icon} />
                    <span className="sr-only">
                      {t("recipe.categoryDelete")}
                    </span>
                  </Button>
                </li>
              ))}
            </ul>
          )}
        </div>

        <AlertDialog
          open={pending !== null}
          onOpenChange={(next) => {
            if (!next) {
              setPending(null)
            }
          }}
        >
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle>
                {t("recipe.categoryDeleteTitle")}
              </AlertDialogTitle>
              <AlertDialogDescription>
                {t("recipe.categoryDeleteDescription", {
                  name: pending?.name ?? "",
                })}
              </AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel disabled={deleting}>
                {t("recipe.cancel")}
              </AlertDialogCancel>
              <AlertDialogAction disabled={deleting} onClick={confirmDelete}>
                {deleting && <Spinner data-icon="inline-start" />}
                {t("recipe.categoryDelete")}
              </AlertDialogAction>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>
      </DialogContent>
    </Dialog>
  )
}

export { RecipeCategoriesDialog }
