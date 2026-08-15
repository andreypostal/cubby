import * as React from "react"
import { createRoot } from "react-dom/client"
import { BrowserRouter } from "react-router"

import "@fontsource-variable/inter"
import "@fontsource-variable/geist-mono"
import "./styles/globals.css"

import { AppRoutes } from "@/routes"
import { ThemeProvider } from "@/components/theme-provider"
import { Toaster } from "@/components/ui/sonner"
import { AuthProvider } from "@/lib/auth-context"
import { I18nProvider } from "@/lib/i18n/context"

const container = document.getElementById("root")

if (!container) {
  throw new Error("Root container #root not found")
}

createRoot(container).render(
  <React.StrictMode>
    <ThemeProvider>
      <I18nProvider>
        <BrowserRouter>
          <AuthProvider>
            <AppRoutes />
          </AuthProvider>
        </BrowserRouter>
      </I18nProvider>
      <Toaster />
    </ThemeProvider>
  </React.StrictMode>
)
