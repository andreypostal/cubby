import * as React from "react"
import { useNavigate, useParams } from "react-router"
import { AnimatePresence, motion } from "motion/react"
import { toast } from "sonner"
import { HugeiconsIcon } from "@hugeicons/react"
import {
  Add01Icon,
  CogIcon,
  Delete02Icon,
  Note01Icon,
  ShoppingBasket01Icon,
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
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuSeparator,
  ContextMenuTrigger,
} from "@/components/ui/context-menu"
import { ListSettingsDialog } from "@/components/list-settings-dialog"
import { ProductSuggestInput } from "@/components/suggest-input"
import { Separator } from "@/components/ui/separator"
import { ShopItemNoteDialog } from "@/components/shop-item-note-dialog"
import { Spinner } from "@/components/ui/spinner"
import { useAuth } from "@/lib/auth-context"
import { useI18n } from "@/lib/i18n/context"
import { getListIcon } from "@/lib/list-icons"
import type { ShopListDetail, ShopListItem } from "@/lib/api"

export default function ShopListPage() {
  const params = useParams<{ listID: string }>()
  const listId = Number(params.listID)

  return (
    <AnimatePresence mode="wait">
      <ShopListView key={listId} listId={listId} />
    </AnimatePresence>
  )
}

function ShopListView({ listId }: { listId: number }) {
  const {
    getList,
    createListItem,
    updateListItem,
    toggleListItem,
    deleteListItem,
    clearBoughtListItems,
  } = useAuth()
  const { t, locale } = useI18n()
  const navigate = useNavigate()

  const [list, setList] = React.useState<ShopListDetail | null>(null)
  const [loadError, setLoadError] = React.useState(false)
  const [newProduct, setNewProduct] = React.useState("")
  const [adding, setAdding] = React.useState(false)
  const [clearing, setClearing] = React.useState(false)
  const [settingsOpen, setSettingsOpen] = React.useState(false)

  React.useEffect(() => {
    getList(listId)
      .then(setList)
      .catch(() => setLoadError(true))
  }, [getList, listId])

  const dateFormatter = React.useMemo(
    () =>
      new Intl.DateTimeFormat(locale === "pt" ? "pt-BR" : "en-US", {
        day: "numeric",
        month: "short",
      }),
    [locale]
  )

  // Everything on this page is a change to the list's items, and none of it
  // has anything to do while the list itself is still loading.
  function patchItems(map: (items: ShopListItem[]) => ShopListItem[]) {
    setList((current) =>
      current ? { ...current, items: map(current.items) } : current
    )
  }

  function replaceItem(replacement: ShopListItem) {
    patchItems((items) =>
      items.map((item) => (item.id === replacement.id ? replacement : item))
    )
  }

  async function onAddProduct(event: React.FormEvent) {
    event.preventDefault()
    if (!newProduct.trim() || !list) {
      return
    }

    setAdding(true)
    try {
      const item = await createListItem(listId, newProduct.trim())
      patchItems((items) => [...items, item])
      setNewProduct("")
    } finally {
      setAdding(false)
    }
  }

  async function onSaveNote(item: ShopListItem, notes: string) {
    replaceItem(await updateListItem(listId, item.id, { notes }))
  }

  async function onToggleItem(item: ShopListItem) {
    replaceItem({
      ...item,
      bought: !item.bought,
      bought_at: !item.bought ? new Date().toISOString() : "",
    })

    try {
      replaceItem(await toggleListItem(listId, item.id))
    } catch (error) {
      replaceItem(item)
      throw error
    }
  }

  async function onDeleteItem(item: ShopListItem) {
    await deleteListItem(listId, item.id)
    patchItems((items) => items.filter((existing) => existing.id !== item.id))
  }

  async function onClearBought() {
    setClearing(true)
    try {
      const items = await clearBoughtListItems(listId)
      patchItems(() => items)
    } finally {
      setClearing(false)
    }
  }

  if (loadError) {
    return (
      <Alert variant="destructive">
        <AlertDescription>{t("list.loadError")}</AlertDescription>
      </Alert>
    )
  }

  if (!list) {
    return (
      <motion.div
        key="loading"
        initial={{ opacity: 0 }}
        animate={{ opacity: 1 }}
        exit={{ opacity: 0 }}
        transition={{ duration: 0.15 }}
        className="flex flex-1 items-center justify-center"
      >
        <Spinner className="size-6" />
      </motion.div>
    )
  }

  const toBuyItems = list.items.filter((item) => !item.bought)
  const boughtItems = list.items.filter((item) => item.bought)

  return (
    <motion.div
      key="content"
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      exit={{ opacity: 0 }}
      transition={{ duration: 0.2 }}
      className="mx-auto flex w-full max-w-2xl flex-col gap-6"
    >
      <div className="min-w-0">
        <p className="text-xs font-medium tracking-wide text-muted-foreground uppercase">
          {t("list.eyebrow")}
        </p>
        <div className="flex items-center gap-2">
          <HugeiconsIcon
            icon={getListIcon(list.icon)}
            size={18}
            className="shrink-0 text-primary"
          />
          <h1 className="truncate font-heading text-2xl font-semibold">
            {list.name}
          </h1>
          <Button
            type="button"
            variant="ghost"
            size="icon-lg"
            className="shrink-0 text-muted-foreground hover:text-foreground"
            onClick={() => setSettingsOpen(true)}
          >
            <HugeiconsIcon icon={CogIcon} />
            <span className="sr-only">{t("list.settingsLabel")}</span>
          </Button>
        </div>
        <p className="mt-1 text-xs/relaxed text-muted-foreground">
          {t("list.toBuyAndBought", {
            toBuy: toBuyItems.length,
            bought: boughtItems.length,
          })}
        </p>
      </div>

      <div className="flex flex-col gap-1.5">
        <form onSubmit={onAddProduct} className="flex gap-2">
          <ProductSuggestInput
            listId={listId}
            placeholder={t("list.addProductPlaceholder")}
            value={newProduct}
            onChange={setNewProduct}
            disabled={adding}
            className="h-9"
          />
          <Button type="submit" disabled={adding} className="h-9 pr-4">
            {adding ? <Spinner /> : <HugeiconsIcon icon={Add01Icon} />}
            {t("list.add")}
          </Button>
        </form>

        {/* Only worth saying once there is something to right click on. */}
        {list.items.length > 0 && (
          <p className="text-[0.7rem]/relaxed text-muted-foreground">
            {t("list.noteHint")}
          </p>
        )}
      </div>

      <motion.div layout className="flex flex-col gap-2">
        {toBuyItems.length === 0 && boughtItems.length === 0 && (
          <p className="py-6 text-center text-xs/relaxed text-muted-foreground">
            {t("list.empty")}
          </p>
        )}

        <AnimatePresence mode="popLayout">
          {toBuyItems.map((item) => (
            <ListItemRow
              key={item.id}
              item={item}
              onToggle={onToggleItem}
              onDelete={onDeleteItem}
              onSaveNote={onSaveNote}
            />
          ))}
        </AnimatePresence>
      </motion.div>

      {boughtItems.length > 0 && (
        <motion.div layout className="flex flex-col gap-6">
          <Separator />

          <div className="flex flex-col gap-2">
            <div className="flex items-center justify-between">
              <p className="text-xs font-medium tracking-wide text-muted-foreground uppercase">
                {t("list.bought", { count: boughtItems.length })}
              </p>
              <Button
                variant="outline"
                size="sm"
                disabled={clearing}
                onClick={onClearBought}
              >
                {clearing && <Spinner data-icon="inline-start" />}
                {t("list.clearBought")}
              </Button>
            </div>

            <AnimatePresence mode="popLayout">
              {boughtItems.map((item) => (
                <ListItemRow
                  key={item.id}
                  item={item}
                  onToggle={onToggleItem}
                  onDelete={onDeleteItem}
                  onSaveNote={onSaveNote}
                  dateLabel={
                    item.bought_at
                      ? t("list.boughtOn", {
                          date: dateFormatter.format(new Date(item.bought_at)),
                        })
                      : undefined
                  }
                />
              ))}
            </AnimatePresence>
          </div>
        </motion.div>
      )}

      <ListSettingsDialog
        open={settingsOpen}
        onOpenChange={setSettingsOpen}
        list={list}
        onSaved={(updated) =>
          setList((current) =>
            current
              ? { ...current, name: updated.name, icon: updated.icon }
              : current
          )
        }
        onDeleted={() => navigate("/")}
      />
    </motion.div>
  )
}

