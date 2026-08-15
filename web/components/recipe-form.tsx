import * as React from "react"
import { HugeiconsIcon } from "@hugeicons/react"
import {
  Add01Icon,
  ArrowDown01Icon,
  ArrowUp01Icon,
  Delete02Icon,
  FavouriteIcon,
} from "@hugeicons/core-free-icons"

import { cn } from "@/lib/utils"
import { Alert, AlertDescription } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import {
  Field,
  FieldDescription,
  FieldGroup,
  FieldLabel,
} from "@/components/ui/field"
import { IngredientSuggestInput } from "@/components/suggest-input"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { MarkdownEditor } from "@/components/markdown-editor"
import { RecipeCategoryPicker } from "@/components/recipe-category-picker"
import { Spinner } from "@/components/ui/spinner"
import { useI18n } from "@/lib/i18n/context"
import { joinMinutes, markdownPreview, moveItem, splitMinutes } from "@/lib/recipes"
import type { RecipeCategory, RecipeDetail, RecipePayload } from "@/lib/api"

/**
 * Rows carry a client-side key instead of leaning on the array index. Reordering
 * and deleting steps would otherwise make React reuse the wrong editor, and a
 * tiptap instance that gets handed a different step keeps the old document.
 */
type IngredientRow = { key: string; name: string; quantity: string }
type StepRow = { key: string; title: string; description: string }

let rowCounter = 0

function nextKey(prefix: string) {
  rowCounter += 1
  return `${prefix}-${rowCounter}`
}

function emptyIngredient(): IngredientRow {
  return { key: nextKey("ing"), name: "", quantity: "" }
}

function emptyStep(): StepRow {
  return { key: nextKey("step"), title: "", description: "" }
}

