import { useCallback, useEffect, useRef, useState, type FormEvent } from 'react'
import { api, type Court, type Gender, type Level, type Player, type SessionPlayer, type SessionState } from '../api'
import { dateVN, LEVELS, money, parseMoney, timeVN } from '../format'
import { ErrorBanner, Field, GenderMark, LevelTag, MoneyInput, Segmented, Sheet, Shuttle, useAction } from '../ui'

const POLL_MS = 3000

export function SessionPage() {
  // undefined: first load; null: no open session.
  const [state, setState] = useState<SessionState | null | undefined>(undefined)
  const [view, setView] = useState<'courts' | 'fees'>('courts')
  const { busy, error, setError, run } = useAction()
  // Bumped on every change so a poll that started earlier cannot overwrite
  // newer state.
  const version = useRef(0)

  const refresh = useCallback(async () => {
    const started = version.current
    try {
      const next = await api.currentSession()
      if (started === version.current) setState(next)
    } catch {
      // Polling failures are silent; the next tick retries.
    }
  }, [])

  useEffect(() => {
    refresh()
    const timer = setInterval(refresh, POLL_MS)
    return () => clearInterval(timer)
  }, [refresh])

  const act = useCallback(
    async (fn: () => Promise<SessionState>) => {
      version.current++
      const next = await run(fn)
      version.current++
      if (next) setState(next.session.status === 'OPEN' ? next : null)
      return next
    },
    [run],
  )

  if (state === undefined) return <div className="splash">Đang tải buổi chơi…</div>
  if (state === null) {
    return (
      <>
        <ErrorBanner message={error} onClose={() => setError(null)} />
        <NewSession busy={busy} onCreate={(body) => act(() => api.createSession(body))} />
      </>
    )
  }

  return (
    <>
      <SessionBanner state={state} busy={busy} act={act} />
      <ErrorBanner message={error} onClose={() => setError(null)} />
      <div className="pad">
        <Segmented
          value={view}
          onChange={setView}
          options={[
            ['courts', 'Sân và hàng chờ'],
            ['fees', 'Thu phí'],
          ]}
        />
      </div>
      {view === 'courts' ? <CourtsView state={state} busy={busy} act={act} /> : <FeesView state={state} busy={busy} act={act} />}
    </>
  )
}

type Act = (fn: () => Promise<SessionState>) => Promise<SessionState | undefined>
interface ViewProps {
  state: SessionState
  busy: boolean
  act: Act
}

function NewSession({ busy, onCreate }: { busy: boolean; onCreate: (b: { court_count: number; fee_male: number; fee_female: number; court_fee: number }) => void }) {
  const [courts, setCourts] = useState(1)
  const [feeMale, setFeeMale] = useState('')
  const [feeFemale, setFeeFemale] = useState('')
  const [courtFee, setCourtFee] = useState('')
  const [formError, setFormError] = useState<string | null>(null)

  useEffect(() => {
    const fmt = new Intl.NumberFormat('vi-VN')
    api
      .settings()
      .then((s) => {
        setFeeMale(fmt.format(s.fee_male))
        setFeeFemale(fmt.format(s.fee_female))
        setCourtFee(fmt.format(s.court_fee))
      })
      .catch(() => setFormError('Không tải được phí mặc định. Nhập phí bằng tay hoặc thử lại.'))
  }, [])

  const submit = (e: FormEvent) => {
    e.preventDefault()
    const body = { court_count: courts, fee_male: parseMoney(feeMale), fee_female: parseMoney(feeFemale), court_fee: parseMoney(courtFee || '0') }
    if ([body.fee_male, body.fee_female, body.court_fee].some(Number.isNaN)) {
      setFormError('Nhập đủ phí nam, phí nữ trước khi mở buổi.')
      return
    }
    setFormError(null)
    onCreate(body)
  }

  return (
    <>
      <header className="banner">
        <Shuttle size={44} />
        <div>
          <h1>Chưa có buổi chơi</h1>
          <p>Mở buổi mới để check-in người chơi và bắt đầu xếp trận.</p>
        </div>
      </header>
      <form className="pad stack" onSubmit={submit}>
        <ErrorBanner message={formError} onClose={() => setFormError(null)} />
        <div className="field">
          <span>Số sân</span>
          <Segmented
            value={courts}
            onChange={setCourts}
            options={[
              [1, '1 sân'],
              [2, '2 sân'],
              [3, '3 sân'],
            ]}
          />
        </div>
        <div className="row-2">
          <Field label="Phí nam (đ)">
            <MoneyInput value={feeMale} onChange={setFeeMale} />
          </Field>
          <Field label="Phí nữ (đ)">
            <MoneyInput value={feeFemale} onChange={setFeeFemale} />
          </Field>
        </div>
        <Field label="Tiền thuê mỗi sân (đ)">
          <MoneyInput value={courtFee} onChange={setCourtFee} />
        </Field>
        <button className="btn btn-primary" disabled={busy}>
          {busy ? 'Đang mở buổi…' : 'Mở buổi chơi'}
        </button>
      </form>
    </>
  )
}

