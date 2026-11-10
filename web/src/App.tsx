import { useEffect, useState, type FormEvent } from 'react'
import { api, setUnauthorizedHandler } from './api'
import { Field, ErrorBanner, Shuttle, useAction } from './ui'
import { SessionPage } from './pages/SessionPage'
import { PlayersPage } from './pages/PlayersPage'
import { MoneyPage } from './pages/MoneyPage'
import { ReportPage } from './pages/ReportPage'
import { SettingsPage } from './pages/SettingsPage'

type Tab = 'session' | 'players' | 'money' | 'report' | 'settings'

const TABS: [Tab, string][] = [
  ['session', 'Buổi chơi'],
  ['players', 'Người chơi'],
  ['money', 'Thu chi'],
  ['report', 'Báo cáo'],
  ['settings', 'Cài đặt'],
]

export function App() {
  const [auth, setAuth] = useState<'checking' | 'in' | 'out'>('checking')
  const [tab, setTab] = useState<Tab>('session')

  useEffect(() => {
    setUnauthorizedHandler(() => setAuth('out'))
    api
      .me()
      .then((r) => setAuth(r.authenticated ? 'in' : 'out'))
      .catch(() => setAuth('out'))
  }, [])

  if (auth === 'checking') return <div className="splash">Đang tải…</div>
  if (auth === 'out') return <Login onDone={() => setAuth('in')} />

  return (
    <div className="app">
      <main>
        {tab === 'session' && <SessionPage />}
        {tab === 'players' && <PlayersPage />}
        {tab === 'money' && <MoneyPage />}
        {tab === 'report' && <ReportPage />}
        {tab === 'settings' && <SettingsPage onLogout={() => setAuth('out')} />}
      </main>
      <nav className="tabs" aria-label="Điều hướng">
        {TABS.map(([id, label]) => (
          <button key={id} className={tab === id ? 'on' : ''} aria-current={tab === id ? 'page' : undefined} onClick={() => setTab(id)}>
            <TabIcon tab={id} />
            {label}
          </button>
        ))}
      </nav>
    </div>
  )
}

function TabIcon({ tab }: { tab: Tab }) {
  const paths: Record<Tab, string> = {
    session: 'M4 5h16v14H4zM12 5v14M4 12h16',
    players: 'M9 11a3.5 3.5 0 1 0 0-7 3.5 3.5 0 0 0 0 7zM2.5 20c.6-3.4 3.2-5.5 6.5-5.5s5.9 2.1 6.5 5.5M16 4.3a3.5 3.5 0 0 1 0 6.4M18 14.8c1.9.8 3.1 2.6 3.5 5.2',
    money: 'M3 7h18v11H3zM3 11h18M7 15h4',
    report: 'M5 20V10M12 20V4M19 20v-7',
    settings: 'M4 7h10M18 7h2M4 17h2M10 17h10M16 4.5v5M8 14.5v5',
  }
  return (
    <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d={paths[tab]} />
    </svg>
  )
}

function Login({ onDone }: { onDone: () => void }) {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const { busy, error, setError, run } = useAction()

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    if ((await run(() => api.login(username, password))) !== undefined) onDone()
  }

  return (
    <div className="login">
      <div className="login-banner">
        <Shuttle size={72} />
        <h1>Xếp sân cầu lông</h1>
        <p>Ghép trận công bằng, theo dõi sân và thu chi ngay tại sân.</p>
      </div>
      <form className="login-form" onSubmit={submit}>
        <ErrorBanner message={error} onClose={() => setError(null)} />
        <Field label="Tên đăng nhập">
          <input value={username} onChange={(e) => setUsername(e.target.value)} autoComplete="username" autoCapitalize="none" required />
        </Field>
        <Field label="Mật khẩu">
          <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} autoComplete="current-password" required />
        </Field>
        <button className="btn btn-primary" disabled={busy}>
          {busy ? 'Đang đăng nhập…' : 'Đăng nhập'}
        </button>
      </form>
    </div>
  )
}
