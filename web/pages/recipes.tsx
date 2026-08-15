import * as React from "react"
import { Link } from "react-router"
import { toast } from "sonner"
import { HugeiconsIcon } from "@hugeicons/react"
import {
  Add01Icon,
  ArrowLeft01Icon,
  ArrowRight01Icon,
  Cancel01Icon,
  ChefHatIcon,
  FavouriteIcon,
  Search01Icon,
  Settings02Icon,
  Time04Icon,
} from "@hugeicons/core-free-icons"

import { cn } from "@/lib/utils"
import { Alert, AlertDescription } from "@/components/ui/alert"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { RecipeCategoriesDialog } from "@/components/recipe-categories-dialog"
import { Skeleton } from "@/components/ui/skeleton"
import { useAuth } from "@/lib/auth-context"
import { useI18n } from "@/lib/i18n/context"
import { durationLabel } from "@/lib/recipes"
import type {
  Recipe,
  RecipeArchivedFilter,
  RecipeCategory,
  RecipePagination,
} from "@/lib/api"

const PER_PAGE = 12

export default function RecipesPage() {
  const { workspace } = useAuth()

  // Remounts (and re-fetches from a clean state) whenever the workspace changes.
  return <RecipesList key={workspace?.id} />
}

function RecipesList() {
  const { listRecipes, listRecipeCategories, setRecipeFavorite } = useAuth()
  const { t } = useI18n()

  const [categories, setCategories] = React.useState<RecipeCategory[]>([])
  const [categoriesOpen, setCategoriesOpen] = React.useState(false)

  const [search, setSearch] = React.useState("")
  // The applied term lags the typed one so a listing is not refetched on every
  // keystroke.
  const [appliedSearch, setAppliedSearch] = React.useState("")
  const [categoryIds, setCategoryIds] = React.useState<number[]>([])
  const [archived, setArchived] = React.useState<RecipeArchivedFilter>("exclude")
  const [favoritesOnly, setFavoritesOnly] = React.useState(false)
  const [page, setPage] = React.useState(1)

  // The whole query in one string. Comparing it against the one the current
  // result was fetched for is what tells a stale render from a fresh one,
  // without an extra "loading" flag that would have to be flipped on the way in.
  const queryKey = JSON.stringify({
    appliedSearch,
    categoryIds,
    archived,
    favoritesOnly,
    page,
  })

  const [result, setResult] = React.useState<{
    key: string
    recipes: Recipe[]
    pagination: RecipePagination | null
    failed: boolean
  } | null>(null)

  React.useEffect(() => {
    const timer = setTimeout(() => {
      setAppliedSearch(search.trim())
      setPage(1)
    }, 300)
    return () => clearTimeout(timer)
  }, [search])

  React.useEffect(() => {
    listRecipeCategories()
      .then(setCategories)
      .catch(() => setCategories([]))
  }, [listRecipeCategories])

  React.useEffect(() => {
    let cancelled = false

    listRecipes({
      search: appliedSearch,
      categoryIds,
      archived,
      favorite: favoritesOnly,
      page,
      perPage: PER_PAGE,
    })
      .then((res) => {
        if (!cancelled) {
          setResult({
            key: queryKey,
            recipes: res.recipes,
            pagination: res.pagination,
            failed: false,
          })
        }
      })
      .catch(() => {
        if (!cancelled) {
          setResult({
            key: queryKey,
            recipes: [],
            pagination: null,
            failed: true,
          })
        }
      })

    return () => {
      cancelled = true
    }
  }, [listRecipes, appliedSearch, categoryIds, archived, favoritesOnly, page, queryKey])

  const fresh = result?.key === queryKey ? result : null
  const recipes = fresh?.recipes ?? null
  const pagination = fresh?.pagination ?? null
  const loadError = fresh?.failed ?? false

  // Any filter change invalidates the current page number: page 3 of the old
  // result set is rarely a page of the new one.
  function toggleCategory(id: number) {
    setPage(1)
    setCategoryIds((current) =>
      current.includes(id)
        ? current.filter((item) => item !== id)
        : [...current, id]
    )
  }

  function patchFavorite(recipeId: number, favorite: boolean) {
    setResult((current) =>
      current
        ? {
            ...current,
            recipes: current.recipes.map((item) =>
              item.id === recipeId ? { ...item, favorite } : item
            ),
          }
        : current
    )
  }

  async function toggleFavorite(recipe: Recipe) {
    // Flipped locally first: waiting on the round trip makes the tap feel
    // broken, and the listing is re-sorted only on the next fetch anyway.
    patchFavorite(recipe.id, !recipe.favorite)

    try {
      await setRecipeFavorite(recipe.id, !recipe.favorite)
    } catch {
      toast.error(t("recipe.favoriteError"))
      patchFavorite(recipe.id, recipe.favorite)
    }
  }

  const filterCount =
    categoryIds.length +
    (archived !== "exclude" ? 1 : 0) +
    (favoritesOnly ? 1 : 0) +
    (appliedSearch ? 1 : 0)

  const totalPages = pagination?.total_pages ?? 0

  return (
    <div className="mx-auto flex w-full max-w-4xl flex-col gap-5">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <p className="text-xs font-medium tracking-wide text-muted-foreground uppercase">
            {t("recipe.listEyebrow")}
          </p>
          <h1 className="font-heading text-2xl font-semibold">
            {t("recipe.listTitle")}
          </h1>
        </div>
        <Button
          nativeButton={false}
          render={<Link to="/recipes/new" />}
          className="shrink-0"
        >
          <HugeiconsIcon icon={Add01Icon} data-icon="inline-start" />
          <span className="hidden sm:inline">{t("recipe.new")}</span>
        </Button>
      </div>

      <div className="flex flex-col gap-3">
        <div className="relative">
          <HugeiconsIcon
            icon={Search01Icon}
            size={16}
            className="pointer-events-none absolute top-1/2 left-3 -translate-y-1/2 text-muted-foreground"
          />
          <Input
            value={search}
            placeholder={t("recipe.searchPlaceholder")}
            onChange={(event) => setSearch(event.target.value)}
            className="pl-9"
          />
        </div>

        {/* Scrolls sideways on a phone instead of wrapping into a wall of
            chips that pushes the results off screen; on a wider screen there
            is room to wrap, and clipping "Manage categories" would hide the
            only way in. */}
        <div className="-mx-4 flex gap-2 overflow-x-auto px-4 pb-1 [scrollbar-width:none] sm:mx-0 sm:flex-wrap sm:overflow-visible sm:px-0 [&::-webkit-scrollbar]:hidden">
          <FilterChip
            active={favoritesOnly}
            onClick={() => {
              setPage(1)
              setFavoritesOnly((current) => !current)
            }}
            icon={FavouriteIcon}
          >
            {t("recipe.onlyFavorites")}
          </FilterChip>
          <FilterChip
            active={archived === "include"}
            onClick={() => {
              setPage(1)
              setArchived((current) =>
                current === "include" ? "exclude" : "include"
              )
            }}
          >
            {t("recipe.showArchived")}
          </FilterChip>
          <FilterChip
            active={archived === "only"}
            onClick={() => {
              setPage(1)
              setArchived((current) =>
                current === "only" ? "exclude" : "only"
              )
            }}
          >
            {t("recipe.onlyArchived")}
          </FilterChip>

          {categories.length > 0 && (
            <span className="my-1 w-px shrink-0 bg-border" aria-hidden />
          )}

          {categories.map((category) => (
            <FilterChip
              key={category.id}
              active={categoryIds.includes(category.id)}
              onClick={() => toggleCategory(category.id)}
            >
              {category.name}
              <span className="tabular-nums opacity-60">
                {category.recipe_count}
              </span>
            </FilterChip>
          ))}

          <button
            type="button"
            onClick={() => setCategoriesOpen(true)}
            className="inline-flex h-8 shrink-0 cursor-pointer items-center gap-1.5 rounded-full border border-dashed border-border px-3 text-sm text-muted-foreground transition-colors hover:bg-muted"
          >
            <HugeiconsIcon icon={Settings02Icon} size={14} />
            {t("recipe.manageCategories")}
          </button>
        </div>

        <div className="flex items-center gap-2">
          {pagination && (
            <span className="text-xs text-muted-foreground">
              {pagination.total === 1
                ? t("recipe.resultsOne")
                : t("recipe.results", { total: pagination.total })}
            </span>
          )}
          {filterCount > 0 && (
            <Button
              type="button"
              variant="ghost"
              size="xs"
              onClick={() => {
                setSearch("")
                setAppliedSearch("")
                setCategoryIds([])
                setArchived("exclude")
                setFavoritesOnly(false)
                setPage(1)
              }}
              className="text-muted-foreground"
            >
              <HugeiconsIcon icon={Cancel01Icon} data-icon="inline-start" />
              {t("recipe.clearFilters")}
            </Button>
          )}
        </div>
      </div>

      {loadError && (
        <Alert variant="destructive">
          <AlertDescription>{t("recipe.loadError")}</AlertDescription>
        </Alert>
      )}

      {!recipes && !loadError && (
        <div className="grid gap-3 sm:grid-cols-2">
          {Array.from({ length: 4 }).map((_, index) => (
            <Skeleton key={index} className="h-32 w-full rounded-2xl" />
          ))}
        </div>
      )}

      {recipes && recipes.length === 0 && !loadError && (
        <div className="flex flex-col items-center gap-3 rounded-2xl border border-dashed border-border px-6 py-12 text-center">
          <HugeiconsIcon
            icon={ChefHatIcon}
            size={28}
            className="text-muted-foreground"
          />
          <p className="text-sm text-muted-foreground">
            {filterCount > 0 ? t("recipe.emptyFiltered") : t("recipe.empty")}
          </p>
          {filterCount === 0 && (
            <Button
              variant="outline"
              nativeButton={false}
              render={<Link to="/recipes/new" />}
            >
              <HugeiconsIcon icon={Add01Icon} data-icon="inline-start" />
              {t("recipe.new")}
            </Button>
          )}
        </div>
      )}

      {recipes && recipes.length > 0 && (
        <ul className="grid gap-3 sm:grid-cols-2">
          {recipes.map((recipe) => (
            <RecipeCard
              key={recipe.id}
              recipe={recipe}
              onToggleFavorite={() => toggleFavorite(recipe)}
            />
          ))}
        </ul>
      )}

      {pagination && totalPages > 1 && (
        <div className="flex items-center justify-between gap-3">
          <Button
            type="button"
            variant="outline"
            size="sm"
            disabled={page <= 1}
            onClick={() => setPage((current) => Math.max(1, current - 1))}
          >
            <HugeiconsIcon icon={ArrowLeft01Icon} data-icon="inline-start" />
            {t("recipe.pagePrevious")}
          </Button>
          <span className="text-xs text-muted-foreground tabular-nums">
            {t("recipe.pageOf", { page, total: totalPages })}
          </span>
          <Button
            type="button"
            variant="outline"
            size="sm"
            disabled={page >= totalPages}
            onClick={() => setPage((current) => current + 1)}
          >
            {t("recipe.pageNext")}
            <HugeiconsIcon icon={ArrowRight01Icon} data-icon="inline-end" />
          </Button>
        </div>
      )}

      <RecipeCategoriesDialog
        open={categoriesOpen}
        onOpenChange={setCategoriesOpen}
        categories={categories}
        onChanged={(next) => {
          setCategories(next)
          // A deleted category must stop filtering, or the listing would keep
          // asking for an id the server no longer knows.
          setCategoryIds((current) =>
            current.filter((id) => next.some((category) => category.id === id))
          )
        }}
      />
    </div>
  )
}

