import * as React from "react"
import { Link, useNavigate } from "react-router"
import { HugeiconsIcon } from "@hugeicons/react"
import {
  Building05Icon,
  Logout05Icon,
  ChefHatIcon,
  Settings02Icon,
  TaskDaily01Icon,
  UnfoldMoreIcon,
} from "@hugeicons/core-free-icons"

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
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupLabel,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarSeparator,
  useSidebar,
} from "@/components/ui/sidebar"
import { ConfigurationDialog } from "@/components/configuration-dialog"
import { RoutineGoalsMenu } from "@/components/routine-goals-menu"
import { ShopListsMenu } from "@/components/shop-lists-menu"
import { useAuth } from "@/lib/auth-context"
import { useI18n } from "@/lib/i18n/context"

function AppSidebar() {
  const navigate = useNavigate()
  const { workspace, resetWorkspace, logout } = useAuth()
  const { t } = useI18n()
  const { isMobile, setOpenMobile } = useSidebar()

  const [confirmOpen, setConfirmOpen] = React.useState(false)
  const [changing, setChanging] = React.useState(false)
  const [logoutConfirmOpen, setLogoutConfirmOpen] = React.useState(false)
  const [configOpen, setConfigOpen] = React.useState(false)

  function closeMobileSidebar() {
    if (isMobile) {
      setOpenMobile(false)
    }
  }

  async function onConfirmChangeWorkspace() {
    setChanging(true)
    try {
      await resetWorkspace()
      setConfirmOpen(false)
      navigate("/workspace", { replace: true })
    } finally {
      setChanging(false)
    }
  }

  return (
    <Sidebar collapsible="icon">
      <SidebarHeader>
        <SidebarMenu>
          <SidebarMenuItem>
            <SidebarMenuButton
              size="lg"
              onClick={() => {
                closeMobileSidebar()
                setConfirmOpen(true)
              }}
            >
              <div className="flex size-6 shrink-0 items-center justify-center rounded-md bg-primary text-primary-foreground">
                <HugeiconsIcon icon={Building05Icon} className="size-3.5" />
              </div>
              <div className="grid min-w-0 flex-1 text-left leading-tight">
                <span className="truncate font-medium">{workspace?.name}</span>
                <span className="truncate text-[0.65rem] text-sidebar-foreground/60">
                  {t("sidebar.switchWorkspace")}
                </span>
              </div>
              <HugeiconsIcon icon={UnfoldMoreIcon} className="ml-auto" />
            </SidebarMenuButton>
            <AlertDialog open={confirmOpen} onOpenChange={setConfirmOpen}>
              <AlertDialogContent forceOverlay>
                <AlertDialogHeader>
                  <AlertDialogTitle>
                    {t("sidebar.switchWorkspaceDialogTitle")}
                  </AlertDialogTitle>
                  <AlertDialogDescription>
                    {t("sidebar.switchWorkspaceDialogDescription")}
                  </AlertDialogDescription>
                </AlertDialogHeader>
                <AlertDialogFooter>
                  <AlertDialogCancel disabled={changing}>
                    {t("sidebar.cancel")}
                  </AlertDialogCancel>
                  <AlertDialogAction
                    disabled={changing}
                    onClick={onConfirmChangeWorkspace}
                  >
                    {t("sidebar.switchWorkspaceConfirm")}
                  </AlertDialogAction>
                </AlertDialogFooter>
              </AlertDialogContent>
            </AlertDialog>
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarHeader>
      <SidebarContent>
        <SidebarGroup>
          <SidebarMenu>
            <SidebarMenuItem>
              <SidebarMenuButton
                render={<Link to="/tasks" />}
                onClick={closeMobileSidebar}
              >
                <HugeiconsIcon icon={TaskDaily01Icon} />
                <span>{t("sidebar.tasksBoard")}</span>
              </SidebarMenuButton>
            </SidebarMenuItem>
            <SidebarMenuItem>
              <SidebarMenuButton
                render={<Link to="/recipes" />}
                onClick={closeMobileSidebar}
              >
                <HugeiconsIcon icon={ChefHatIcon} />
                <span>{t("sidebar.recipes")}</span>
              </SidebarMenuButton>
            </SidebarMenuItem>
          </SidebarMenu>
        </SidebarGroup>
        <SidebarGroup>
          <SidebarGroupLabel>{t("sidebar.shop")}</SidebarGroupLabel>
          <SidebarMenu>
            <ShopListsMenu />
          </SidebarMenu>
        </SidebarGroup>
        <SidebarGroup>
          <SidebarGroupLabel>{t("sidebar.routine")}</SidebarGroupLabel>
          <SidebarMenu>
            <RoutineGoalsMenu />
          </SidebarMenu>
        </SidebarGroup>
      </SidebarContent>
      <SidebarFooter>
        <SidebarMenu>
          <SidebarMenuItem>
            <SidebarMenuButton
              onClick={() => {
                closeMobileSidebar()
                setConfigOpen(true)
              }}
            >
              <HugeiconsIcon icon={Settings02Icon} />
              <span>{t("sidebar.configuration")}</span>
            </SidebarMenuButton>
            <ConfigurationDialog
              open={configOpen}
              onOpenChange={setConfigOpen}
            />
          </SidebarMenuItem>
        </SidebarMenu>
        <SidebarSeparator className="mx-0" />
        <SidebarMenu>
          <SidebarMenuItem>
            <SidebarMenuButton
              onClick={() => {
                closeMobileSidebar()
                setLogoutConfirmOpen(true)
              }}
            >
              <HugeiconsIcon icon={Logout05Icon} />
              <span>{t("sidebar.logout")}</span>
            </SidebarMenuButton>
            <AlertDialog
              open={logoutConfirmOpen}
              onOpenChange={setLogoutConfirmOpen}
            >
              <AlertDialogContent forceOverlay>
                <AlertDialogHeader>
                  <AlertDialogTitle>
                    {t("sidebar.logoutDialogTitle")}
                  </AlertDialogTitle>
                  <AlertDialogDescription>
                    {t("sidebar.logoutDialogDescription")}
                  </AlertDialogDescription>
                </AlertDialogHeader>
                <AlertDialogFooter>
                  <AlertDialogCancel>{t("sidebar.cancel")}</AlertDialogCancel>
                  <AlertDialogAction onClick={logout}>
                    {t("sidebar.logoutConfirm")}
                  </AlertDialogAction>
                </AlertDialogFooter>
              </AlertDialogContent>
            </AlertDialog>
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarFooter>
    </Sidebar>
  )
}

export { AppSidebar }