function SessionBanner({ state, busy, act }: ViewProps) {
  const [ending, setEnding] = useState(false)
  const { session, players, fees } = state
  const playing = state.courts.filter((c) => c.status === 'IN_USE').length

  return (
    <header className="banner">
      <Shuttle size={44} />
      <div className="banner-main">
        <h1>Buổi {dateVN(session.date)}</h1>
        <p>
          {players.length} người, bắt đầu {timeVN(session.start_time)}
        </p>
      </div>
      <button className="btn-on-dark" onClick={() => setEnding(true)}>
        Kết thúc buổi
      </button>
      {ending && (
        <Sheet title="Kết thúc buổi chơi?" onClose={() => setEnding(false)}>
          <ul className="facts">
            <li>
              <span>Đã thu</span>
              <b>{money(fees.collected)}</b>
            </li>
            <li>
              <span>Chưa thu</span>
              <b className={fees.outstanding > 0 ? 'neg' : ''}>{money(fees.outstanding)}</b>
            </li>
            <li>
              <span>Tiền thuê sân</span>
              <b>{money(session.court_fee * session.court_count)}</b>
            </li>
          </ul>
          {playing > 0 && <p className="hint">{playing} trận đang đánh sẽ được đóng và vẫn tính cho người chơi.</p>}
          <p className="hint">Sau khi kết thúc chỉ còn sửa được phần thu chi của buổi (trong Báo cáo).</p>
          <button
            className="btn btn-danger"
            disabled={busy}
            onClick={async () => {
              await act(() => api.endSession(session.id))
              setEnding(false)
            }}
          >
            {busy ? 'Đang kết thúc…' : 'Kết thúc buổi'}
          </button>
        </Sheet>
      )}
    </header>
  )
}

