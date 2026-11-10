// Typed client for the Go API. The server is the source of truth; nothing
// about a session is kept in localStorage.

export type Gender = 'M' | 'F'
export type Level = 'Y' | 'Y+' | 'TBY' | 'TB-' | 'TB' | 'TB+'
export type PlayerStatus = 'WAITING' | 'PLAYING' | 'RESTING'

export interface Player {
  id: number
  name: string
  gender: Gender
  level: Level
  is_regular: boolean
  monthly_fee: number
  note: string
}

export interface Settings {
  fee_male: number
  fee_female: number
  court_fee: number
  gender_gap_threshold: number
  balance_threshold: number
  repeat_lookback: number
}

export interface Session {
  id: number
  date: string
  start_time: string | null
  end_time: string | null
  status: 'OPEN' | 'ENDED'
  court_count: number
  fee_male: number
  fee_female: number
  court_fee: number
}

export interface Match {
  id: number
  status: 'DRAFT' | 'ONGOING' | 'FINISHED'
  started_at: string | null
  team_a: number[]
  team_b: number[]
  diff: number
  balanced: boolean
}

export interface Court {
  id: number
  index: number
  status: 'EMPTY' | 'IN_USE'
  match: Match | null
}

export interface SessionPlayer {
  id: number
  player_id: number
  name: string
  gender: Gender
  level: Level
  is_regular: boolean
  status: PlayerStatus
  matches_played: number
  consecutive_rest: number
  fee: number
  fee_override: number | null
  is_paid: boolean
  held: boolean
  queue_position: number
}

export interface Fees {
  expected: number
  collected: number
  outstanding: number
}

export interface SessionState {
  session: Session
  courts: Court[]
  players: SessionPlayer[]
  fees: Fees
}

export type TransactionType = 'income' | 'expense'

export interface Transaction {
  id: number
  session_id: number | null
  player_id: number | null
  player_name: string
  type: TransactionType
  category: string
  amount: number
  note: string
  date: string
  auto: boolean
}

export interface Totals {
  income: number
  expense: number
  profit: number
}

export interface MonthSummary extends Totals {
  month: string
  session_count: number
}

export interface Summary extends Totals {
  months: MonthSummary[]
}

export interface SessionRow extends Totals {
  session: Session
  player_count: number
  male: number
  female: number
  casual: number
  regular: number
  fees: Fees
  fee_income: number
  other_income: number
}

export interface MonthReport extends Totals {
  month: string
  session_count: number
  avg_income: number
  avg_profit: number
  categories: { category: string; type: TransactionType; amount: number }[]
  regular_income: number
  regular_payments: { player_id: number; name: string; amount: number }[]
  casual_income: number
  other_income: number
  sessions: SessionRow[]
  unlinked: Transaction[]
  unlinked_totals: Totals
}

export interface SessionReport extends SessionState {
  transactions: Transaction[]
}

export class ApiError extends Error {
  constructor(
    public status: number,
    public code: string,
    message: string,
  ) {
    super(message)
  }
}

let onUnauthorized: () => void = () => {}

/** Registers what happens when the server says the login expired. */
export function setUnauthorizedHandler(fn: () => void) {
  onUnauthorized = fn
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  let res: Response
  try {
    res = await fetch(path, {
      method,
      headers: body === undefined ? undefined : { 'Content-Type': 'application/json' },
      body: body === undefined ? undefined : JSON.stringify(body),
      credentials: 'same-origin',
    })
  } catch {
    throw new ApiError(0, 'network', 'Không kết nối được máy chủ. Kiểm tra mạng rồi thử lại.')
  }
  let data: unknown = null
  try {
    data = await res.json()
  } catch {
    // Non-JSON body: handled by the status check below.
  }
  if (!res.ok) {
    const err = (data as { error?: { code: string; message: string } } | null)?.error
    if (res.status === 401 && err?.code === 'unauthorized') onUnauthorized()
    throw new ApiError(res.status, err?.code ?? 'unknown', err?.message ?? 'Có lỗi xảy ra. Vui lòng thử lại.')
  }
  return data as T
}

const get = <T>(path: string) => request<T>('GET', path)
const post = <T>(path: string, body: unknown = {}) => request<T>('POST', path, body)
const put = <T>(path: string, body: unknown) => request<T>('PUT', path, body)
const del = <T>(path: string) => request<T>('DELETE', path)

export const api = {
  me: () => get<{ authenticated: boolean }>('/api/me'),
  login: (username: string, password: string) => post<unknown>('/api/login', { username, password }),
  logout: () => post<unknown>('/api/logout'),

  settings: () => get<Settings>('/api/settings'),
  saveSettings: (s: Settings) => put<Settings>('/api/settings', s),

  players: () => get<Player[]>('/api/players'),
  createPlayer: (p: Omit<Player, 'id'>) => post<Player>('/api/players', p),
  updatePlayer: (p: Player) => put<Player>(`/api/players/${p.id}`, p),
  deletePlayer: (id: number) => del<unknown>(`/api/players/${id}`),

  currentSession: () => get<SessionState | null>('/api/session/current'),
  createSession: (body: { court_count: number; fee_male: number; fee_female: number; court_fee: number }) =>
    post<SessionState>('/api/sessions', body),
  endSession: (id: number) => post<SessionState>(`/api/sessions/${id}/end`),
  setCourtCount: (id: number, court_count: number) => put<SessionState>(`/api/sessions/${id}/courts`, { court_count }),
  checkIn: (sessionId: number, player_id: number) => post<SessionState>(`/api/sessions/${sessionId}/players`, { player_id }),
  removeFromSession: (id: number) => del<SessionState>(`/api/session-players/${id}`),
  setStatus: (id: number, status: PlayerStatus) => put<SessionState>(`/api/session-players/${id}/status`, { status }),
  setPayment: (id: number, body: { is_paid?: boolean; fee_override?: number | null }) =>
    put<SessionState>(`/api/session-players/${id}/payment`, body),

  propose: (courtId: number) => post<SessionState>(`/api/courts/${courtId}/propose`),
  swap: (matchId: number, out_id: number, in_id: number) => put<SessionState>(`/api/matches/${matchId}/swap`, { out_id, in_id }),
  confirm: (matchId: number, force: boolean) => post<SessionState>(`/api/matches/${matchId}/confirm`, { force }),
  dismiss: (matchId: number) => del<SessionState>(`/api/matches/${matchId}`),
  finish: (matchId: number) => post<SessionState>(`/api/matches/${matchId}/finish`),

  transactions: (month: string) => get<Transaction[]>(`/api/transactions?month=${month}`),
  createTransaction: (t: Partial<Transaction>) => post<Transaction>('/api/transactions', t),
  deleteTransaction: (id: number) => del<unknown>(`/api/transactions/${id}`),

  summary: () => get<Summary>('/api/reports/summary'),
  monthReport: (month: string) => get<MonthReport>(`/api/reports/months/${month}`),
  sessionReport: (id: number) => get<SessionReport>(`/api/reports/sessions/${id}`),
}
