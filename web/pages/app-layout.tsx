import * as React from "react"
import { Outlet, useNavigate } from "react-router"

import { AppSidebar } from "@/components/app-sidebar"
import {
  SidebarInset,
  SidebarProvider,
  SidebarTrigger,
} from "@/components/ui/sidebar"
import { Spinner } from "@/components/ui/spinner"
import { useAuth } from "@/lib/auth-context"

export default function AppLayout() {
  const navigate = useNavigate()
  const { status, setupRequired, workspace } = useAuth()

  React.useEffect(() => {
    if (setupRequired) {
      navigate("/setup", { replace: true })
    } else if (status === "unauthenticated") {
      navigate("/login", { replace: true })
    } else if (status === "authenticated" && !workspace) {
      navigate("/workspace", { replace: true })
    }
  }, [status, setupRequired, workspace, navigate])

  if (status === "loading" || status === "unauthenticated" || !workspace) {
    return (
      <div className="flex min-h-svh items-center justify-center">
        <Spinner className="size-6" />
      </div>
    )
  }

  return (
    <SidebarProvider>
      <AppSidebar />
      <SidebarInset>
        <header className="flex h-12 shrink-0 items-center gap-2 px-4">
          <SidebarTrigger />
        </header>
        <div className="flex flex-1 flex-col gap-4 p-4">
          <Outlet />
        </div>
      </SidebarInset>
    </SidebarProvider>
  )
}
