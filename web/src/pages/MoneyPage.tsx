import { useCallback, useEffect, useState, type FormEvent } from 'react'
import { api, type Player, type SessionState, type Transaction, type TransactionType } from '../api'
import { categoryLabel, dateVN, EXPENSE_CATEGORIES, INCOME_CATEGORIES, money, monthVN, parseMoney, signed, todayVN } from '../format'
import { ErrorBanner, Field, MoneyInput, Segmented, Sheet, useAction } from '../ui'

export function MoneyPage() {
  const [month, setMonth] = useState(todayVN().slice(0, 7))
  const [txs, setTxs] = useState<Transaction[] | null>(null)
  const [regulars, setRegulars] = useState<Player[]>([])
  const [adding, setAdding] = useState(false)
  const [collecting, setCollecting] = useState<Player | null>(null)
  const [deleting, setDeleting] = useState<number | null>(null)
  const { busy, error, setError, run } = useAction()

  const load = useCallback(async () => {
    const [list, players] = await Promise.all([run(() => api.transactions(month)), run(() => api.players())])
    if (list) setTxs(list)
    if (players) setRegulars(players.filter((p) => p.is_regular))
  }, [month, run])

  useEffect(() => {
    setTxs(null)
    load()
  }, [load])

  const paidBy = new Map<number, number>()
  for (const t of txs ?? []) {
    if (t.category === 'monthly_fee' && t.player_id !== null) paidBy.set(t.player_id, (paidBy.get(t.player_id) ?? 0) + t.amount)
  }
  // A month's default date: today when it is the current month, else its first day.
  const defaultDate = todayVN().startsWith(month) ? todayVN() : `${month}-01`

  return (
    <>
      <header className="banner banner-plain">
        <div className="banner-main">
          <h1>Thu chi</h1>
          <p>{monthVN(month)}</p>
        </div>
        <button className="btn-on-dark" onClick={() => setAdding(true)}>
          Thêm thu chi
        </button>
      </header>
      <ErrorBanner message={error} onClose={() => setError(null)} />
      <div className="pad stack">
        <Field label="Tháng">
          <input type="month" value={month} onChange={(e) => e.target.value && setMonth(e.target.value)} />
        </Field>

        {regulars.length > 0 && (
          <section>
            <div className="section-head">
              <h2>Phí tháng người cố định</h2>
            </div>
            <ul className="fee-list">
              {regulars.map((p) => {
                const paid = paidBy.get(p.id)
                return (
                  <li key={p.id}>
                    <div className="fee-who">
                      <span className="queue-name">{p.name}</span>
                      <span className="fee-amount">{paid === undefined ? 'Chưa ghi nhận tháng này' : `Đã đóng ${money(paid)}`}</span>
                    </div>
                    <button className="paid" onClick={() => setCollecting(p)}>
                      Thu phí tháng
                    </button>
                  </li>
                )
              })}
            </ul>
          </section>
        )}

        <section>
          <div className="section-head">
            <h2>Giao dịch trong tháng</h2>
          </div>
          {txs === null ? (
            <p className="empty">Đang tải…</p>
          ) : txs.length === 0 ? (
            <p className="empty">Chưa có khoản thu chi nào trong {monthVN(month).toLowerCase()}.</p>
          ) : (
            <ul className="tx-list">
              {txs.map((t) => (
                <li key={t.id}>
                  <div className="tx-main">
                    <span className="tx-title">
                      {categoryLabel(t.category)}
                      {t.player_name && `: ${t.player_name}`}
                    </span>
                    <span className="tx-sub">
                      {dateVN(t.date)}
                      {t.session_id !== null && ', theo buổi'}
                      {t.note && `, ${t.note}`}
                    </span>
                  </div>
                  <span className={t.type === 'income' ? 'tx-amount pos' : 'tx-amount neg'}>{signed(t.type, t.amount)}</span>
                  {deleting === t.id ? (
                    <button
                      className="btn-text neg"
                      disabled={busy}
                      onClick={async () => {
                        if ((await run(() => api.deleteTransaction(t.id))) !== undefined) {
                          setDeleting(null)
                          load()
                        }
                      }}
                    >
                      Xác nhận xóa
                    </button>
                  ) : (
                    <button className="btn-text" onClick={() => setDeleting(t.id)} aria-label={`Xóa khoản ${categoryLabel(t.category)}`}>
                      Xóa
                    </button>
                  )}
                </li>
              ))}
            </ul>
          )}
        </section>
      </div>

      {adding && (
        <TransactionSheet
          defaultDate={defaultDate}
          onClose={() => setAdding(false)}
          onSaved={() => {
            setAdding(false)
            load()
          }}
        />
      )}
      {collecting && (
        <MonthlyFeeSheet
          player={collecting}
          month={month}
          defaultDate={defaultDate}
          onClose={() => setCollecting(null)}
          onSaved={() => {
            setCollecting(null)
            load()
          }}
        />
      )}
    </>
  )
}