function CourtsView({ state, busy, act }: ViewProps) {
  const [adding, setAdding] = useState(false)
  const [selected, setSelected] = useState<number | null>(null)
  const [swap, setSwap] = useState<{ matchId: number; outId: number } | null>(null)
  const { session, courts, players } = state
  const byId = new Map(players.map((p) => [p.id, p]))
  const waiting = players.filter((p) => p.status === 'WAITING').sort((a, b) => a.queue_position - b.queue_position)
  const resting = players.filter((p) => p.status === 'RESTING')
  const selectedPlayer = selected === null ? undefined : byId.get(selected)
  const swapOut = swap ? byId.get(swap.outId) : undefined

  return (
    <div className="pad stack">
      <div className="court-count">
        <span>Số sân mở</span>
        <Segmented
          value={session.court_count}
          onChange={(n) => n !== session.court_count && act(() => api.setCourtCount(session.id, n))}
          options={[
            [1, '1'],
            [2, '2'],
            [3, '3'],
          ]}
        />
      </div>

      {courts.map((court) => (
        <CourtCard key={court.id} court={court} byId={byId} busy={busy} act={act} onSwap={(matchId, outId) => setSwap({ matchId, outId })} />
      ))}

      <section>
        <div className="section-head">
          <h2>Hàng chờ ({waiting.length})</h2>
          <button className="btn btn-small" onClick={() => setAdding(true)}>
            Thêm người
          </button>
        </div>
        {waiting.length === 0 ? (
          <p className="empty">Chưa ai trong hàng chờ. Bấm “Thêm người” để check-in.</p>
        ) : (
          <ol className="queue">
            {waiting.map((p) => (
              <li key={p.id}>
                <button className="queue-row" onClick={() => setSelected(p.id)}>
                  <span className="queue-pos">{p.queue_position}</span>
                  <span className="queue-name">
                    {p.name}
                    {p.held && <span className="tag tag-held">Đã xếp</span>}
                  </span>
                  <GenderMark gender={p.gender} />
                  <LevelTag level={p.level} />
                  <span className="queue-count">{p.matches_played} trận</span>
                </button>
              </li>
            ))}
          </ol>
        )}
      </section>

      {resting.length > 0 && (
        <section>
          <div className="section-head">
            <h2>Đang nghỉ ({resting.length})</h2>
          </div>
          <ul className="queue">
            {resting.map((p) => (
              <li key={p.id}>
                <button className="queue-row queue-row-rest" onClick={() => setSelected(p.id)}>
                  <span className="queue-name">{p.name}</span>
                  <GenderMark gender={p.gender} />
                  <LevelTag level={p.level} />
                  <span className="queue-count">{p.matches_played} trận</span>
                </button>
              </li>
            ))}
          </ul>
        </section>
      )}

      {adding && <AddPlayersSheet state={state} busy={busy} act={act} onClose={() => setAdding(false)} />}

      {selectedPlayer && (
        <Sheet title={selectedPlayer.name} onClose={() => setSelected(null)}>
          <p className="hint">
            {selectedPlayer.gender === 'M' ? 'Nam' : 'Nữ'}, level {selectedPlayer.level}, {selectedPlayer.is_regular ? 'cố định' : 'vãng lai'}. Đã đánh{' '}
            {selectedPlayer.matches_played} trận.
          </p>
          {selectedPlayer.status === 'WAITING' && (
            <button
              className="btn"
              disabled={busy}
              onClick={async () => {
                await act(() => api.setStatus(selectedPlayer.id, 'RESTING'))
                setSelected(null)
              }}
            >
              Cho nghỉ{selectedPlayer.held ? ' (bỏ đề xuất đang có)' : ''}
            </button>
          )}
          {selectedPlayer.status === 'RESTING' && (
            <button
              className="btn btn-primary"
              disabled={busy}
              onClick={async () => {
                await act(() => api.setStatus(selectedPlayer.id, 'WAITING'))
                setSelected(null)
              }}
            >
              Đưa vào hàng chờ
            </button>
          )}
          {selectedPlayer.matches_played === 0 && !selectedPlayer.held && (
            <button
              className="btn btn-danger-quiet"
              disabled={busy}
              onClick={async () => {
                await act(() => api.removeFromSession(selectedPlayer.id))
                setSelected(null)
              }}
            >
              Xóa khỏi buổi
            </button>
          )}
        </Sheet>
      )}

      {swap && swapOut && (
        <Sheet title={`Đổi ${swapOut.name} lấy ai?`} onClose={() => setSwap(null)}>
          {waiting.filter((p) => !p.held).length === 0 ? (
            <p className="empty">Không còn ai rảnh trong hàng chờ để đổi.</p>
          ) : (
            <ul className="queue">
              {waiting
                .filter((p) => !p.held)
                .map((p) => (
                  <li key={p.id}>
                    <button
                      className="queue-row"
                      disabled={busy}
                      onClick={async () => {
                        await act(() => api.swap(swap.matchId, swap.outId, p.id))
                        setSwap(null)
                      }}
                    >
                      <span className="queue-name">{p.name}</span>
                      <GenderMark gender={p.gender} />
                      <LevelTag level={p.level} />
                      <span className="queue-count">{p.matches_played} trận</span>
                    </button>
                  </li>
                ))}
            </ul>
          )}
        </Sheet>
      )}
    </div>
  )
}

