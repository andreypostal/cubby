// Same origin in both modes: the Go binary serves the built UI in production,
// and Vite proxies /api to it in dev (see server.proxy in vite.config.ts).
const API_URL = import.meta.env.VITE_API_URL ?? "/api/v1"

export type User = {
  id: number
  name: string
  username: string
  is_system_admin: boolean
}

export type Workspace = {
  id: number
  name: string
  /** Only the creator may rename, delete or share the workspace. */
  is_creator: boolean
}

/** A user the workspace was shared with. The creator is never listed. */
export type WorkspaceMember = {
  id: number
  username: string
  name: string
}

export type WorkspaceMemberListResponse = {
  members: WorkspaceMember[]
}

export type AuthResponse = {
  token: string
  user: User
}

export type MeResponse = {
  user: User
  current_workspace?: Workspace
}

export type WorkspaceListResponse = {
  workspaces: Workspace[]
}

export type ShopList = {
  id: number
  name: string
  icon: string
  created_at: string
}

export type ShopListsResponse = {
  lists: ShopList[]
}

export type ShopListItem = {
  id: number
  name: string
  notes: string
  bought: boolean
  bought_at: string
  created_at: string
}

export type ShopListDetail = {
  id: number
  name: string
  icon: string
  items: ShopListItem[]
}

export type ItemSuggestionsResponse = {
  terms: string[]
}

export type Routine = {
  id: number
  name: string
  description: string
  pinned: boolean
  active: boolean
  points: number
  created_at: string
  on_monday: boolean
  on_tuesday: boolean
  on_wednesday: boolean
  on_thursday: boolean
  on_friday: boolean
  on_saturday: boolean
  on_sunday: boolean
}

export type RoutinesResponse = {
  routines: Routine[]
}

export type CreateRoutinePayload = {
  name: string
  description: string
  pinned: boolean
  on_monday: boolean
  on_tuesday: boolean
  on_wednesday: boolean
  on_thursday: boolean
  on_friday: boolean
  on_saturday: boolean
  on_sunday: boolean
}

export type UpdateRoutinePayload = CreateRoutinePayload & {
  active: boolean
}

export type RoutineDayStatus =
  "disabled" | "achieved" | "failed" | "waiting" | "future"

export type RoutineDay = {
  day: string
  status: RoutineDayStatus
}

export type RoutineMonth = {
  routine_id: number
  year: number
  month: number
  days: RoutineDay[]
}

export type MarkRoutineDayResult = {
  routine: Routine
  day: RoutineDay
}

export class ApiError extends Error {
  status: number
  /**
   * The `details` object the API sends alongside the message, when it sends
   * one. Refusals that need naming the thing in the way put it here.
   */
  details?: Record<string, unknown>

