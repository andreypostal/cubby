import * as React from "react"

import * as api from "@/lib/api"
import { ApiError } from "@/lib/api"
import type {
  Routine,
  ShopList,
  UpdateRoutinePayload,
  User,
  Workspace,
} from "@/lib/api"

const TOKEN_KEY = "auth_token"

type AuthStatus = "loading" | "authenticated" | "unauthenticated"

type ListsChangeEvent =
  | { type: "created"; list: ShopList }
  | { type: "updated"; list: ShopList }
  | { type: "deleted"; listId: number }

type RoutinesChangeEvent =
  { type: "updated"; routine: Routine } | { type: "deleted"; routineId: number }

/**
 * Every route but the login is behind the session token, and nothing else in
 * the app is allowed to reach for it — the context is the only reader.
 */
function requireToken() {
  const token = localStorage.getItem(TOKEN_KEY)
  if (!token) {
    throw new ApiError(401, "Not authenticated")
  }
  return token
}

/**
 * Binds an `@/lib/api` call to the stored session token, hiding it from the
 * components. Signed out, the returned function rejects with a 401 instead of
 * reaching the network — the api functions are module level, so the identity of
 * the result is stable for the whole lifetime of the provider.
 */
function useAuthed<A extends unknown[], R>(
  call: (token: string, ...args: A) => Promise<R>
) {
  return React.useCallback(
    async (...args: A) => call(requireToken(), ...args),
    [call]
  )
}

/** A listener set plus the callbacks that read and write it. */
function useChangeListeners<E>() {
  const listeners = React.useRef(new Set<(event: E) => void>()).current

  const subscribe = React.useCallback(
    (listener: (event: E) => void) => {
      listeners.add(listener)
      return () => {
        listeners.delete(listener)
      }
    },
    [listeners]
  )

  const emit = React.useCallback(
    (event: E) => listeners.forEach((listener) => listener(event)),
    [listeners]
  )

  return { subscribe, emit }
}

