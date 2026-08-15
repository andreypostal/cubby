import * as React from "react"
import { Link } from "react-router"
import { HugeiconsIcon } from "@hugeicons/react"
import { Add01Icon, ArrowDown01Icon, Target02Icon } from "@hugeicons/core-free-icons"

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
import { CreateRoutineDialog } from "@/components/create-routine-dialog"
import { useAuth } from "@/lib/auth-context"
import { useI18n } from "@/lib/i18n/context"
import type { Routine } from "@/lib/api"

const PAGE_SIZE = 5

function RoutineGoalsMenu() {
  const { workspace } = useAuth()

  // Remounts (and re-fetches from a clean state) whenever the selected workspace changes.
  return <RoutineGoalsMenuContent key={workspace?.id} />
}

function RoutineGoalsMenuContent() {
  const { listRoutines, subscribeRoutinesChanged } = useAuth()
  const { t } = useI18n()
  const { isMobile, setOpenMobile } = useSidebar()

  function closeMobileSidebar() {
    if (isMobile) {
      setOpenMobile(false)
    }
  }

  const [routines, setRoutines] = React.useState<Routine[] | null>(null)
  const [page, setPage] = React.useState(1)
  const [createOpen, setCreateOpen] = React.useState(false)

  React.useEffect(() => {
    listRoutines()
      .then(setRoutines)
      .catch(() => setRoutines([]))
  }, [listRoutines])

  React.useEffect(() => {
    return subscribeRoutinesChanged((event) => {
      if (event.type === "deleted") {
        setRoutines((current) =>
          current
            ? current.filter((routine) => routine.id !== event.routineId)
            : current
        )
      } else {
        setRoutines((current) =>
          current
            ? current.map((routine) =>
                routine.id === event.routine.id ? event.routine : routine
              )
            : current
        )
      }
    })
  }, [subscribeRoutinesChanged])

  function onCreated(routine: Routine) {
    setRoutines((current) => [routine, ...(current ?? [])])
    setPage(1)
  }

  const totalPages = routines
    ? Math.max(1, Math.ceil(routines.length / PAGE_SIZE))
    : 1
  if (page > totalPages) {
    setPage(totalPages)
  }
  const pageItems =
    routines?.slice((page - 1) * PAGE_SIZE, page * PAGE_SIZE) ?? []

  return (
    <Collapsible defaultOpen>
      <SidebarMenuItem>
        <CollapsibleTrigger render={<SidebarMenuButton className="group" />}>
          <HugeiconsIcon icon={Target02Icon} />
          <span>{t("sidebar.goals")}</span>
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
                <span>{t("sidebar.createGoal")}</span>
              </SidebarMenuSubButton>
            </SidebarMenuSubItem>

            {routines === null &&
              Array.from({ length: 3 }).map((_, index) => (
                <SidebarMenuSubItem key={index}>
                  <SidebarMenuSkeleton />
                </SidebarMenuSubItem>
              ))}

            {routines?.length === 0 && (
              <SidebarMenuSubItem>
                <span className="flex h-7 items-center px-2 text-xs text-sidebar-foreground/60">
                  {t("sidebar.noGoals")}
                </span>
              </SidebarMenuSubItem>
            )}

            {pageItems.map((routine) => (
              <SidebarMenuSubItem key={routine.id}>
                <SidebarMenuSubButton
                  render={<Link to={`/routines/${routine.id}`} />}
                  onClick={closeMobileSidebar}
                >
                  <span className="truncate">{routine.name}</span>
                </SidebarMenuSubButton>
              </SidebarMenuSubItem>
            ))}
          </SidebarMenuSub>

          {routines !== null && routines.length > PAGE_SIZE && (
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

      <CreateRoutineDialog
        open={createOpen}
        onOpenChange={setCreateOpen}
        onCreated={onCreated}
      />
    </Collapsible>
  )
}

export { RoutineGoalsMenu }