  constructor(
    status: number,
    message: string,
    details?: Record<string, unknown>
  ) {
    super(message)
    this.status = status
    this.details = details
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${API_URL}${path}`, {
    ...init,
    headers: {
      "Content-Type": "application/json",
      ...init?.headers,
    },
  })

  if (!res.ok) {
    const body = await res.json().catch(() => null)
    throw new ApiError(res.status, body?.message ?? res.statusText, body?.details)
  }

  if (res.status === 204) {
    return undefined as T
  }

  return res.json() as Promise<T>
}

/** Every route below `/auth/login` is behind the session bearer token. */
function authed<T>(token: string, path: string, init?: RequestInit) {
  return request<T>(path, {
    ...init,
    headers: { Authorization: `Bearer ${token}`, ...init?.headers },
  })
}

export function login(username: string, password: string) {
  return request<AuthResponse>("/auth/login", {
    method: "POST",
    body: JSON.stringify({ username, password }),
  })
}

export function me(token: string) {
  return authed<MeResponse>(token, "/auth/me")
}

export function logout(token: string) {
  return authed<void>(token, "/auth/logout", { method: "DELETE" })
}

export function listWorkspaces(token: string) {
  return authed<WorkspaceListResponse>(token, "/auth/workspaces")
}

export function createWorkspace(token: string, name: string) {
  return authed<Workspace>(token, "/auth/workspaces", {
    method: "POST",
    body: JSON.stringify({ name }),
  })
}

export function updateWorkspace(
  token: string,
  workspaceId: number,
  name: string
) {
  return authed<Workspace>(token, `/auth/workspaces/${workspaceId}`, {
    method: "PATCH",
    body: JSON.stringify({ name }),
  })
}

export function deleteWorkspace(token: string, workspaceId: number) {
  return authed<void>(token, `/auth/workspaces/${workspaceId}`, {
    method: "DELETE",
  })
}

export function listWorkspaceMembers(token: string, workspaceId: number) {
  return authed<WorkspaceMemberListResponse>(
    token,
    `/auth/workspaces/${workspaceId}/members`
  )
}

/** 404 when no such username exists, 409 when they already have access. */
export function shareWorkspace(
  token: string,
  workspaceId: number,
  username: string
) {
  return authed<WorkspaceMember>(
    token,
    `/auth/workspaces/${workspaceId}/members`,
    { method: "POST", body: JSON.stringify({ username }) }
  )
}

export function revokeWorkspaceAccess(
  token: string,
  workspaceId: number,
  userId: number
) {
  return authed<void>(
    token,
    `/auth/workspaces/${workspaceId}/members/${userId}`,
    { method: "DELETE" }
  )
}

/** Only for a shared user: the creator deletes the workspace instead. */
export function leaveWorkspace(token: string, workspaceId: number) {
  return authed<void>(token, `/auth/workspaces/${workspaceId}/leave`, {
    method: "POST",
  })
}

export function switchWorkspace(token: string, workspaceId: number) {
  return authed<void>(token, "/auth/me/workspace", {
    method: "PATCH",
    body: JSON.stringify({ workspace_id: workspaceId }),
  })
}

export function resetWorkspace(token: string) {
  return authed<void>(token, "/auth/me/workspace", { method: "DELETE" })
}

export function listShopLists(token: string) {
  return authed<ShopListsResponse>(token, "/shop/lists")
}

export function createShopList(token: string, name: string, icon: string) {
  return authed<ShopList>(token, "/shop/lists", {
    method: "POST",
    body: JSON.stringify({ name, icon }),
  })
}

export function getShopList(token: string, listId: number) {
  return authed<ShopListDetail>(token, `/shop/lists/${listId}`)
}

export function updateShopList(
  token: string,
  listId: number,
  name: string,
  icon: string
) {
  return authed<ShopList>(token, `/shop/lists/${listId}`, {
    method: "PATCH",
    body: JSON.stringify({ name, icon }),
  })
}

export function deleteShopList(token: string, listId: number) {
  return authed<void>(token, `/shop/lists/${listId}`, { method: "DELETE" })
}

export function createShopListItem(
  token: string,
  listId: number,
  name: string
) {
  return authed<ShopListItem>(token, `/shop/lists/${listId}/items`, {
    method: "POST",
    body: JSON.stringify({ name }),
  })
}

export type UpdateShopListItemPayload = {
  name?: string
  notes?: string
}

export function updateShopListItem(
  token: string,
  listId: number,
  itemId: number,
  payload: UpdateShopListItemPayload
) {
  return authed<ShopListItem>(token, `/shop/lists/${listId}/items/${itemId}`, {
    method: "PATCH",
    body: JSON.stringify(payload),
  })
}

export function suggestShopListItemNames(
  token: string,
  listId: number,
  term: string
) {
  return authed<ItemSuggestionsResponse>(
    token,
    `/shop/lists/${listId}/items/suggestion`,
    { method: "POST", body: JSON.stringify({ term }) }
  )
}

export function toggleShopListItem(
  token: string,
  listId: number,
  itemId: number
) {
  return authed<ShopListItem>(
    token,
    `/shop/lists/${listId}/items/${itemId}/toggle`,
    { method: "PATCH" }
  )
}

export function deleteShopListItem(
  token: string,
  listId: number,
  itemId: number
) {
  return authed<void>(token, `/shop/lists/${listId}/items/${itemId}`, {
    method: "DELETE",
  })
}

export function clearBoughtShopListItems(token: string, listId: number) {
  return authed<ShopListItem[]>(
    token,
    `/shop/lists/${listId}/items/clear-bought`,
    { method: "PATCH" }
  )
}

export function listRoutines(token: string) {
  return authed<RoutinesResponse>(token, "/routines")
}

export function createRoutine(token: string, payload: CreateRoutinePayload) {
  return authed<Routine>(token, "/routines", {
    method: "POST",
    body: JSON.stringify(payload),
  })
}

export function getRoutine(token: string, routineId: number) {
  return authed<Routine>(token, `/routines/${routineId}`)
}

export function updateRoutine(
  token: string,
  routineId: number,
  payload: UpdateRoutinePayload
) {
  return authed<Routine>(token, `/routines/${routineId}`, {
    method: "PATCH",
    body: JSON.stringify(payload),
  })
}

export function deleteRoutine(token: string, routineId: number) {
  return authed<void>(token, `/routines/${routineId}`, { method: "DELETE" })
}

export function getRoutineMonth(
  token: string,
  routineId: number,
  year: number,
  month: number
) {
  return authed<RoutineMonth>(
    token,
    `/routines/${routineId}/month?year=${year}&month=${month}`
  )
}

export function markRoutineDay(
  token: string,
  routineId: number,
  date: string,
  achieved: boolean
) {
  return authed<MarkRoutineDayResult>(
    token,
    `/routines/${routineId}/days/${date}`,
    { method: "PATCH", body: JSON.stringify({ achieved }) }
  )
}

export function unmarkRoutineDay(
  token: string,
  routineId: number,
  date: string
) {
  return authed<Routine>(token, `/routines/${routineId}/days/${date}`, {
    method: "DELETE",
  })
}

export type TaskRecurrenceType =
  "once" | "daily" | "weekly" | "monthly" | "annual"

export type TaskBusinessDayStrategy = "none" | "previous" | "next" | "closest"

export type TaskDayStatus =
  "completed" | "skipped" | "failed" | "pending" | "future"

export type TaskBoard = {
  id: number
  timezone: string
  today: string
  created_at: string
}

export type Task = {
  id: number
  name: string
  icon: string
  description: string
  instructions: string
  pinned: boolean
  recurrence_type: TaskRecurrenceType
  weekdays: number[]
  day_of_month: number
  /** 1 = January .. 12 = December, only set on annual tasks. */
  month_of_year: number
  business_day_strategy: TaskBusinessDayStrategy
  starts_on: string
  ends_on: string | null
  stopped: boolean
  stopped_at?: string
  origin_task_id: number | null
  created_at: string
}

export type TaskOccurrence = {
  task: Task
  day: string
  status: TaskDayStatus
  note?: string
}

export type TaskDayView = {
  day: string
  occurrences: TaskOccurrence[]
}

/** A task definition plus what the workspace overview derives from it. */
export type TaskSummary = Task & {
  /** First day from today onwards it fires on, null when it has none left. */
  next_occurrence: string | null
  /** Whether resuming it would produce a new run — see `listTasks`. */
  resumable: boolean
}

export type TaskListFilter = "active" | "stopped" | "all"

export type TaskListResponse = {
  tasks: TaskSummary[]
  /** Both counts cover the whole board, not the returned slice. */
  active_count: number
  stopped_count: number
}

export type CreateTaskPayload = {
  name: string
  icon: string
  description: string
  instructions: string
  pinned: boolean
  recurrence_type: TaskRecurrenceType
  weekdays?: number[]
  day_of_month?: number
  month_of_year?: number
  business_day_strategy?: TaskBusinessDayStrategy
  starts_on?: string
}

/** The API only accepts metadata here — recurrence is fixed once a task exists. */
export type UpdateTaskPayload = {
  name: string
  icon: string
  description: string
  instructions: string
  pinned: boolean
}

export function getTaskBoard(token: string) {
  return authed<TaskBoard>(token, "/tasks/board")
}

export function updateTaskBoard(token: string, timezone: string) {
  return authed<TaskBoard>(token, "/tasks/board", {
    method: "PATCH",
    body: JSON.stringify({ timezone }),
  })
}

export function getTaskDay(token: string, date?: string) {
  const query = date ? `?date=${date}` : ""
  return authed<TaskDayView>(token, `/tasks/day${query}`)
}

/**
 * Every task definition in the workspace, filtered by whether it is still
 * running. Only the newest stopped run of a chain comes back `resumable` — an
 * older one overlaps the task that already continued it, and the API refuses.
 */
export function listTasks(token: string, status: TaskListFilter = "active") {
  return authed<TaskListResponse>(token, `/tasks?status=${status}`)
}

export function createTask(token: string, payload: CreateTaskPayload) {
  return authed<Task>(token, "/tasks", {
    method: "POST",
    body: JSON.stringify(payload),
  })
}

export function updateTask(
  token: string,
  taskId: number,
  payload: UpdateTaskPayload
) {
  return authed<Task>(token, `/tasks/${taskId}`, {
    method: "PATCH",
    body: JSON.stringify(payload),
  })
}

/** Ends the recurrence today. Onetime tasks cannot be stopped. */
export function stopTask(token: string, taskId: number) {
  return authed<Task>(token, `/tasks/${taskId}/stop`, { method: "POST" })
}

/**
 * Continues a stopped recurrence as a new task starting on `from`, which has to
 * be after the day the stopped one ended.
 */
export function resumeTask(token: string, taskId: number, from?: string) {
  return authed<Task>(token, `/tasks/${taskId}/resume`, {
    method: "POST",
    body: JSON.stringify({ from: from ?? "" }),
  })
}

export function deleteTask(token: string, taskId: number) {
  return authed<void>(token, `/tasks/${taskId}`, { method: "DELETE" })
}

export function markTaskDay(
  token: string,
  taskId: number,
  date: string,
  status: "completed" | "skipped" | "failed"
) {
  return authed<TaskOccurrence>(token, `/tasks/${taskId}/days/${date}`, {
    method: "PATCH",
    body: JSON.stringify({ status, note: "" }),
  })
}

export function clearTaskDay(token: string, taskId: number, date: string) {
  return authed<TaskOccurrence>(token, `/tasks/${taskId}/days/${date}`, {
    method: "DELETE",
  })
}

export type RecipeCategory = {
  id: number
  name: string
  /** Counts non-archived recipes only, and is always 0 when embedded in a recipe. */
  recipe_count: number
  created_at: string
}

export type RecipeCategoriesResponse = {
  categories: RecipeCategory[]
}

export type RecipeIngredient = {
  id: number
  name: string
  quantity: string
  position: number
}

export type RecipeStep = {
  id: number
  title: string
  description: string
  position: number
}

/** The listing summary: no ingredients or steps, only how many there are. */
export type Recipe = {
  id: number
  name: string
  description: string
  estimated_minutes: number
  favorite: boolean
  archived: boolean
  archived_at?: string
  categories: RecipeCategory[]
  ingredient_count: number
  step_count: number
  created_at: string
  updated_at: string
}

export type RecipeDetail = Recipe & {
  ingredients: RecipeIngredient[]
  steps: RecipeStep[]
}

export type RecipePagination = {
  page: number
  per_page: number
  total: number
  total_pages: number
}

export type RecipeListResponse = {
  recipes: Recipe[]
  pagination: RecipePagination
}

export type RecipeArchivedFilter = "exclude" | "only" | "include"

export type RecipeListParams = {
  search?: string
  categoryIds?: number[]
  archived?: RecipeArchivedFilter
  favorite?: boolean
  page?: number
  perPage?: number
}

/**
 * The recipe as the API wants it written. Ingredients and steps carry no
 * position: the array order is the order, and the whole recipe is replaced on
 * every save.
 */
export type RecipePayload = {
  name: string
  description: string
  estimated_minutes: number
  favorite: boolean
  category_ids: number[]
  ingredients: { name: string; quantity: string }[]
  steps: { title: string; description: string }[]
}

export function listRecipeCategories(token: string) {
  return authed<RecipeCategoriesResponse>(token, "/recipes/categories")
}

export function createRecipeCategory(token: string, name: string) {
  return authed<RecipeCategory>(token, "/recipes/categories", {
    method: "POST",
    body: JSON.stringify({ name }),
  })
}

export function updateRecipeCategory(
  token: string,
  categoryId: number,
  name: string
) {
  return authed<RecipeCategory>(token, `/recipes/categories/${categoryId}`, {
    method: "PATCH",
    body: JSON.stringify({ name }),
  })
}

export function deleteRecipeCategory(token: string, categoryId: number) {
  return authed<void>(token, `/recipes/categories/${categoryId}`, {
    method: "DELETE",
  })
}

export function listRecipes(token: string, params: RecipeListParams = {}) {
  const query = new URLSearchParams()

  if (params.search?.trim()) {
    query.set("search", params.search.trim())
  }
  if (params.categoryIds?.length) {
    query.set("category_id", params.categoryIds.join(","))
  }
  if (params.archived) {
    query.set("archived", params.archived)
  }
  if (params.favorite) {
    query.set("favorite", "true")
  }
  if (params.page) {
    query.set("page", String(params.page))
  }
  if (params.perPage) {
    query.set("per_page", String(params.perPage))
  }

  const suffix = query.size > 0 ? `?${query.toString()}` : ""

  return authed<RecipeListResponse>(token, `/recipes${suffix}`)
}

export function createRecipe(token: string, payload: RecipePayload) {
  return authed<RecipeDetail>(token, "/recipes", {
    method: "POST",
    body: JSON.stringify(payload),
  })
}

export function getRecipe(token: string, recipeId: number) {
  return authed<RecipeDetail>(token, `/recipes/${recipeId}`)
}

/** Replaces the whole recipe: anything left out of the payload is removed. */
export function updateRecipe(
  token: string,
  recipeId: number,
  payload: RecipePayload
) {
  return authed<RecipeDetail>(token, `/recipes/${recipeId}`, {
    method: "PATCH",
    body: JSON.stringify(payload),
  })
}

export function setRecipeFavorite(
  token: string,
  recipeId: number,
  favorite: boolean
) {
  return authed<RecipeDetail>(token, `/recipes/${recipeId}/favorite`, {
    method: "PATCH",
    body: JSON.stringify({ favorite }),
  })
}

export function archiveRecipe(token: string, recipeId: number) {
  return authed<RecipeDetail>(token, `/recipes/${recipeId}/archive`, {
    method: "POST",
  })
}

export function unarchiveRecipe(token: string, recipeId: number) {
  return authed<RecipeDetail>(token, `/recipes/${recipeId}/unarchive`, {
    method: "POST",
  })
}

/** Permanent, and only accepted once the recipe is archived. */
export function deleteRecipe(token: string, recipeId: number) {
  return authed<void>(token, `/recipes/${recipeId}`, { method: "DELETE" })
}

/** Workspace-wide, unlike the shop suggestions which are scoped to one list. */
export function suggestRecipeIngredientNames(token: string, term: string) {
  return authed<ItemSuggestionsResponse>(
    token,
    "/recipes/ingredients/suggestion",
    { method: "POST", body: JSON.stringify({ term }) }
  )
}

export type BackupArchive = {
  name: string
  size: number
  created_at: string
}

export type BackupListResponse = {
  backups: BackupArchive[]
  /** How many archives are kept; older ones are pruned after a new backup. */
  keep: number
}

export type BackupDownload = {
  blob: Blob
  filename: string
}

/**
 * Archives come back as a byte stream and the route needs the bearer header, so
 * they cannot be fetched with a plain `<a download>` link.
 */
async function downloadArchive(
  path: string,
  token: string,
  fallbackName: string,
  method: "GET" | "POST" = "GET"
): Promise<BackupDownload> {
  const res = await fetch(`${API_URL}${path}`, {
    method,
    headers: { Authorization: `Bearer ${token}` },
  })

  if (!res.ok) {
    const body = await res.json().catch(() => null)
    throw new ApiError(res.status, body?.message ?? res.statusText)
  }

  return {
    blob: await res.blob(),
    filename:
      filenameFromDisposition(res.headers.get("Content-Disposition")) ??
      fallbackName,
  }
}

function filenameFromDisposition(header: string | null) {
  const match = header?.match(/filename=(?:"([^"]+)"|([^;]+))/i)
  return match ? (match[1] ?? match[2]).trim() : null
}

export function listBackups(token: string) {
  return authed<BackupListResponse>(token, "/backups")
}

/** Freezes the server, writes a new archive and streams it back in one request. */
export function generateBackup(token: string) {
  return downloadArchive("/backup", token, "cubby-backup.zip", "POST")
}

export function downloadBackup(token: string, name: string) {
  return downloadArchive(`/backups/${encodeURIComponent(name)}`, token, name)
}

export function deleteBackup(token: string, name: string) {
  return authed<void>(token, `/backups/${encodeURIComponent(name)}`, {
    method: "DELETE",
  })
}

/**
 * Puts an archive back in place. The server holds every request while it swaps
 * the database and the indexes, so this one takes longer than it looks and the
 * data on screen is stale once it resolves.
 */
export function restoreBackup(token: string, name: string) {
  return authed<void>(token, `/backups/${encodeURIComponent(name)}/restore`, {
    method: "POST",
  })
}

/**
 * What identifies the certificate the server was configured with. The private
 * key never leaves the server.
 */
export type TLSCertificate = {
  subject: string
  issuer: string
  dns_names: string[]
  ip_addresses: string[]
  not_before: string
  not_after: string
  /** SHA-256 of the certificate, the same one a browser shows. */
  fingerprint: string
  chain_length: number
  self_signed: boolean
  /** Answered against the server's clock, not the browser's. */
  expired: boolean
  /** When the certificate file was last written, as the server found it. */
  loaded_at: string
}

export type TLSSettings = {
  enabled: boolean
  certificate?: TLSCertificate
}

/**
 * Reports the certificate in use. It is read only: TLS is set up in the
 * server's configuration, so it can only be changed there.
 */
export function getTLSSettings(token: string) {
  return authed<TLSSettings>(token, "/settings/tls")
}

/** One execution of the expired sessions cleanup job. */
export type SessionCleanupRun = {
  started_at: string
  duration_ms: number
  /** How many sessions were erased, always zero unless the run succeeded. */
  removed: number
  status: "ok" | "skipped" | "failed"
  /** Why a skipped or a failed run cleaned nothing. */
  error?: string
}

export type SessionCleanupReport = {
  job: string
  interval_seconds: number
  /** Absent while the scheduler is not running. */
  next_run?: string
  /** The latest runs, newest first. */
  runs: SessionCleanupRun[]
}

/** Reports how the expired sessions cleanup has been doing lately. */
export function getSessionCleanupReport(token: string) {
  return authed<SessionCleanupReport>(token, "/cron/sessions-cleanup")
}

/**
 * Whether this server still has to be set up. A server nobody has set up has no
 * users at all, so this is the one thing it can be asked without a token.
 */
export type SetupStatus = {
  required: boolean
}

export function getSetupStatus() {
  return request<SetupStatus>("/setup/status")
}

export type CompleteSetupPayload = {
  /** The token the server wrote to its log on first boot. */
  token: string
  name: string
  username: string
  password: string
}

/** Creates the first system administrator. The created user is always one. */
export function completeSetup(payload: CompleteSetupPayload) {
  return request<void>("/setup", {
    method: "POST",
    body: JSON.stringify(payload),
  })
}

export type UserListResponse = {
  users: User[]
}

export function listUsers(token: string) {
  return authed<UserListResponse>(token, "/users")
}

export function createUser(
  token: string,
  payload: {
    name: string
    username: string
    password: string
    is_system_admin: boolean
  }
) {
  return authed<User>(token, "/users", {
    method: "POST",
    body: JSON.stringify(payload),
  })
}

export function updateUser(
  token: string,
  userId: number,
  payload: { name: string; is_system_admin: boolean }
) {
  return authed<User>(token, `/users/${userId}`, {
    method: "PATCH",
    body: JSON.stringify(payload),
  })
}

export function changeUserPassword(
  token: string,
  userId: number,
  password: string
) {
  return authed<void>(token, `/users/${userId}/password`, {
    method: "PUT",
    body: JSON.stringify({ password }),
  })
}

/**
 * Refused with a 409 when the user created cubbies, whose names come back in
 * `details.workspaces` so the caller can say what is in the way.
 */
export function deleteUser(token: string, userId: number) {
  return authed<void>(token, `/users/${userId}`, { method: "DELETE" })
}