function CourtCard({
  court,
  byId,
  busy,
  act,
  onSwap,
}: {
  court: Court
  byId: Map<number, SessionPlayer>
  busy: boolean
  act: Act
  onSwap: (matchId: number, outId: number) => void
}) {
  const match = court.match
  const draft = match?.status === 'DRAFT'
  const playing = court.status === 'IN_USE'
  const label = playing ? `Đang đánh từ ${timeVN(match?.started_at ?? null)}` : draft ? 'Đề xuất chờ duyệt' : 'Trống'

  const team = (ids: number[]) =>
    ids.map((id) => {
      const p = byId.get(id)
      if (!p) return null
      const body = (
        <>
          <span className="chip-name">{p.name}</span>
          <span className="chip-meta">
            {p.gender === 'M' ? 'Nam' : 'Nữ'} {p.level}
          </span>
        </>
      )
      return draft && match ? (
        <button key={id} className="chip chip-swap" disabled={busy} onClick={() => onSwap(match.id, id)} aria-label={`Đổi ${p.name}`}>
          {body}
        </button>
      ) : (
        <div key={id} className="chip">
          {body}
        </div>
      )
    })

  return (
    <section className="court-card">
      <div className="court-head">
        <h2>Sân {court.index}</h2>
        <span className={playing ? 'state state-play' : draft ? 'state state-draft' : 'state state-empty'}>{label}</span>
      </div>
      <div className={playing ? 'court court-play' : draft ? 'court court-draft' : 'court'}>
        <div className="court-half">{match && team(match.team_a)}</div>
        <div className="court-net" aria-hidden="true" />
        <div className="court-half">{match && team(match.team_b)}</div>
      </div>

      {draft && match && (
        <>
          {!match.balanced && <p className="warn">Hai đội đang lệch {match.diff} điểm level. Đổi người hoặc vẫn cho đánh.</p>}
          <p className="hint">Chạm vào tên để đổi người.</p>
          <div className="row-2">
            <button className="btn" disabled={busy} onClick={() => act(() => api.dismiss(match.id))}>
              Bỏ đề xuất
            </button>
            <button className="btn btn-primary" disabled={busy} onClick={() => act(() => api.confirm(match.id, !match.balanced))}>
              {match.balanced ? 'Xác nhận' : 'Vẫn xác nhận'}
            </button>
          </div>
        </>
      )}
      {playing && match && (
        <button className="btn btn-play" disabled={busy} onClick={() => act(() => api.finish(match.id))}>
          Kết thúc trận
        </button>
      )}
      {!match && (
        <button className="btn btn-primary" disabled={busy} onClick={() => act(() => api.propose(court.id))}>
          Đề xuất trận
        </button>
      )}
    </section>
  )
}