function FilterChip({
  active,
  onClick,
  icon,
  children,
}: {
  active: boolean
  onClick: () => void
  icon?: React.ComponentProps<typeof HugeiconsIcon>["icon"]
  children: React.ReactNode
}) {
  return (
    <button
      type="button"
      aria-pressed={active}
      onClick={onClick}
      className={cn(
        "inline-flex h-8 shrink-0 cursor-pointer items-center gap-1.5 rounded-full border px-3 text-sm whitespace-nowrap transition-colors",
        active
          ? "border-primary bg-primary text-primary-foreground"
          : "border-border bg-input/30 text-foreground hover:bg-muted"
      )}
    >
      {icon && <HugeiconsIcon icon={icon} size={14} />}
      {children}
    </button>
  )
}

function RecipeCard({
  recipe,
  onToggleFavorite,
}: {
  recipe: Recipe
  onToggleFavorite: () => void
}) {
  const { t } = useI18n()
  const duration = durationLabel(recipe.estimated_minutes)

  return (
    <li className="relative flex flex-col rounded-2xl bg-card ring-1 ring-foreground/10 transition-shadow hover:ring-foreground/20">
      <Link
        to={`/recipes/${recipe.id}`}
        className="flex flex-1 flex-col gap-2 p-4 pr-12"
      >
        <div className="flex min-w-0 items-start gap-2">
          <h2 className="min-w-0 flex-1 font-heading text-base font-semibold break-words">
            {recipe.name}
          </h2>
        </div>

        {recipe.description && (
          <p className="line-clamp-2 text-sm text-muted-foreground">
            {recipe.description}
          </p>
        )}

        <div className="mt-auto flex flex-wrap items-center gap-x-3 gap-y-1 pt-1 text-xs text-muted-foreground">
          <span className="inline-flex items-center gap-1">
            <HugeiconsIcon icon={Time04Icon} size={14} />
            {t(duration.key, duration.vars)}
          </span>
          <span>
            {recipe.ingredient_count === 1
              ? t("recipe.ingredientsCountOne")
              : t("recipe.ingredientsCount", {
                  count: recipe.ingredient_count,
                })}
          </span>
          <span>
            {recipe.step_count === 1
              ? t("recipe.stepsCountOne")
              : t("recipe.stepsCount", { count: recipe.step_count })}
          </span>
        </div>

        {(recipe.categories.length > 0 || recipe.archived) && (
          <div className="flex flex-wrap gap-1.5 pt-1">
            {recipe.archived && (
              <Badge variant="secondary" className="text-[0.65rem]">
                {t("recipe.archivedBadge")}
              </Badge>
            )}
            {recipe.categories.map((category) => (
              <Badge
                key={category.id}
                variant="outline"
                className="text-[0.65rem]"
              >
                {category.name}
              </Badge>
            ))}
          </div>
        )}
      </Link>

      {/* Outside the Link: a favorite toggle nested in an anchor would
          navigate as well as toggle. */}
      <Button
        type="button"
        variant="ghost"
        size="icon-sm"
        onClick={onToggleFavorite}
        title={
          recipe.favorite ? t("recipe.removeFavorite") : t("recipe.addFavorite")
        }
        className={cn(
          "absolute top-3 right-3",
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
          {recipe.favorite ? t("recipe.removeFavorite") : t("recipe.addFavorite")}
        </span>
      </Button>
    </li>
  )
}