function RecipeForm({
  recipe,
  categories,
  onCategoryCreated,
  onSubmit,
  onCancel,
  submitting,
  error,
}: {
  /** Absent for a new recipe. */
  recipe?: RecipeDetail
  categories: RecipeCategory[]
  onCategoryCreated: (category: RecipeCategory) => void
  onSubmit: (payload: RecipePayload) => void
  onCancel: () => void
  submitting: boolean
  error: string | null
}) {
  const { t } = useI18n()

  const initialTime = splitMinutes(recipe?.estimated_minutes ?? 0)

  const [name, setName] = React.useState(recipe?.name ?? "")
  const [description, setDescription] = React.useState(recipe?.description ?? "")
  const [hours, setHours] = React.useState(
    initialTime.hours > 0 ? String(initialTime.hours) : ""
  )
  const [minutes, setMinutes] = React.useState(
    initialTime.minutes > 0 ? String(initialTime.minutes) : ""
  )
  const [favorite, setFavorite] = React.useState(recipe?.favorite ?? false)
  const [categoryIds, setCategoryIds] = React.useState<number[]>(
    recipe?.categories.map((category) => category.id) ?? []
  )

  const [ingredients, setIngredients] = React.useState<IngredientRow[]>(() =>
    recipe && recipe.ingredients.length > 0
      ? recipe.ingredients.map((ingredient) => ({
          key: nextKey("ing"),
          name: ingredient.name,
          quantity: ingredient.quantity,
        }))
      : [emptyIngredient()]
  )

  const [steps, setSteps] = React.useState<StepRow[]>(() =>
    recipe && recipe.steps.length > 0
      ? recipe.steps.map((step) => ({
          key: nextKey("step"),
          title: step.title,
          description: step.description,
        }))
      : [emptyStep()]
  )

  // An existing recipe opens collapsed so the whole flow is visible at a
  // glance; a new one opens on its single empty step, ready to type into.
  const [openSteps, setOpenSteps] = React.useState<string[]>(() =>
    recipe && recipe.steps.length > 0 ? [] : [steps[0].key]
  )

  const [nameTouched, setNameTouched] = React.useState(false)

  const nameInvalid = name.trim().length < 2

  const ingredientRefs = React.useRef(new Map<string, HTMLInputElement>())
  const pendingFocus = React.useRef<string | null>(null)

  // Runs after every commit, which is the earliest point the new row's input
  // actually exists. Focusing straight from the handler (or from a rAF) races
  // the render and lands the next keystrokes back in the row just left.
  React.useEffect(() => {
    const key = pendingFocus.current
    if (!key) {
      return
    }
    pendingFocus.current = null
    ingredientRefs.current.get(key)?.focus()
  })

  function addIngredient(afterKey?: string) {
    const row = emptyIngredient()
    setIngredients((current) => {
      if (!afterKey) {
        return [...current, row]
      }
      const index = current.findIndex((item) => item.key === afterKey)
      if (index === -1) {
        return [...current, row]
      }
      return [...current.slice(0, index + 1), row, ...current.slice(index + 1)]
    })
    // Focus lands on the new row so a list can be typed without touching the
    // mouse: type, Enter, type, Enter.
    pendingFocus.current = row.key
  }

  /** Typing then Enter walks down the list without touching the mouse. */
  function addIngredientOnEnter(afterKey: string) {
    return (event: React.KeyboardEvent) => {
      if (event.key === "Enter") {
        event.preventDefault()
        addIngredient(afterKey)
      }
    }
  }

  function patchIngredient(key: string, patch: Partial<IngredientRow>) {
    setIngredients((current) =>
      current.map((row) => (row.key === key ? { ...row, ...patch } : row))
    )
  }

  function addStep() {
    const row = emptyStep()
    setSteps((current) => [...current, row])
    setOpenSteps((current) => [...current, row.key])
  }

  function patchStep(key: string, patch: Partial<StepRow>) {
    setSteps((current) =>
      current.map((row) => (row.key === key ? { ...row, ...patch } : row))
    )
  }

  function toggleStep(key: string) {
    setOpenSteps((current) =>
      current.includes(key)
        ? current.filter((item) => item !== key)
        : [...current, key]
    )
  }

  function submit(event: React.FormEvent) {
    event.preventDefault()
    setNameTouched(true)

    if (nameInvalid || submitting) {
      return
    }

    onSubmit({
      name: name.trim(),
      description: description.trim(),
      estimated_minutes: joinMinutes(Number(hours), Number(minutes)),
      favorite,
      category_ids: categoryIds,
      // Blank rows are dropped here as well as server-side, so the count shown
      // in the confirmation matches what was saved.
      ingredients: ingredients
        .filter((row) => row.name.trim().length > 0)
        .map((row) => ({ name: row.name.trim(), quantity: row.quantity.trim() })),
      steps: steps
        .filter((row) => row.title.trim().length > 0)
        .map((row) => ({ title: row.title.trim(), description: row.description })),
    })
  }

  return (
    <form onSubmit={submit} noValidate className="flex flex-col gap-8 pb-24">
      {error && (
        <Alert variant="destructive">
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}

      <FieldGroup className="gap-6">
        <Field>
          <FieldLabel htmlFor="recipe-name">{t("recipe.nameLabel")}</FieldLabel>
          <Input
            id="recipe-name"
            autoFocus={!recipe}
            value={name}
            maxLength={150}
            placeholder={t("recipe.namePlaceholder")}
            disabled={submitting}
            aria-invalid={nameTouched && nameInvalid}
            onBlur={() => setNameTouched(true)}
            onChange={(event) => setName(event.target.value)}
          />
          {nameTouched && nameInvalid && (
            <FieldDescription className="text-destructive">
              {t("recipe.nameRequired")}
            </FieldDescription>
          )}
        </Field>

        <Field>
          <FieldLabel htmlFor="recipe-description">
            {t("recipe.descriptionLabel")}
          </FieldLabel>
          <Input
            id="recipe-description"
            value={description}
            maxLength={250}
            placeholder={t("recipe.descriptionPlaceholder")}
            disabled={submitting}
            onChange={(event) => setDescription(event.target.value)}
          />
        </Field>

        <Field>
          <FieldLabel>{t("recipe.timeLabel")}</FieldLabel>
          <div className="flex items-center gap-2">
            <div className="flex flex-1 items-center gap-2">
              <Input
                id="recipe-hours"
                type="number"
                inputMode="numeric"
                min={0}
                max={99}
                value={hours}
                placeholder="0"
                disabled={submitting}
                onChange={(event) => setHours(event.target.value)}
                className="w-full"
              />
              <Label
                htmlFor="recipe-hours"
                className="shrink-0 text-sm text-muted-foreground"
              >
                {t("recipe.timeHours")}
              </Label>
            </div>
            <div className="flex flex-1 items-center gap-2">
              <Input
                id="recipe-minutes"
                type="number"
                inputMode="numeric"
                min={0}
                max={59}
                value={minutes}
                placeholder="0"
                disabled={submitting}
                onChange={(event) => setMinutes(event.target.value)}
                className="w-full"
              />
              <Label
                htmlFor="recipe-minutes"
                className="shrink-0 text-sm text-muted-foreground"
              >
                {t("recipe.timeMinutes")}
              </Label>
            </div>
          </div>
          <FieldDescription>{t("recipe.timeHint")}</FieldDescription>
        </Field>

        <Field>
          <FieldLabel>{t("recipe.categoriesLabel")}</FieldLabel>
          <RecipeCategoryPicker
            categories={categories}
            selectedIds={categoryIds}
            onChange={setCategoryIds}
            onCategoryCreated={onCategoryCreated}
            disabled={submitting}
          />
          <FieldDescription>{t("recipe.categoriesHint")}</FieldDescription>
        </Field>

        <Field orientation="horizontal">
          <Checkbox
            id="recipe-favorite"
            checked={favorite}
            disabled={submitting}
            onCheckedChange={(checked) => setFavorite(checked === true)}
          />
          <Label
            htmlFor="recipe-favorite"
            className="flex items-center gap-1.5 font-normal"
          >
            <HugeiconsIcon
              icon={FavouriteIcon}
              size={16}
              className="text-muted-foreground"
            />
            {t("recipe.markFavoriteLabel")}
          </Label>
        </Field>
      </FieldGroup>

      <section className="flex flex-col gap-3">
        <div className="flex items-baseline justify-between gap-3">
          <h2 className="font-heading text-lg font-semibold">
            {t("recipe.ingredientsTitle")}
          </h2>
          <span className="text-xs text-muted-foreground tabular-nums">
            {ingredients.filter((row) => row.name.trim()).length}
          </span>
        </div>
        <p className="-mt-1 text-xs text-muted-foreground">
          {t("recipe.ingredientsHint")}
        </p>

        <ul className="flex flex-col gap-2">
          {ingredients.map((row) => (
            <li key={row.key} className="flex items-center gap-2">
              <IngredientSuggestInput
                ref={(element) => {
                  if (element) {
                    ingredientRefs.current.set(row.key, element)
                  } else {
                    ingredientRefs.current.delete(row.key)
                  }
                }}
                value={row.name}
                maxLength={100}
                placeholder={t("recipe.ingredientNamePlaceholder")}
                disabled={submitting}
                onChange={(name) => patchIngredient(row.key, { name })}
                onKeyDown={addIngredientOnEnter(row.key)}
                containerClassName="min-w-0 flex-1"
              />
              <Input
                value={row.quantity}
                maxLength={50}
                placeholder={t("recipe.ingredientQuantityPlaceholder")}
                disabled={submitting}
                onChange={(event) =>
                  patchIngredient(row.key, { quantity: event.target.value })
                }
                onKeyDown={addIngredientOnEnter(row.key)}
                className="w-24 shrink-0 sm:w-36"
              />
              <Button
                type="button"
                variant="ghost"
                size="icon-sm"
                disabled={submitting || ingredients.length === 1}
                onClick={() =>
                  setIngredients((current) =>
                    current.filter((item) => item.key !== row.key)
                  )
                }
                className="shrink-0 text-muted-foreground"
              >
                <HugeiconsIcon icon={Delete02Icon} />
                <span className="sr-only">{t("recipe.removeIngredient")}</span>
              </Button>
            </li>
          ))}
        </ul>

        <Button
          type="button"
          variant="outline"
          size="sm"
          disabled={submitting}
          onClick={() => addIngredient()}
          className="self-start"
        >
          <HugeiconsIcon icon={Add01Icon} data-icon="inline-start" />
          {t("recipe.addIngredient")}
        </Button>
      </section>

      <section className="flex flex-col gap-3">
        <div className="flex items-baseline justify-between gap-3">
          <h2 className="font-heading text-lg font-semibold">
            {t("recipe.stepsTitle")}
          </h2>
          <span className="text-xs text-muted-foreground tabular-nums">
            {steps.filter((row) => row.title.trim()).length}
          </span>
        </div>
        <p className="-mt-1 text-xs text-muted-foreground">
          {t("recipe.stepsHint")}
        </p>

        <ol className="flex flex-col gap-3">
          {steps.map((row, index) => {
            const open = openSteps.includes(row.key)
            const preview = markdownPreview(row.description)

            return (
              <li
                key={row.key}
                className="rounded-2xl bg-card ring-1 ring-foreground/10"
              >
                {/* Wraps on a phone: the title gets the whole first line and
                    the reorder controls drop below it, instead of all four
                    fighting over 390px. */}
                <div className="flex flex-wrap items-center gap-2 p-3">
                  <span className="flex size-7 shrink-0 items-center justify-center rounded-full bg-muted text-xs font-semibold tabular-nums">
                    {index + 1}
                  </span>
                  <Input
                    value={row.title}
                    maxLength={250}
                    placeholder={t("recipe.stepTitlePlaceholder")}
                    disabled={submitting}
                    onChange={(event) =>
                      patchStep(row.key, { title: event.target.value })
                    }
                    onFocus={() => {
                      if (!open) {
                        toggleStep(row.key)
                      }
                    }}
                    // Scoped to `max-sm` rather than overridden at `sm`: a
                    // plain `basis-*` beats the `sm:` variant that would undo
                    // it, so the rule has to not exist above the breakpoint.
                    className="min-w-0 flex-1 max-sm:basis-[calc(100%-2.75rem)]"
                  />
                  <div className="ml-auto flex shrink-0 items-center">
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon-sm"
                      disabled={submitting || index === 0}
                      onClick={() =>
                        setSteps((current) => moveItem(current, index, -1))
                      }
                      className="text-muted-foreground"
                    >
                      <HugeiconsIcon icon={ArrowUp01Icon} />
                      <span className="sr-only">{t("recipe.moveUp")}</span>
                    </Button>
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon-sm"
                      disabled={submitting || index === steps.length - 1}
                      onClick={() =>
                        setSteps((current) => moveItem(current, index, 1))
                      }
                      className="text-muted-foreground"
                    >
                      <HugeiconsIcon icon={ArrowDown01Icon} />
                      <span className="sr-only">{t("recipe.moveDown")}</span>
                    </Button>
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon-sm"
                      disabled={submitting || steps.length === 1}
                      onClick={() => {
                        setSteps((current) =>
                          current.filter((item) => item.key !== row.key)
                        )
                        setOpenSteps((current) =>
                          current.filter((item) => item !== row.key)
                        )
                      }}
                      className="text-muted-foreground"
                    >
                      <HugeiconsIcon icon={Delete02Icon} />
                      <span className="sr-only">{t("recipe.removeStep")}</span>
                    </Button>
                  </div>
                </div>

                {open ? (
                  <div className="px-3 pb-3">
                    <MarkdownEditor
                      value={row.description}
                      disabled={submitting}
                      placeholder={t("recipe.stepDescriptionPlaceholder")}
                      onChange={(markdown) =>
                        patchStep(row.key, { description: markdown })
                      }
                    />
                    <Button
                      type="button"
                      variant="ghost"
                      size="xs"
                      onClick={() => toggleStep(row.key)}
                      className="mt-1.5 text-muted-foreground"
                    >
                      {t("recipe.collapseStep")}
                    </Button>
                  </div>
                ) : (
                  // Collapsed steps show flattened text rather than a second
                  // editor: a recipe with fifteen steps would otherwise mount
                  // fifteen ProseMirror instances to render previews.
                  <button
                    type="button"
                    onClick={() => toggleStep(row.key)}
                    className={cn(
                      "flex w-full cursor-pointer items-center gap-2 px-3 pb-3 text-left text-xs",
                      preview ? "text-muted-foreground" : "text-muted-foreground/60"
                    )}
                  >
                    <span className="min-w-0 flex-1 truncate">
                      {preview || t("recipe.stepNoDescription")}
                    </span>
                    <span className="shrink-0 underline underline-offset-4">
                      {t("recipe.expandStep")}
                    </span>
                  </button>
                )}
              </li>
            )
          })}
        </ol>

        <Button
          type="button"
          variant="outline"
          size="sm"
          disabled={submitting}
          onClick={addStep}
          className="self-start"
        >
          <HugeiconsIcon icon={Add01Icon} data-icon="inline-start" />
          {t("recipe.addStep")}
        </Button>
      </section>

      {/* Sticky so the save button is always one tap away on a phone, however
          long the recipe gets. */}
      <div className="fixed inset-x-0 bottom-0 z-20 border-t border-border/70 bg-background/95 px-4 py-3 backdrop-blur">
        <div className="mx-auto flex w-full max-w-3xl justify-end gap-2">
          <Button
            type="button"
            variant="outline"
            disabled={submitting}
            onClick={onCancel}
          >
            {t("recipe.cancel")}
          </Button>
          <Button type="submit" disabled={submitting}>
            {submitting && <Spinner data-icon="inline-start" />}
            {submitting
              ? recipe
                ? t("recipe.saving")
                : t("recipe.creating")
              : recipe
                ? t("recipe.save")
                : t("recipe.create")}
          </Button>
        </div>
      </div>
    </form>
  )
}

export { RecipeForm }