function AddPlayersSheet({ state, busy, act, onClose }: ViewProps & { onClose: () => void }) {
  const [directory, setDirectory] = useState<Player[] | null>(null)
  const [query, setQuery] = useState('')
  const [creating, setCreating] = useState(false)
  const [name, setName] = useState('')
  const [gender, setGender] = useState<Gender>('M')
  const [level, setLevel] = useState<Level>('TBY')
  const [regular, setRegular] = useState(false)
  const local = useAction()

  useEffect(() => {
    local.run(() => api.players()).then((list) => list && setDirectory(list))
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const inSession = new Set(state.players.map((p) => p.player_id))
  const available = (directory ?? []).filter((p) => !inSession.has(p.id) && p.name.toLowerCase().includes(query.trim().toLowerCase()))

  const createAndCheckIn = async (e: FormEvent) => {
    e.preventDefault()
    const created = await local.run(() => api.createPlayer({ name, gender, level, is_regular: regular, monthly_fee: 0, note: '' }))
    if (!created) return
    setDirectory((d) => [...(d ?? []), created])
    await act(() => api.checkIn(state.session.id, created.id))
    setName('')
    setCreating(false)
  }

  return (
    <Sheet title="Thêm người vào buổi" onClose={onClose}>
      <ErrorBanner message={local.error} onClose={() => local.setError(null)} />
      {creating ? (
        <form className="stack" onSubmit={createAndCheckIn}>
          <Field label="Tên">
            <input value={name} onChange={(e) => setName(e.target.value)} required autoFocus />
          </Field>
          <div className="field">
            <span>Giới tính</span>
            <Segmented
              value={gender}
              onChange={setGender}
              options={[
                ['M', 'Nam'],
                ['F', 'Nữ'],
              ]}
            />
          </div>
          <div className="field">
            <span>Level</span>
            <Segmented value={level} onChange={setLevel} options={LEVELS.map((l) => [l, l])} />
          </div>
          <label className="check">
            <input type="checkbox" checked={regular} onChange={(e) => setRegular(e.target.checked)} />
            Người cố định (không thu phí buổi)
          </label>
          <div className="row-2">
            <button type="button" className="btn" onClick={() => setCreating(false)}>
              Quay lại
            </button>
            <button className="btn btn-primary" disabled={busy || local.busy}>
              Lưu và check-in
            </button>
          </div>
        </form>
      ) : (
        <>
          <input className="search" placeholder="Tìm theo tên" value={query} onChange={(e) => setQuery(e.target.value)} />
          {directory === null ? (
            <p className="empty">Đang tải danh bạ…</p>
          ) : available.length === 0 ? (
            <p className="empty">{query ? 'Không thấy ai tên như vậy trong danh bạ.' : 'Mọi người trong danh bạ đã có mặt.'}</p>
          ) : (
            <ul className="queue">
              {available.map((p) => (
                <li key={p.id}>
                  <button className="queue-row" disabled={busy} onClick={() => act(() => api.checkIn(state.session.id, p.id))}>
                    <span className="queue-name">
                      {p.name}
                      {p.is_regular && <span className="tag">Cố định</span>}
                    </span>
                    <GenderMark gender={p.gender} />
                    <LevelTag level={p.level} />
                    <span className="queue-add">Check-in</span>
                  </button>
                </li>
              ))}
            </ul>
          )}
          <button
            className="btn"
            onClick={() => {
              setName(query.trim())
              setCreating(true)
            }}
          >
            Thêm người mới vào danh bạ
          </button>
        </>
      )}
    </Sheet>
  )
}

function FeesView({ state, busy, act }: ViewProps) {
  const [editing, setEditing] = useState<SessionPlayer | null>(null)
  const { fees, players } = state
  return (
    <div className="pad stack">
      <ul className="facts facts-row">
        <li>
          <span>Phải thu</span>
          <b>{money(fees.expected)}</b>
        </li>
        <li>
          <span>Đã thu</span>
          <b className="pos">{money(fees.collected)}</b>
        </li>
        <li>
          <span>Chưa thu</span>
          <b className={fees.outstanding > 0 ? 'neg' : ''}>{money(fees.outstanding)}</b>
        </li>
      </ul>
      {players.length === 0 ? (
        <p className="empty">Chưa có ai check-in.</p>
      ) : (
        <ul className="fee-list">
          {players.map((p) => (
            <li key={p.id}>
              <button className="fee-who" onClick={() => setEditing(p)} aria-label={`Sửa phí của ${p.name}`}>
                <span className="queue-name">
                  {p.name}
                  {p.is_regular && <span className="tag">Cố định</span>}
                </span>
                <span className="fee-amount">
                  {money(p.fee)}
                  {p.fee_override !== null && ' (sửa tay)'}
                </span>
              </button>
              <button
                className={p.is_paid ? 'paid paid-on' : 'paid'}
                disabled={busy || p.fee === 0}
                aria-pressed={p.is_paid}
                onClick={() => act(() => api.setPayment(p.id, { is_paid: !p.is_paid }))}
              >
                {p.fee === 0 ? 'Miễn phí' : p.is_paid ? 'Đã thu' : 'Chưa thu'}
              </button>
            </li>
          ))}
        </ul>
      )}
      {editing && <FeeSheet player={editing} busy={busy} act={act} onClose={() => setEditing(null)} />}
    </div>
  )
}

export function FeeSheet({ player, busy, act, onClose }: { player: SessionPlayer; busy: boolean; act: Act; onClose: () => void }) {
  const [amount, setAmount] = useState(new Intl.NumberFormat('vi-VN').format(player.fee))
  const save = async (fee_override: number | null) => {
    if (fee_override !== null && Number.isNaN(fee_override)) return
    await act(() => api.setPayment(player.id, { fee_override }))
    onClose()
  }
  return (
    <Sheet title={`Phí của ${player.name}`} onClose={onClose}>
      <Field label="Phí buổi này (đ)">
        <MoneyInput value={amount} onChange={setAmount} />
      </Field>
      <button className="btn btn-primary" disabled={busy} onClick={() => save(parseMoney(amount))}>
        Lưu phí
      </button>
      {player.fee_override !== null && (
        <button className="btn" disabled={busy} onClick={() => save(null)}>
          Dùng lại phí mặc định
        </button>
      )}
    </Sheet>
  )
}
