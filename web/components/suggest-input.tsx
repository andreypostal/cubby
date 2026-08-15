import * as React from "react"

import { cn } from "@/lib/utils"
import { Input } from "@/components/ui/input"
import { useAuth } from "@/lib/auth-context"

const SUGGESTION_DEBOUNCE_MS = 150

type SuggestInputProps = {
  /** Looks up the terms for what has been typed so far. */
  suggest: (term: string) => Promise<string[]>
  value: string
  onChange: (value: string) => void
  disabled?: boolean
  placeholder?: string
  className?: string
  containerClassName?: string
  id?: string
  maxLength?: number
  ref?: React.Ref<HTMLInputElement>
  /** Runs only for keys the suggestion list did not consume. */
  onKeyDown?: (event: React.KeyboardEvent<HTMLInputElement>) => void
  deferUntilFocused?: boolean
}

/**
 * A text input backed by name suggestions from the workspace, with the closest
 * match shown inline as a greyed out completion that Tab accepts.
 */
function SuggestInput({
  suggest,
  value,
  onChange,
  disabled,
  placeholder,
  className,
  containerClassName,
  id,
  maxLength,
  ref,
  onKeyDown: onKeyDownProp,
  deferUntilFocused = false,
}: SuggestInputProps) {
  const [suggestions, setSuggestions] = React.useState<string[]>([])
  const [open, setOpen] = React.useState(false)
  const [activeIndex, setActiveIndex] = React.useState(-1)
  const [focused, setFocused] = React.useState(!deferUntilFocused)

  const containerRef = React.useRef<HTMLDivElement>(null)
  const inputRef = React.useRef<HTMLInputElement>(null)
  const requestId = React.useRef(0)

  // A caller may own a ref of its own to focus freshly added rows, so the node
  // has to reach both it and the local ref used to restore focus after a pick.
  const setInputRef = React.useCallback(
    (element: HTMLInputElement | null) => {
      inputRef.current = element
      if (typeof ref === "function") {
        ref(element)
      } else if (ref) {
        ref.current = element
      }
    },
    [ref]
  )

  React.useEffect(() => {
    const term = value.trim()
    if (!term || !focused) {
      return
    }

    const currentRequest = ++requestId.current
    const timeout = setTimeout(() => {
      suggest(term)
        .then((terms) => {
          if (requestId.current !== currentRequest) {
            return
          }
          setSuggestions(terms)
          setOpen(terms.length > 0)
          setActiveIndex(-1)
        })
        .catch(() => {
          if (requestId.current !== currentRequest) {
            return
          }
          setSuggestions([])
          setOpen(false)
        })
    }, SUGGESTION_DEBOUNCE_MS)

    return () => clearTimeout(timeout)
  }, [value, focused, suggest])

  React.useEffect(() => {
    function onPointerDown(event: PointerEvent) {
      if (!containerRef.current?.contains(event.target as Node)) {
        setOpen(false)
      }
    }
    document.addEventListener("pointerdown", onPointerDown)
    return () => document.removeEventListener("pointerdown", onPointerDown)
  }, [])

  const visibleSuggestions = value.trim() ? suggestions : []

  const closestMatch =
    visibleSuggestions[0] &&
    visibleSuggestions[0].toLowerCase().startsWith(value.toLowerCase()) &&
    visibleSuggestions[0].toLowerCase() !== value.toLowerCase()
      ? visibleSuggestions[0]
      : undefined

  const ghostCompletion = closestMatch
    ? value + closestMatch.slice(value.length)
    : undefined

  function selectSuggestion(term: string) {
    onChange(term)
    setOpen(false)
    setActiveIndex(-1)
    inputRef.current?.focus()
  }

  function onKeyDown(event: React.KeyboardEvent<HTMLInputElement>) {
    if (event.key === "Tab" && closestMatch) {
      event.preventDefault()
      onChange(closestMatch)
      setOpen(false)
      return
    }

    if (open && visibleSuggestions.length > 0) {
      if (event.key === "ArrowDown") {
        event.preventDefault()
        setActiveIndex((current) => (current + 1) % visibleSuggestions.length)
        return
      }
      if (event.key === "ArrowUp") {
        event.preventDefault()
        setActiveIndex((current) =>
          current <= 0 ? visibleSuggestions.length - 1 : current - 1
        )
        return
      }
      if (event.key === "Enter" && activeIndex >= 0) {
        event.preventDefault()
        selectSuggestion(visibleSuggestions[activeIndex])
        return
      }
      if (event.key === "Escape") {
        setOpen(false)
        return
      }
    }

    // Enter with nothing highlighted still means "add the next row".
    onKeyDownProp?.(event)
  }

  return (
    <div
      ref={containerRef}
      className={cn("relative w-full", containerClassName)}
    >
      {ghostCompletion && (
        <Input
          aria-hidden
          tabIndex={-1}
          readOnly
          value={ghostCompletion}
          className={cn(
            "pointer-events-none absolute inset-0 select-none",
            className,
            "border-transparent bg-transparent text-muted-foreground"
          )}
        />
      )}

      <Input
        ref={setInputRef}
        id={id}
        value={value}
        maxLength={maxLength}
        onChange={(event) => onChange(event.target.value)}
        onKeyDown={onKeyDown}
        onFocus={() => {
          setFocused(true)
          if (suggestions.length > 0) {
            setOpen(true)
          }
        }}
        onBlur={
          deferUntilFocused
            ? () => {
                setFocused(false)
                setOpen(false)
              }
            : undefined
        }
        disabled={disabled}
        placeholder={placeholder}
        autoComplete="off"
        className={cn("relative bg-transparent", className)}
      />

      {open && visibleSuggestions.length > 0 && (
        <div className="absolute top-full right-0 left-0 z-50 mt-1.5 flex flex-col gap-0.5 rounded-2xl bg-popover/95 p-1 text-sm text-popover-foreground shadow-2xl ring-1 ring-foreground/5 backdrop-blur-2xl backdrop-saturate-150">
          {visibleSuggestions.map((term, index) => (
            <button
              key={term}
              type="button"
              onMouseDown={(event) => event.preventDefault()}
              onClick={() => selectSuggestion(term)}
              onMouseEnter={() => setActiveIndex(index)}
              className={cn(
                "flex cursor-default items-center truncate rounded-xl px-3 py-2 text-left outline-hidden",
                index === activeIndex && "bg-foreground/10"
              )}
            >
              {term}
            </button>
          ))}
        </div>
      )}
    </div>
  )
}

type BoundProps = Omit<SuggestInputProps, "suggest" | "deferUntilFocused">

/** Suggests product names already seen in this one shopping list. */
function ProductSuggestInput({
  listId,
  ...props
}: BoundProps & { listId: number }) {
  const { suggestItemNames } = useAuth()

  const suggest = React.useCallback(
    (term: string) => suggestItemNames(listId, term),
    [suggestItemNames, listId]
  )

  return <SuggestInput {...props} suggest={suggest} />
}

/** Suggests ingredient names from anywhere in the workspace's recipes. */
function IngredientSuggestInput(props: BoundProps) {
  const { suggestIngredientNames } = useAuth()

  return (
    <SuggestInput
      {...props}
      suggest={suggestIngredientNames}
      deferUntilFocused
    />
  )
}

export { IngredientSuggestInput, ProductSuggestInput }
