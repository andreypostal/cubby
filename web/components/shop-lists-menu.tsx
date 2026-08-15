import * as React from "react"
import { Link, useNavigate } from "react-router"
import { HugeiconsIcon } from "@hugeicons/react"
import { Add01Icon, ArrowDown01Icon } from "@hugeicons/core-free-icons"

import { cn } from "@/lib/utils"
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "@/components/ui/collapsible"
import {
  Pagination,
  PaginationContent,
  PaginationItem,
  PaginationNext,
  PaginationPrevious,
} from "@/components/ui/pagination"
import {
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarMenuSkeleton,
  SidebarMenuSub,
  SidebarMenuSubButton,
  SidebarMenuSubItem,
  useSidebar,
} from "@/components/ui/sidebar"
import { CreateListDialog } from "@/components/create-list-dialog"
import { useAuth } from "@/lib/auth-context"
import { useI18n } from "@/lib/i18n/context"
import { getListIcon } from "@/lib/list-icons"
import type { ShopList } from "@/lib/api"

const PAGE_SIZE = 5

function ShopListsMenu() {
  const { workspace } = useAuth()

  // Remounts (and re-fetches from a clean state) whenever the selected workspace changes.
  return <ShopListsMenuContent key={workspace?.id} />
}

function ShopListsMenuContent() {
  const { listLists, subscribeListsChanged } = useAuth()
  const { t } = useI18n()
  const navigate = useNavigate()
  const { isMobile, setOpenMobile } = useSidebar()

  function closeMobileSidebar() {
    if (isMobile) {
      setOpenMobile(false)
    }
  }

  const [lists, setLists] = React.useState<ShopList[] | null>(null)
  const [page, setPage] = React.useState(1)
  const [createOpen, setCreateOpen] = React.useState(false)

  React.useEffect(() => {
    listLists()
      .then(setLists)
      .catch(() => setLists([]))
  }, [listLists])

  React.useEffect(() => {
    return subscribeListsChanged((event) => {
      if (event.type === "deleted") {
        setLists((current) =>
          current ? current.filter((list) => list.id !== event.listId) : current
        )
      } else if (event.type === "created") {
        setLists((current) => (current ? [event.list, ...current] : current))
      } else {
        setLists((current) =>
          current
            ? current.map((list) =>
                list.id === event.list.id ? event.list : list
              )
            : current
        )
      }
    })
  }, [subscribeListsChanged])

  // The list itself arrives through the change subscription above, wherever in
  // the app it was created; this only follows it.
  function onCreated(list: ShopList) {
    setPage(1)
    navigate(`/shop/lists/${list.id}`)
  }

  const totalPages = lists
    ? Math.max(1, Math.ceil(lists.length / PAGE_SIZE))
    : 1
  if (page > totalPages) {
    setPage(totalPages)
  }
  const pageItems = lists?.slice((page - 1) * PAGE_SIZE, page * PAGE_SIZE) ?? []

  return (
    <Collapsible defaultOpen>
      <SidebarMenuItem>
        <CollapsibleTrigger render={<SidebarMenuButton className="group" />}>
          <span>{t("sidebar.lists")}</span>
          <HugeiconsIcon
            icon={ArrowDown01Icon}
            className="ml-auto transition-transform group-data-[state=open]:rotate-180"
          />
        </CollapsibleTrigger>
        <CollapsibleContent>
          <SidebarMenuSub>
            <SidebarMenuSubItem>
              <SidebarMenuSubButton
                onClick={() => {
                  closeMobileSidebar()
                  setCreateOpen(true)
                }}
              >
                <HugeiconsIcon icon={Add01Icon} />
                <span>{t("sidebar.createList")}</span>
              </SidebarMenuSubButton>
            </SidebarMenuSubItem>

            {lists === null &&
              Array.from({ length: 3 }).map((_, index) => (
                <SidebarMenuSubItem key={index}>
                  <SidebarMenuSkeleton />
                </SidebarMenuSubItem>
              ))}

            {lists?.length === 0 && (
              <SidebarMenuSubItem>
                <span className="flex h-7 items-center px-2 text-xs text-sidebar-foreground/60">
                  {t("sidebar.noLists")}
                </span>
              </SidebarMenuSubItem>
            )}

            {pageItems.map((list) => (
              <SidebarMenuSubItem key={list.id}>
                <SidebarMenuSubButton
                  render={<Link to={`/shop/lists/${list.id}`} />}
                  onClick={closeMobileSidebar}
                >
                  <HugeiconsIcon icon={getListIcon(list.icon)} />
                  <span className="truncate">{list.name}</span>
                </SidebarMenuSubButton>
              </SidebarMenuSubItem>
            ))}
          </SidebarMenuSub>

          {lists !== null && lists.length > PAGE_SIZE && (
            <Pagination className="mx-0 mt-1 w-auto justify-start pl-2">
              <PaginationContent className="gap-0.5">
                <PaginationItem>
                  <PaginationPrevious
                    href="#"
                    text=""
                    size="icon-sm"
                    className={cn(
                      page === 1 && "pointer-events-none opacity-40"
                    )}
                    onClick={(event) => {
                      event.preventDefault()
                      setPage((current) => Math.max(1, current - 1))
                    }}
                  />
                </PaginationItem>
                <PaginationItem>
                  <span className="block px-1 text-[0.65rem] text-sidebar-foreground/60 tabular-nums">
                    {page}/{totalPages}
                  </span>
                </PaginationItem>
                <PaginationItem>
                  <PaginationNext
                    href="#"
                    text=""
                    size="icon-sm"
                    className={cn(
                      page === totalPages && "pointer-events-none opacity-40"
                    )}
                    onClick={(event) => {
                      event.preventDefault()
                      setPage((current) => Math.min(totalPages, current + 1))
                    }}
                  />
                </PaginationItem>
              </PaginationContent>
            </Pagination>
          )}
        </CollapsibleContent>
      </SidebarMenuItem>

      <CreateListDialog
        open={createOpen}
        onOpenChange={setCreateOpen}
        onCreated={onCreated}
      />
    </Collapsible>
  )
}

export { ShopListsMenu }
