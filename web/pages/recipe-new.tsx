import * as React from "react"
import { Link, useNavigate } from "react-router"
import { toast } from "sonner"
import { HugeiconsIcon } from "@hugeicons/react"
import { ArrowLeft01Icon } from "@hugeicons/core-free-icons"

import { Button } from "@/components/ui/button"
import { RecipeForm } from "@/components/recipe-form"
import { useAuth } from "@/lib/auth-context"
import { useI18n } from "@/lib/i18n/context"
import { ApiError, type RecipeCategory, type RecipePayload } from "@/lib/api"

export default function NewRecipePage() {
  const navigate = useNavigate()
  const { createRecipe, listRecipeCategories } = useAuth()
  const { t } = useI18n()

  const [categories, setCategories] = React.useState<RecipeCategory[]>([])
  const [submitting, setSubmitting] = React.useState(false)
  const [error, setError] = React.useState<string | null>(null)

  React.useEffect(() => {
    listRecipeCategories()
      .then(setCategories)
      .catch(() => setCategories([]))
  }, [listRecipeCategories])

  async function submit(payload: RecipePayload) {
    setSubmitting(true)
    setError(null)
    try {
      const created = await createRecipe(payload)
      toast.success(t("recipe.createSuccess"))
      navigate(`/recipes/${created.id}`, { replace: true })
    } catch (err) {
      setError((err as ApiError)?.message ?? t("recipe.createError"))
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
          render={<Link to="/recipes" />}
          className="-ml-2 self-start text-muted-foreground"
        >
          <HugeiconsIcon icon={ArrowLeft01Icon} data-icon="inline-start" />
          {t("recipe.back")}
        </Button>
        <div>
          <h1 className="font-heading text-2xl font-semibold">
            {t("recipe.createTitle")}
          </h1>
          <p className="text-sm text-muted-foreground">
            {t("recipe.createDescription")}
          </p>
        </div>
      </div>

      <RecipeForm
        categories={categories}
        onCategoryCreated={(category) =>
          setCategories((current) =>
            [...current, category].sort((a, b) => a.name.localeCompare(b.name))
          )
        }
        onSubmit={submit}
        onCancel={() => navigate("/recipes")}
        submitting={submitting}
        error={error}
      />
    </div>
  )
}
