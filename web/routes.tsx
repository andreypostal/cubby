import { Navigate, Route, Routes } from "react-router"

import AdminPage from "@/pages/admin"
import AppLayout from "@/pages/app-layout"
import LoginPage from "@/pages/login"
import RecipeDetailPage from "@/pages/recipe-detail"
import RecipeEditPage from "@/pages/recipe-edit"
import RecipeNewPage from "@/pages/recipe-new"
import RecipesPage from "@/pages/recipes"
import RoutineDetailPage from "@/pages/routine-detail"
import SetupPage from "@/pages/setup"
import ShopListPage from "@/pages/shop-list"
import TasksBoardPage from "@/pages/tasks"
import WorkspaceSelectionPage from "@/pages/workspace"

function AppRoutes() {
  return (
    <Routes>
      {/* Pathless layout route, replacing the Next "(app)" route group: it
          gates on auth and renders the sidebar shell around every child. */}
      <Route element={<AppLayout />}>
        {/* The board is the home page: picking a cubby lands straight on the
            day's tasks, which is the only thing anyone opens this for. */}
        <Route index element={<TasksBoardPage />} />
        <Route path="tasks" element={<TasksBoardPage />} />
        <Route path="recipes" element={<RecipesPage />} />
        <Route path="recipes/new" element={<RecipeNewPage />} />
        <Route path="recipes/:recipeID" element={<RecipeDetailPage />} />
        <Route path="recipes/:recipeID/edit" element={<RecipeEditPage />} />
        <Route path="routines/:routineID" element={<RoutineDetailPage />} />
        <Route path="shop/lists/:listID" element={<ShopListPage />} />
      </Route>

      {/* First run: a server with no users at all sends everyone here. */}
      <Route path="setup" element={<SetupPage />} />
      <Route path="login" element={<LoginPage />} />
      <Route path="workspace" element={<WorkspaceSelectionPage />} />
      {/* Outside the app shell: it manages the server, not a workspace. */}
      <Route path="admin" element={<AdminPage />} />

      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  )
}

export { AppRoutes }