/**
 * A single product. The row itself is the context menu trigger — right click on
 * a pointer device, long press on touch — which is where noting, marking as
 * bought and deleting live.
 */
function ListItemRow({
  item,
  onToggle,
  onDelete,
  onSaveNote,
  dateLabel,
}: {
  item: ShopListItem
  onToggle: (item: ShopListItem) => Promise<void>
  onDelete: (item: ShopListItem) => Promise<void>
  onSaveNote: (item: ShopListItem, notes: string) => Promise<void>
  dateLabel?: string
}) {
  const { t } = useI18n()

  const [noteOpen, setNoteOpen] = React.useState(false)
  const [markBoughtOpen, setMarkBoughtOpen] = React.useState(false)
  const [deleteOpen, setDeleteOpen] = React.useState(false)
  const [busy, setBusy] = React.useState(false)

  async function onConfirmMarkBought() {
    setBusy(true)
    try {
      await onToggle(item)
      setMarkBoughtOpen(false)
    } catch {
      toast.error(t("list.markBoughtError"))
    } finally {
      setBusy(false)
    }
  }

  async function onConfirmDelete() {
    setBusy(true)
    try {
      await onDelete(item)
      setDeleteOpen(false)
    } catch {
      toast.error(t("list.deleteItemError"))
    } finally {
      setBusy(false)
    }
  }

  return (
    <motion.div
      layout
      layoutId={`item-${item.id}`}
      initial={{ opacity: 0, scale: 0.9 }}
      animate={{ opacity: 1, scale: 1 }}
      exit={{ opacity: 0, scale: 0.9 }}
      transition={{
        layout: { type: "spring", bounce: 0.2, duration: 0.5 },
        opacity: { duration: 0.15 },
        scale: { duration: 0.15 },
      }}
    >
      <ContextMenu>
        <ContextMenuTrigger className="flex items-center gap-3 rounded-lg bg-card px-3 py-2.5 ring-1 ring-foreground/10">
          <Checkbox
            checked={item.bought}
            onCheckedChange={() => {
              onToggle(item).catch(() => toast.error(t("list.markBoughtError")))
            }}
            className="size-5 rounded-full"
          />
          <div className="min-w-0 flex-1">
            <p
              className={cn(
                "truncate text-sm font-medium",
                item.bought && "text-muted-foreground"
              )}
            >
              {item.name}
            </p>
            {/* A bought item has no use for its note anymore, so the row drops
                it; the context menu still reads it back. */}
            {item.notes && !item.bought && (
              <p className="truncate text-xs text-muted-foreground">
                {item.notes}
              </p>
            )}
            {dateLabel && (
              <p className="text-[0.65rem] text-primary">{dateLabel}</p>
            )}
          </div>

          {/* The context menu is the full set of actions, but deleting stays
              reachable in one tap for people who never try a right click. */}
          <Button
            type="button"
            variant="ghost"
            size="icon-sm"
            className="shrink-0 text-muted-foreground hover:text-destructive"
            onClick={() => setDeleteOpen(true)}
          >
            <HugeiconsIcon icon={Delete02Icon} />
            <span className="sr-only">{t("list.deleteItemMenu")}</span>
          </Button>
        </ContextMenuTrigger>

        <ContextMenuContent>
          {/* Bought items can only be read back, and there is nothing to read
              when they were never noted. */}
          {(!item.bought || item.notes) && (
            <ContextMenuItem onClick={() => setNoteOpen(true)}>
              <HugeiconsIcon icon={Note01Icon} />
              {item.bought ? t("list.noteViewMenu") : t("list.noteMenu")}
            </ContextMenuItem>
          )}
          {!item.bought && (
            <ContextMenuItem onClick={() => setMarkBoughtOpen(true)}>
              <HugeiconsIcon icon={ShoppingBasket01Icon} />
              {t("list.markBoughtMenu")}
            </ContextMenuItem>
          )}
          <ContextMenuSeparator />
          <ContextMenuItem
            variant="destructive"
            onClick={() => setDeleteOpen(true)}
          >
            <HugeiconsIcon icon={Delete02Icon} />
            {t("list.deleteItemMenu")}
          </ContextMenuItem>
        </ContextMenuContent>
      </ContextMenu>

      <ShopItemNoteDialog
        open={noteOpen}
        onOpenChange={setNoteOpen}
        item={item}
        onSave={(notes) => onSaveNote(item, notes)}
        readOnly={item.bought}
      />

      <AlertDialog open={markBoughtOpen} onOpenChange={setMarkBoughtOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {t("list.markBoughtDialogTitle")}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {t("list.markBoughtDialogDescription", { name: item.name })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={busy}>
              {t("list.cancel")}
            </AlertDialogCancel>
            <AlertDialogAction disabled={busy} onClick={onConfirmMarkBought}>
              {busy && <Spinner data-icon="inline-start" />}
              {t("list.markBoughtMenu")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AlertDialog open={deleteOpen} onOpenChange={setDeleteOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {t("list.deleteItemDialogTitle")}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {t("list.deleteItemDialogDescription", { name: item.name })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={busy}>
              {t("list.cancel")}
            </AlertDialogCancel>
            <AlertDialogAction
              variant="destructive"
              disabled={busy}
              onClick={onConfirmDelete}
            >
              {busy && <Spinner data-icon="inline-start" />}
              {t("list.deleteConfirmAction")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </motion.div>
  )
}
