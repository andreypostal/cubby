import * as React from "react"
import { Link, useNavigate, useParams } from "react-router"
import { toast } from "sonner"
import { HugeiconsIcon } from "@hugeicons/react"
import {
  ArrowLeft01Icon,
  Archive02Icon,
  Delete02Icon,
  Edit02Icon,
  FavouriteIcon,
  MoreHorizontalIcon,
  ShoppingCart01Icon,
  Time04Icon,
} from "@hugeicons/core-free-icons"

import { cn } from "@/lib/utils"
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
import { Alert, AlertDescription } from "@/components/ui/alert"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { AddToListDialog } from "@/components/add-to-list-dialog"
import { CreateListDialog } from "@/components/create-list-dialog"
import { MarkdownView } from "@/components/markdown-editor"
import { Separator } from "@/components/ui/separator"
import { Skeleton } from "@/components/ui/skeleton"
import { Spinner } from "@/components/ui/spinner"
import { useAuth } from "@/lib/auth-context"
import { useI18n } from "@/lib/i18n/context"
import { durationLabel } from "@/lib/recipes"
import type { RecipeDetail, ShopList } from "@/lib/api"

export default function RecipeDetailPage() {
  const params = useParams<{ recipeID: string }>()
  const recipeId = Number(params.recipeID)

  const navigate = useNavigate()
  const {
    getRecipe,
    setRecipeFavorite,
    archiveRecipe,
    unarchiveRecipe,
    deleteRecipe,
    createListItem,
    updateListItem,
  } = useAuth()
  const { t } = useI18n()

  const [recipe, setRecipe] = React.useState<RecipeDetail | null>(null)
  const [fetchFailed, setFetchFailed] = React.useState(false)
  const [busy, setBusy] = React.useState(false)
  const [archiveOpen, setArchiveOpen] = React.useState(false)
  const [deleteOpen, setDeleteOpen] = React.useState(false)
  const [pickListOpen, setPickListOpen] = React.useState(false)
  const [createListOpen, setCreateListOpen] = React.useState(false)

  // A non-numeric path segment is a bad URL, not a failed request, so it is
  // derived rather than stored.
  const invalidId = !Number.isFinite(recipeId)
  const loadError = invalidId || fetchFailed

  React.useEffect(() => {
    if (!Number.isFinite(recipeId)) {
      return
    }

    let cancelled = false

    getRecipe(recipeId)
      .then((loaded) => {
        if (!cancelled) {
          setRecipe(loaded)
          setFetchFailed(false)
        }
      })
      .catch(() => {
        if (!cancelled) {
          setFetchFailed(true)
        }
      })

    return () => {
      cancelled = true
    }
  }, [getRecipe, recipeId])

  async function toggleFavorite() {
    if (!recipe || busy) {
      return
    }

    setBusy(true)
    try {
      setRecipe(await setRecipeFavorite(recipe.id, !recipe.favorite))
    } catch {
      toast.error(t("recipe.favoriteError"))
    } finally {
      setBusy(false)
    }
  }

  async function archive() {
    if (!recipe) {
      return
    }

    setBusy(true)
    try {
      setRecipe(await archiveRecipe(recipe.id))
      setArchiveOpen(false)
      toast.success(t("recipe.archiveSuccess"))
    } catch {
      toast.error(t("recipe.archiveError"))
    } finally {
      setBusy(false)
    }
  }

  async function unarchive() {
    if (!recipe) {
      return
    }

    setBusy(true)
    try {
      setRecipe(await unarchiveRecipe(recipe.id))
      toast.success(t("recipe.unarchiveSuccess"))
    } catch {
      toast.error(t("recipe.unarchiveError"))
    } finally {
      setBusy(false)
    }
  }

  /**
   * The item endpoint only takes a name on creation, so a quantity lands in
   * the note with a follow-up call. One ingredient at a time, in the recipe's
   * own order, since the list shows items in the order they were created.
   */
  async function addIngredientsTo(list: ShopList) {
    if (!recipe) {
      return
    }

    setPickListOpen(false)
    setBusy(true)
    try {
      for (const ingredient of recipe.ingredients) {
        const item = await createListItem(list.id, ingredient.name)
        const quantity = ingredient.quantity.trim()
        if (quantity) {
          await updateListItem(list.id, item.id, { notes: quantity })
        }
      }
      toast.success(t("recipe.addToListSuccess", { name: list.name }))
    } catch {
      toast.error(t("recipe.addToListError"))
    } finally {
      setBusy(false)
    }
  }

  async function remove() {
    if (!recipe) {
      return
    }

    setBusy(true)
    try {
      await deleteRecipe(recipe.id)
      toast.success(t("recipe.deleteSuccess"))
      navigate("/recipes", { replace: true })
    } catch {
      toast.error(t("recipe.deleteError"))
      setBusy(false)
    }
  }

  if (loadError) {
    return (
      <div className="mx-auto flex w-full max-w-3xl flex-col gap-4">
        <BackLink />
        <Alert variant="destructive">
          <AlertDescription>{t("recipe.detailLoadError")}</AlertDescription>
        </Alert>
      </div>
    )
  }

  if (!recipe) {
    return (
      <div className="mx-auto flex w-full max-w-3xl flex-col gap-4">
        <BackLink />
        <Skeleton className="h-9 w-2/3 rounded-xl" />
        <Skeleton className="h-24 w-full rounded-2xl" />
        <Skeleton className="h-48 w-full rounded-2xl" />
      </div>
    )
  }

  const duration = durationLabel(recipe.estimated_minutes)

  return (
    <div className={cn("mx-auto flex w-full max-w-3xl flex-col gap-6", busy && "opacity-70")}>
      <BackLink />

      {recipe.archived && (
        <Alert>
          <AlertDescription className="flex flex-wrap items-center justify-between gap-2">
            <span>{t("recipe.archivedNotice")}</span>
            <Button
              type="button"
              variant="outline"
              size="sm"
              disabled={busy}
              onClick={unarchive}
            >
              {t("recipe.unarchive")}
            </Button>
          </AlertDescription>
        </Alert>
      )}

      <header className="flex flex-col gap-3">
        <div className="flex items-start justify-between gap-3">
          <h1 className="min-w-0 font-heading text-2xl font-semibold break-words">
            {recipe.name}
          </h1>
          <div className="flex shrink-0 items-center gap-1">
            <Button
              type="button"
              variant="ghost"
              size="icon"
              disabled={busy}
              onClick={toggleFavorite}
              title={
                recipe.favorite
                  ? t("recipe.removeFavorite")
                  : t("recipe.addFavorite")
              }
              className={cn(
                recipe.favorite
                  ? "text-rose-600 dark:text-rose-400"
                  : "text-muted-foreground"
              )}
            >
              <HugeiconsIcon
                icon={FavouriteIcon}
                fill={recipe.favorite ? "currentColor" : "none"}
              />
              <span className="sr-only">
                {recipe.favorite
                  ? t("recipe.removeFavorite")
                  : t("recipe.addFavorite")}
              </span>
            </Button>

            <Button
              variant="outline"
              size="sm"
              nativeButton={false}
              render={<Link to={`/recipes/${recipe.id}/edit`} />}
            >
              <HugeiconsIcon icon={Edit02Icon} data-icon="inline-start" />
              <span className="hidden sm:inline">{t("recipe.edit")}</span>
            </Button>

            <DropdownMenu>
              <DropdownMenuTrigger
                render={
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon"
                    className="text-muted-foreground"
                  />
                }
              >
                <HugeiconsIcon icon={MoreHorizontalIcon} />
                <span className="sr-only">{t("recipe.actions")}</span>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end" className="w-auto min-w-52">
                <DropdownMenuItem
                  disabled={busy || recipe.ingredients.length === 0}
                  onClick={() => setPickListOpen(true)}
                >
                  <HugeiconsIcon icon={ShoppingCart01Icon} />
                  {t("recipe.addToList")}
                </DropdownMenuItem>
                {recipe.ingredients.length === 0 && (
                  <p className="px-2 py-1.5 text-xs text-muted-foreground">
                    {t("recipe.addToListNoIngredients")}
                  </p>
                )}
                <DropdownMenuSeparator />
                {recipe.archived ? (
                  <DropdownMenuItem disabled={busy} onClick={unarchive}>
                    <HugeiconsIcon icon={Archive02Icon} />
                    {t("recipe.unarchive")}
                  </DropdownMenuItem>
                ) : (
                  <DropdownMenuItem
                    disabled={busy}
                    onClick={() => setArchiveOpen(true)}
                  >
                    <HugeiconsIcon icon={Archive02Icon} />
                    {t("recipe.archive")}
                  </DropdownMenuItem>
                )}
                <DropdownMenuSeparator />
                {/* Deleting is only reachable once archived, matching what
                    the API accepts — offering it earlier would only produce a
                    rejection the user cannot act on. */}
                <DropdownMenuItem
                  variant="destructive"
                  disabled={busy || !recipe.archived}
                  onClick={() => setDeleteOpen(true)}
                >
                  <HugeiconsIcon icon={Delete02Icon} />
                  {t("recipe.delete")}
                </DropdownMenuItem>
                {!recipe.archived && (
                  <p className="px-2 py-1.5 text-xs text-muted-foreground">
                    {t("recipe.deleteNeedsArchive")}
                  </p>
                )}
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
        </div>

        <p className="text-sm text-muted-foreground">
          {recipe.description || t("recipe.noDescription")}
        </p>

        <div className="flex flex-wrap items-center gap-2">
          <Badge variant="outline" className="gap-1.5">
            <HugeiconsIcon icon={Time04Icon} size={14} />
            {t(duration.key, duration.vars)}
          </Badge>
          {recipe.archived && (
            <Badge variant="secondary">{t("recipe.archivedBadge")}</Badge>
          )}
          {recipe.categories.map((category) => (
            <Badge key={category.id} variant="outline">
              {category.name}
            </Badge>
          ))}
        </div>
      </header>

      <Separator />

      <section className="flex flex-col gap-3">
        <h2 className="font-heading text-lg font-semibold">
          {t("recipe.ingredientsTitle")}
        </h2>
        {recipe.ingredients.length === 0 ? (
          <p className="text-sm text-muted-foreground">
            {t("recipe.noIngredients")}
          </p>
        ) : (
          <ul className="flex flex-col overflow-hidden rounded-2xl bg-card ring-1 ring-foreground/10">
            {recipe.ingredients.map((ingredient, index) => (
              <li
                key={ingredient.id}
                className={cn(
                  "flex items-center justify-between gap-3 px-4 py-2.5 text-sm",
                  index > 0 && "border-t border-border/60"
                )}
              >
                <span className="min-w-0 break-words">{ingredient.name}</span>
                {ingredient.quantity && (
                  <span className="shrink-0 text-muted-foreground tabular-nums">
                    {ingredient.quantity}
                  </span>
                )}
              </li>
            ))}
          </ul>
        )}
      </section>

      <section className="flex flex-col gap-3">
        <h2 className="font-heading text-lg font-semibold">
          {t("recipe.stepsTitle")}
        </h2>
        {recipe.steps.length === 0 ? (
          <p className="text-sm text-muted-foreground">{t("recipe.noSteps")}</p>
        ) : (
          // Read top to bottom, one step after the other — the same order the
          // cooking happens in. Nothing here collapses.
          <ol className="flex flex-col gap-4">
            {recipe.steps.map((step, index) => (
              <li key={step.id} className="flex gap-3">
                <div className="flex flex-col items-center gap-2">
                  <span className="flex size-8 shrink-0 items-center justify-center rounded-full bg-primary text-sm font-semibold text-primary-foreground tabular-nums">
                    {index + 1}
                  </span>
                  {index < recipe.steps.length - 1 && (
                    <span
                      className="w-px flex-1 bg-border"
                      aria-hidden
                    />
                  )}
                </div>
                <div className="min-w-0 flex-1 pb-2">
                  <h3 className="font-heading text-base font-semibold break-words">
                    {step.title}
                  </h3>
                  {step.description.trim() ? (
                    <MarkdownView
                      value={step.description}
                      className="mt-1 text-muted-foreground"
                    />
                  ) : (
                    <p className="mt-1 text-sm text-muted-foreground/60">
                      {t("recipe.stepNoDescription")}
                    </p>
                  )}
                </div>
              </li>
            ))}
          </ol>
        )}
      </section>

      <AddToListDialog
        open={pickListOpen}
        onOpenChange={setPickListOpen}
        onSelect={addIngredientsTo}
        onCreateNew={() => {
          // Handing over rather than stacking: the picker closes so the
          // creation dialog is the only thing on screen.
          setPickListOpen(false)
          setCreateListOpen(true)
        }}
      />

      <CreateListDialog
        open={createListOpen}
        onOpenChange={setCreateListOpen}
        onCreated={addIngredientsTo}
        defaultName={recipe.name}
      />

      <AlertDialog open={archiveOpen} onOpenChange={setArchiveOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("recipe.archiveTitle")}</AlertDialogTitle>
            <AlertDialogDescription>
              {t("recipe.archiveDescription", { name: recipe.name })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={busy}>
              {t("recipe.cancel")}
            </AlertDialogCancel>
            <AlertDialogAction disabled={busy} onClick={archive}>
              {busy && <Spinner data-icon="inline-start" />}
              {t("recipe.archiveConfirm")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AlertDialog open={deleteOpen} onOpenChange={setDeleteOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("recipe.deleteTitle")}</AlertDialogTitle>
            <AlertDialogDescription>
              {t("recipe.deleteDescription", { name: recipe.name })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={busy}>
              {t("recipe.cancel")}
            </AlertDialogCancel>
            <AlertDialogAction disabled={busy} onClick={remove}>
              {busy && <Spinner data-icon="inline-start" />}
              {busy ? t("recipe.deleting") : t("recipe.deleteConfirm")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}

function BackLink() {
  const { t } = useI18n()

  return (
    <Button
      variant="ghost"
      size="sm"
      nativeButton={false}
      render={<Link to="/recipes" />}
      className="-ml-2 self-start text-muted-foreground"
    >
      <HugeiconsIcon icon={ArrowLeft01Icon} data-icon="inline-start" />
      {t("recipe.back")}
    </Button>
  )
}
