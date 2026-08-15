import * as React from "react"
import { HugeiconsIcon } from "@hugeicons/react"
import { Add01Icon, Tick02Icon } from "@hugeicons/core-free-icons"

import { cn } from "@/lib/utils"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Spinner } from "@/components/ui/spinner"
import { useAuth } from "@/lib/auth-context"
import { useI18n } from "@/lib/i18n/context"
import { ApiError, type RecipeCategory } from "@/lib/api"

/**
 * Chips to assign categories, plus an inline field to create one. Creating from
 * here matters: picking categories is the moment the user notices the one they
 * need does not exist yet, and bouncing them to a separate screen would lose
 * the recipe they are in the middle of writing.
 */
function RecipeCategoryPicker({
  categories,
  selectedIds,
  onChange,
  onCategoryCreated,
  disabled,
}: {
  categories: RecipeCategory[]
  selectedIds: number[]
  onChange: (ids: number[]) => void
  onCategoryCreated: (category: RecipeCategory) => void
  disabled?: boolean
}) {
  const { createRecipeCategory } = useAuth()
  const { t } = useI18n()

  const [name, setName] = React.useState("")
  const [creating, setCreating] = React.useState(false)
  const [error, setError] = React.useState<string | null>(null)

  function toggle(id: number) {
    onChange(
      selectedIds.includes(id)
        ? selectedIds.filter((current) => current !== id)
        : [...selectedIds, id]
    )
  }

  async function create() {
    const trimmed = name.trim()
    if (trimmed.length < 2 || creating) {
      return
    }

    setCreating(true)
    setError(null)
    try {
      const category = await createRecipeCategory(trimmed)
      onCategoryCreated(category)
      // A category created from here is one the user wants on this recipe.
      onChange([...selectedIds, category.id])
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

  return (
    <div className="flex flex-col gap-3">
      {categories.length > 0 && (
        <div className="flex flex-wrap gap-2">
          {categories.map((category) => {
            const selected = selectedIds.includes(category.id)
            return (
              <button
                key={category.id}
                type="button"
                disabled={disabled}
                aria-pressed={selected}
                onClick={() => toggle(category.id)}
                className={cn(
                  "inline-flex h-8 cursor-pointer items-center gap-1.5 rounded-full border px-3 text-sm transition-colors",
                  selected
                    ? "border-primary bg-primary text-primary-foreground"
                    : "border-border bg-input/30 text-foreground hover:bg-muted",
                  disabled && "pointer-events-none opacity-50"
                )}
              >
                {selected && <HugeiconsIcon icon={Tick02Icon} size={14} />}
                {category.name}
              </button>
            )
          })}
        </div>
      )}

      <div className="flex items-center gap-2">
        <Input
          value={name}
          maxLength={100}
          placeholder={t("recipe.newCategoryPlaceholder")}
          disabled={disabled || creating}
          onChange={(event) => setName(event.target.value)}
          onKeyDown={(event) => {
            // The picker lives inside the recipe form, so a bare Enter here
            // would submit the recipe instead of creating the category.
            if (event.key === "Enter") {
              event.preventDefault()
              create()
            }
          }}
          className="h-9"
        />
        <Button
          type="button"
          variant="outline"
          size="sm"
          disabled={disabled || creating || name.trim().length < 2}
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

      {error && <p className="text-xs text-destructive">{error}</p>}
    </div>
  )
}

export { RecipeCategoryPicker }
