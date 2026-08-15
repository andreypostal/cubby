import * as React from "react"
import { Link, useNavigate, useParams } from "react-router"
import { toast } from "sonner"
import { HugeiconsIcon } from "@hugeicons/react"
import { ArrowLeft01Icon } from "@hugeicons/core-free-icons"

import { Alert, AlertDescription } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { RecipeForm } from "@/components/recipe-form"
import { Skeleton } from "@/components/ui/skeleton"
import { useAuth } from "@/lib/auth-context"
import { useI18n } from "@/lib/i18n/context"
import {
  ApiError,
  type RecipeCategory,
  type RecipeDetail,
  type RecipePayload,
} from "@/lib/api"

export default function EditRecipePage() {
  const params = useParams<{ recipeID: string }>()
  const recipeId = Number(params.recipeID)

  const navigate = useNavigate()
  const { getRecipe, updateRecipe, listRecipeCategories } = useAuth()
  const { t } = useI18n()

  const [recipe, setRecipe] = React.useState<RecipeDetail | null>(null)
  const [categories, setCategories] = React.useState<RecipeCategory[]>([])
  const [fetchFailed, setFetchFailed] = React.useState(false)
  const [submitting, setSubmitting] = React.useState(false)
  const [error, setError] = React.useState<string | null>(null)

  // A non-numeric path segment is a bad URL, not a failed request, so it is
  // derived rather than stored.
  const loadError = !Number.isFinite(recipeId) || fetchFailed

  React.useEffect(() => {
    if (!Number.isFinite(recipeId)) {
      return
    }

    let cancelled = false

    getRecipe(recipeId)
      .then((loaded) => {
        if (!cancelled) {
          setRecipe(loaded)
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

  React.useEffect(() => {
    listRecipeCategories()
      .then(setCategories)
      .catch(() => setCategories([]))
  }, [listRecipeCategories])

  async function submit(payload: RecipePayload) {
    setSubmitting(true)
    setError(null)
    try {
      await updateRecipe(recipeId, payload)
      toast.success(t("recipe.updateSuccess"))
      navigate(`/recipes/${recipeId}`, { replace: true })
    } catch (err) {
      setError((err as ApiError)?.message ?? t("recipe.updateError"))
      setSubmitting(false)
    }
  }

  return (
    <div className="mx-auto flex w-full max-w-3xl flex-col gap-5">
      <div className="flex flex-col gap-2">
        <Button
          variant="ghost"
          size="sm"
          nativeButton={false}
          render={<Link to={`/recipes/${recipeId}`} />}
          className="-ml-2 self-start text-muted-foreground"
        >
          <HugeiconsIcon icon={ArrowLeft01Icon} data-icon="inline-start" />
          {t("recipe.back")}
        </Button>
        <div>
          <h1 className="font-heading text-2xl font-semibold">
            {t("recipe.editTitle")}
          </h1>
          <p className="text-sm text-muted-foreground">
            {t("recipe.editDescription")}
          </p>
        </div>
      </div>

      {loadError && (
        <Alert variant="destructive">
          <AlertDescription>{t("recipe.detailLoadError")}</AlertDescription>
        </Alert>
      )}

      {!recipe && !loadError && (
        <div className="flex flex-col gap-4">
          <Skeleton className="h-10 w-full rounded-xl" />
          <Skeleton className="h-10 w-full rounded-xl" />
          <Skeleton className="h-32 w-full rounded-2xl" />
        </div>
      )}

      {recipe && (
        // Keyed on the loaded recipe so the form's initial state is only read
        // once the data is actually there.
        <RecipeForm
          key={recipe.id}
          recipe={recipe}
          categories={categories}
          onCategoryCreated={(category) =>
            setCategories((current) =>
              [...current, category].sort((a, b) => a.name.localeCompare(b.name))
            )
          }
          onSubmit={submit}
          onCancel={() => navigate(`/recipes/${recipeId}`)}
          submitting={submitting}
          error={error}
        />
      )}
    </div>
  )
}
