import * as React from "react"
import { HugeiconsIcon } from "@hugeicons/react"
import { Add01Icon } from "@hugeicons/core-free-icons"

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
import { Skeleton } from "@/components/ui/skeleton"
import { useAuth } from "@/lib/auth-context"
import { useI18n } from "@/lib/i18n/context"
import { getListIcon } from "@/lib/list-icons"
import type { ShopList } from "@/lib/api"

/**
 * Picks the list something gets added to. Creating a new list is offered next
 * to the existing ones but handed back to the caller rather than done here, so
 * the creation dialog can open with whatever the caller knows (a suggested
 * name, say).
 */
function AddToListDialog({
  open,
  onOpenChange,
  onSelect,
  onCreateNew,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  onSelect: (list: ShopList) => void
  onCreateNew: () => void
}) {
  const { listLists } = useAuth()
  const { t } = useI18n()

  const [lists, setLists] = React.useState<ShopList[] | null>(null)
  const [failed, setFailed] = React.useState(false)

  useResetOnOpen(open, () => {
    // Reopening re-fetches: another session may have added a list since.
    setLists(null)
    setFailed(false)
  })

  React.useEffect(() => {
    if (!open) {
      return
    }

    let cancelled = false

    listLists()
      .then((loaded) => {
        if (!cancelled) {
          setLists(loaded)
        }
      })
      .catch(() => {
        if (!cancelled) {
          setFailed(true)
        }
      })

    return () => {
      cancelled = true
    }
  }, [listLists, open])

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{t("recipe.addToListTitle")}</DialogTitle>
          <DialogDescription>
            {t("recipe.addToListDescription")}
          </DialogDescription>
        </DialogHeader>

        <div className="flex flex-col gap-3">
          {failed && (
            <Alert variant="destructive">
              <AlertDescription>{t("recipe.addToListError")}</AlertDescription>
            </Alert>
          )}

          <Button
            type="button"
            variant="outline"
            className="w-full justify-start"
            onClick={onCreateNew}
          >
            <HugeiconsIcon icon={Add01Icon} data-icon="inline-start" />
            {t("recipe.addToListNew")}
          </Button>

          {lists === null && !failed && (
            <div className="flex flex-col gap-2">
              {Array.from({ length: 3 }).map((_, index) => (
                <Skeleton key={index} className="h-9 w-full rounded-xl" />
              ))}
            </div>
          )}

          {lists?.length === 0 && (
            <p className="py-2 text-center text-sm text-muted-foreground">
              {t("recipe.addToListEmpty")}
            </p>
          )}

          {lists && lists.length > 0 && (
            <ul className="flex max-h-72 flex-col gap-2 overflow-y-auto">
              {lists.map((list) => (
                <li key={list.id}>
                  <Button
                    type="button"
                    variant="outline"
                    className="w-full justify-start"
                    onClick={() => onSelect(list)}
                  >
                    <HugeiconsIcon
                      icon={getListIcon(list.icon)}
                      data-icon="inline-start"
                    />
                    <span className="truncate">{list.name}</span>
                  </Button>
                </li>
              ))}
            </ul>
          )}
        </div>
      </DialogContent>
    </Dialog>
  )
}

export { AddToListDialog }