function TransactionSheet({ defaultDate, onClose, onSaved }: { defaultDate: string; onClose: () => void; onSaved: () => void }) {
  const [type, setType] = useState<TransactionType>('expense')
  const [category, setCategory] = useState('shuttles')
  const [amount, setAmount] = useState('')
  const [date, setDate] = useState(defaultDate)
  const [note, setNote] = useState('')
  const [open, setOpen] = useState<SessionState | null>(null)
  const [attach, setAttach] = useState(false)
  const { busy, error, setError, run } = useAction()

  useEffect(() => {
    api
      .currentSession()
      .then(setOpen)
      .catch(() => setOpen(null))
  }, [])

  // Session and monthly fees have their own flows; exclude them here.
  const categories = (type === 'income' ? INCOME_CATEGORIES : EXPENSE_CATEGORIES).filter(([id]) => id !== 'session_fee' && id !== 'monthly_fee')

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    const value = parseMoney(amount)
    if (Number.isNaN(value)) {
      setError('Nhập số tiền trước khi lưu.')
      return
    }
    const saved = await run(() =>
      api.createTransaction({ type, category, amount: value, date, note, session_id: attach && open ? open.session.id : null, player_id: null }),
    )
    if (saved) onSaved()
  }

  return (
    <Sheet title="Thêm thu chi" onClose={onClose}>
      <form className="stack" onSubmit={submit}>
        <ErrorBanner message={error} onClose={() => setError(null)} />
        <Segmented
          value={type}
          onChange={(t) => {
            setType(t)
            setCategory(t === 'income' ? 'shuttle_sale' : 'shuttles')
          }}
          options={[
            ['expense', 'Chi'],
            ['income', 'Thu'],
          ]}
        />
        <Field label="Danh mục">
          <select value={category} onChange={(e) => setCategory(e.target.value)}>
            {categories.map(([id, label]) => (
              <option key={id} value={id}>
                {label}
              </option>
            ))}
          </select>
        </Field>
        <Field label="Số tiền (đ)">
          <MoneyInput value={amount} onChange={setAmount} />
        </Field>
        <Field label="Ngày">
          <input type="date" value={date} onChange={(e) => setDate(e.target.value)} required />
        </Field>
        <Field label="Ghi chú">
          <input value={note} onChange={(e) => setNote(e.target.value)} />
        </Field>
        {open && (
          <label className="check">
            <input type="checkbox" checked={attach} onChange={(e) => setAttach(e.target.checked)} />
            Tính vào buổi đang mở ({dateVN(open.session.date)})
          </label>
        )}
        <button className="btn btn-primary" disabled={busy}>
          {busy ? 'Đang lưu…' : 'Lưu khoản này'}
        </button>
      </form>
    </Sheet>
  )
}

function MonthlyFeeSheet({
  player,
  month,
  defaultDate,
  onClose,
  onSaved,
}: {
  player: Player
  month: string
  defaultDate: string
  onClose: () => void
  onSaved: () => void
}) {
  const [amount, setAmount] = useState(new Intl.NumberFormat('vi-VN').format(player.monthly_fee))
  const [date, setDate] = useState(defaultDate)
  const { busy, error, setError, run } = useAction()

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    const value = parseMoney(amount)
    if (Number.isNaN(value)) {
      setError('Nhập số tiền trước khi lưu.')
      return
    }
    const saved = await run(() =>
      api.createTransaction({ type: 'income', category: 'monthly_fee', amount: value, date, note: `Phí ${monthVN(month).toLowerCase()}`, player_id: player.id, session_id: null }),
    )
    if (saved) onSaved()
  }

  return (
    <Sheet title={`Phí tháng của ${player.name}`} onClose={onClose}>
      <form className="stack" onSubmit={submit}>
        <ErrorBanner message={error} onClose={() => setError(null)} />
        <Field label="Số tiền đã nhận (đ)">
          <MoneyInput value={amount} onChange={setAmount} />
        </Field>
        <Field label="Ngày nhận">
          <input type="date" value={date} onChange={(e) => setDate(e.target.value)} required />
        </Field>
        <p className="hint">Khoản này tính vào tháng của ngày nhận.</p>
        <button className="btn btn-primary" disabled={busy}>
          {busy ? 'Đang lưu…' : 'Ghi nhận phí tháng'}
        </button>
      </form>
    </Sheet>
  )
}