function useAuthValue() {
  const [status, setStatus] = React.useState<AuthStatus>("loading")
  const [user, setUser] = React.useState<User | null>(null)
  const [workspace, setWorkspace] = React.useState<Workspace | null>(null)
  const [setupRequired, setSetupRequired] = React.useState(false)

  const { subscribe: subscribeListsChanged, emit: emitLists } =
    useChangeListeners<ListsChangeEvent>()
  const { subscribe: subscribeRoutinesChanged, emit: emitRoutines } =
    useChangeListeners<RoutinesChangeEvent>()

  const loadMe = React.useCallback(async (token: string) => {
    const res = await api.me(token)
    setUser(res.user)
    setWorkspace(res.current_workspace ?? null)
    setStatus("authenticated")
  }, [])

  /**
   * A server nobody has set up yet has no users, so a stored token could only
   * be a leftover and there is nothing to sign in to. The setup status is
   * asked first for that reason, and a server that cannot answer is treated as
   * set up: the ordinary sign-in path then reports whatever is really wrong.
   */
  React.useEffect(() => {
    const start = async () => {
      const { required } = await api
        .getSetupStatus()
        .catch(() => ({ required: false }))

      if (required) {
        localStorage.removeItem(TOKEN_KEY)
        setSetupRequired(true)
        setStatus("unauthenticated")
        return
      }

      const token = localStorage.getItem(TOKEN_KEY)
      if (!token) {
        setStatus("unauthenticated")
        return
      }

      await loadMe(token).catch(() => {
        localStorage.removeItem(TOKEN_KEY)
        setStatus("unauthenticated")
      })
    }

    start()
  }, [loadMe])

  const login = React.useCallback(
    async (username: string, password: string) => {
      const res = await api.login(username, password)
      localStorage.setItem(TOKEN_KEY, res.token)
      setUser(res.user)
      setWorkspace(null)
      setStatus("authenticated")
    },
    []
  )

  /**
   * Creates the first administrator, then signs in as them: whoever just
   * proved they can read the server's log has no reason to type the password
   * again on the login screen.
   */
  const completeSetup = React.useCallback(
    async (
      token: string,
      name: string,
      username: string,
      password: string
    ) => {
      await api.completeSetup({ token, name, username, password })
      setSetupRequired(false)
      await login(username, password)
    },
    [login]
  )

  const logout = React.useCallback(() => {
    const token = localStorage.getItem(TOKEN_KEY)
    if (token) {
      api.logout(token).catch(() => {})
    }
    localStorage.removeItem(TOKEN_KEY)
    setUser(null)
    setWorkspace(null)
    setStatus("unauthenticated")
  }, [])

  const selectWorkspace = React.useCallback(
    async (workspaceId: number) => {
      const token = localStorage.getItem(TOKEN_KEY)
      if (!token) {
        return
      }
      await api.switchWorkspace(token, workspaceId)
      await loadMe(token)
    },
    [loadMe]
  )

  const resetWorkspace = React.useCallback(async () => {
    const token = localStorage.getItem(TOKEN_KEY)
    if (!token) {
      return
    }
    await api.resetWorkspace(token)
    setWorkspace(null)
  }, [])

  const refresh = React.useCallback(async () => {
    const token = localStorage.getItem(TOKEN_KEY)
    if (!token) {
      return
    }
    await loadMe(token)
  }, [loadMe])

  const listWorkspaces = React.useCallback(async () => {
    const token = localStorage.getItem(TOKEN_KEY)
    return token ? (await api.listWorkspaces(token)).workspaces : []
  }, [])

  const createWorkspace = useAuthed(api.createWorkspace)

  const renameWorkspace = React.useCallback(
    async (workspaceId: number, name: string) => {
      const updated = await api.updateWorkspace(
        requireToken(),
        workspaceId,
        name
      )
      setWorkspace((current) =>
        current && current.id === workspaceId ? updated : current
      )
      return updated
    },
    []
  )

  /**
   * Both of these end the access the session was working under, so the active
   * workspace is dropped and the app falls back to the selection screen.
   */
  const forgetWorkspace = React.useCallback(
    async (token: string, workspaceId: number) => {
      setWorkspace((current) =>
        current && current.id === workspaceId ? null : current
      )
      await api.resetWorkspace(token).catch(() => {})
    },
    []
  )

  const deleteWorkspace = React.useCallback(
    async (workspaceId: number) => {
      const token = requireToken()
      await api.deleteWorkspace(token, workspaceId)
      await forgetWorkspace(token, workspaceId)
    },
    [forgetWorkspace]
  )

  const leaveWorkspace = React.useCallback(
    async (workspaceId: number) => {
      const token = requireToken()
      await api.leaveWorkspace(token, workspaceId)
      await forgetWorkspace(token, workspaceId)
    },
    [forgetWorkspace]
  )

  const listWorkspaceMembers = React.useCallback(
    async (workspaceId: number) =>
      (await api.listWorkspaceMembers(requireToken(), workspaceId)).members,
    []
  )

  const shareWorkspace = useAuthed(api.shareWorkspace)
  const revokeWorkspaceAccess = useAuthed(api.revokeWorkspaceAccess)

  const listLists = React.useCallback(async () => {
    const token = localStorage.getItem(TOKEN_KEY)
    return token ? (await api.listShopLists(token)).lists : []
  }, [])

  const createList = React.useCallback(
    async (name: string, icon: string) => {
      const created = await api.createShopList(requireToken(), name, icon)
      emitLists({ type: "created", list: created })
      return created
    },
    [emitLists]
  )

  const getList = useAuthed(api.getShopList)

  const updateList = React.useCallback(
    async (listId: number, name: string, icon: string) => {
      const updated = await api.updateShopList(
        requireToken(),
        listId,
        name,
        icon
      )
      emitLists({ type: "updated", list: updated })
      return updated
    },
    [emitLists]
  )

  const deleteList = React.useCallback(
    async (listId: number) => {
      await api.deleteShopList(requireToken(), listId)
      emitLists({ type: "deleted", listId })
    },
    [emitLists]
  )

  const createListItem = useAuthed(api.createShopListItem)
  const updateListItem = useAuthed(api.updateShopListItem)
  const toggleListItem = useAuthed(api.toggleShopListItem)
  const deleteListItem = useAuthed(api.deleteShopListItem)
  const clearBoughtListItems = useAuthed(api.clearBoughtShopListItems)

  const suggestItemNames = React.useCallback(
    async (listId: number, term: string) =>
      (await api.suggestShopListItemNames(requireToken(), listId, term)).terms,
    []
  )

  const suggestIngredientNames = React.useCallback(
    async (term: string) =>
      (await api.suggestRecipeIngredientNames(requireToken(), term)).terms,
    []
  )

  const listRoutines = React.useCallback(async () => {
    const token = localStorage.getItem(TOKEN_KEY)
    return token ? (await api.listRoutines(token)).routines : []
  }, [])

  const createRoutine = useAuthed(api.createRoutine)
  const getRoutine = useAuthed(api.getRoutine)
  const getRoutineMonth = useAuthed(api.getRoutineMonth)

  const updateRoutine = React.useCallback(
    async (routineId: number, payload: UpdateRoutinePayload) => {
      const updated = await api.updateRoutine(requireToken(), routineId, payload)
      emitRoutines({ type: "updated", routine: updated })
      return updated
    },
    [emitRoutines]
  )

  const deleteRoutine = React.useCallback(
    async (routineId: number) => {
      await api.deleteRoutine(requireToken(), routineId)
      emitRoutines({ type: "deleted", routineId })
    },
    [emitRoutines]
  )

  const markRoutineDay = React.useCallback(
    async (routineId: number, date: string, achieved: boolean) => {
      const result = await api.markRoutineDay(
        requireToken(),
        routineId,
        date,
        achieved
      )
      emitRoutines({ type: "updated", routine: result.routine })
      return result
    },
    [emitRoutines]
  )

  const unmarkRoutineDay = React.useCallback(
    async (routineId: number, date: string) => {
      const updated = await api.unmarkRoutineDay(requireToken(), routineId, date)
      emitRoutines({ type: "updated", routine: updated })
      return updated
    },
    [emitRoutines]
  )

  const getTaskBoard = useAuthed(api.getTaskBoard)
  const updateTaskBoard = useAuthed(api.updateTaskBoard)
  const getTaskDay = useAuthed(api.getTaskDay)
  const listTasks = useAuthed(api.listTasks)
  const createTask = useAuthed(api.createTask)
  const updateTask = useAuthed(api.updateTask)
  const stopTask = useAuthed(api.stopTask)
  const resumeTask = useAuthed(api.resumeTask)
  const deleteTask = useAuthed(api.deleteTask)
  const markTaskDay = useAuthed(api.markTaskDay)
  const clearTaskDay = useAuthed(api.clearTaskDay)

  const listRecipeCategories = React.useCallback(async () => {
    const token = localStorage.getItem(TOKEN_KEY)
    return token ? (await api.listRecipeCategories(token)).categories : []
  }, [])

  const createRecipeCategory = useAuthed(api.createRecipeCategory)
  const updateRecipeCategory = useAuthed(api.updateRecipeCategory)
  const deleteRecipeCategory = useAuthed(api.deleteRecipeCategory)
  const listRecipes = useAuthed(api.listRecipes)
  const createRecipe = useAuthed(api.createRecipe)
  const getRecipe = useAuthed(api.getRecipe)
  const updateRecipe = useAuthed(api.updateRecipe)
  const setRecipeFavorite = useAuthed(api.setRecipeFavorite)
  const archiveRecipe = useAuthed(api.archiveRecipe)
  const unarchiveRecipe = useAuthed(api.unarchiveRecipe)
  const deleteRecipe = useAuthed(api.deleteRecipe)

  const listBackups = useAuthed(api.listBackups)
  const generateBackup = useAuthed(api.generateBackup)
  const downloadBackup = useAuthed(api.downloadBackup)
  const deleteBackup = useAuthed(api.deleteBackup)
  const restoreBackup = useAuthed(api.restoreBackup)
  const getTLSSettings = useAuthed(api.getTLSSettings)
  const getSessionCleanupReport = useAuthed(api.getSessionCleanupReport)

  const listUsers = React.useCallback(
    async () => (await api.listUsers(requireToken())).users,
    []
  )
  const createUser = useAuthed(api.createUser)
  const updateUser = useAuthed(api.updateUser)
  const changeUserPassword = useAuthed(api.changeUserPassword)
  const deleteUser = useAuthed(api.deleteUser)

  return {
    status,
    user,
    workspace,
    setupRequired,
    completeSetup,
    login,
    logout,
    selectWorkspace,
    resetWorkspace,
    refresh,
    listWorkspaces,
    createWorkspace,
    renameWorkspace,
    deleteWorkspace,
    listWorkspaceMembers,
    shareWorkspace,
    revokeWorkspaceAccess,
    leaveWorkspace,
    listLists,
    createList,
    getList,
    updateList,
    deleteList,
    createListItem,
    updateListItem,
    suggestItemNames,
    suggestIngredientNames,
    toggleListItem,
    deleteListItem,
    clearBoughtListItems,
    subscribeListsChanged,
    listRoutines,
    createRoutine,
    getRoutine,
    updateRoutine,
    deleteRoutine,
    getRoutineMonth,
    markRoutineDay,
    unmarkRoutineDay,
    subscribeRoutinesChanged,
    getTaskBoard,
    updateTaskBoard,
    getTaskDay,
    listTasks,
    createTask,
    updateTask,
    stopTask,
    resumeTask,
    deleteTask,
    markTaskDay,
    clearTaskDay,
    listRecipeCategories,
    createRecipeCategory,
    updateRecipeCategory,
    deleteRecipeCategory,
    listRecipes,
    createRecipe,
    getRecipe,
    updateRecipe,
    setRecipeFavorite,
    archiveRecipe,
    unarchiveRecipe,
    deleteRecipe,
    listBackups,
    generateBackup,
    downloadBackup,
    deleteBackup,
    restoreBackup,
    getTLSSettings,
    getSessionCleanupReport,
    listUsers,
    createUser,
    updateUser,
    changeUserPassword,
    deleteUser,
  }
}

/** The provider's own value, so the contract cannot drift from what it builds. */
type AuthContextValue = ReturnType<typeof useAuthValue>

const AuthContext = React.createContext<AuthContextValue | null>(null)

function AuthProvider({ children }: { children: React.ReactNode }) {
  const value = useAuthValue()

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

function useAuth() {
  const ctx = React.useContext(AuthContext)
  if (!ctx) {
    throw new Error("useAuth must be used within an AuthProvider")
  }
  return ctx
}

export { AuthProvider, useAuth, ApiError }
