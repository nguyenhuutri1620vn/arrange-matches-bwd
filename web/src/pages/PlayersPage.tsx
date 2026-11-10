import { useEffect, useState, type FormEvent } from 'react'
import { api, type Gender, type Level, type Player } from '../api'
import { LEVELS, money, parseMoney } from '../format'
import { ErrorBanner, Field, GenderMark, LevelTag, MoneyInput, Segmented, Sheet, useAction } from '../ui'

const BLANK: Omit<Player, 'id'> = { name: '', gender: 'M', level: 'TBY', is_regular: false, monthly_fee: 0, note: '' }

export function PlayersPage() {
  const [players, setPlayers] = useState<Player[] | null>(null)
  const [editing, setEditing] = useState<Player | Omit<Player, 'id'> | null>(null)
  const { error, setError, run } = useAction()

  const load = () => run(() => api.players()).then((list) => list && setPlayers(list))
  useEffect(() => {
    load()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const regulars = (players ?? []).filter((p) => p.is_regular)
  const casuals = (players ?? []).filter((p) => !p.is_regular)

  const group = (title: string, list: Player[]) =>
    list.length > 0 && (
      <section>
        <div className="section-head">
          <h2>
            {title} ({list.length})
          </h2>
        </div>
        <ul className="queue">
          {list.map((p) => (
            <li key={p.id}>
              <button className="queue-row" onClick={() => setEditing(p)}>
                <span className="queue-name">
                  {p.name}
                  {p.note && <small>{p.note}</small>}
                </span>
                <GenderMark gender={p.gender} />
                <LevelTag level={p.level} />
                {p.is_regular && <span className="queue-count">{money(p.monthly_fee)}/tháng</span>}
              </button>
            </li>
          ))}
        </ul>
      </section>
    )

  return (
    <>
      <header className="banner banner-plain">
        <div className="banner-main">
          <h1>Người chơi</h1>
          <p>Danh bạ dùng chung cho mọi buổi.</p>
        </div>
        <button className="btn-on-dark" onClick={() => setEditing(BLANK)}>
          Thêm người
        </button>
      </header>
      <ErrorBanner message={error} onClose={() => setError(null)} />
      <div className="pad stack">
        {players === null ? (
          <p className="empty">Đang tải danh bạ…</p>
        ) : players.length === 0 ? (
          <p className="empty">Danh bạ trống. Bấm “Thêm người” để tạo người chơi đầu tiên.</p>
        ) : (
          <>
            {group('Cố định', regulars)}
            {group('Vãng lai', casuals)}
          </>
        )}
      </div>
      {editing && (
        <PlayerSheet
          player={editing}
          onClose={() => setEditing(null)}
          onSaved={() => {
            setEditing(null)
            load()
          }}
        />
      )}
    </>
  )
}

function PlayerSheet({ player, onClose, onSaved }: { player: Player | Omit<Player, 'id'>; onClose: () => void; onSaved: () => void }) {
  const existing = 'id' in player ? player : null
  const [name, setName] = useState(player.name)
  const [gender, setGender] = useState<Gender>(player.gender)
  const [level, setLevel] = useState<Level>(player.level)
  const [regular, setRegular] = useState(player.is_regular)
  const [monthlyFee, setMonthlyFee] = useState(new Intl.NumberFormat('vi-VN').format(player.monthly_fee))
  const [note, setNote] = useState(player.note)
  const [confirmDelete, setConfirmDelete] = useState(false)
  const { busy, error, setError, run } = useAction()

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    const fee = regular ? parseMoney(monthlyFee || '0') : 0
    const body = { name, gender, level, is_regular: regular, monthly_fee: Number.isNaN(fee) ? 0 : fee, note }
    const saved = await run(() => (existing ? api.updatePlayer({ ...body, id: existing.id }) : api.createPlayer(body)))
    if (saved) onSaved()
  }

  return (
    <Sheet title={existing ? `Sửa ${existing.name}` : 'Thêm người chơi'} onClose={onClose}>
      <form className="stack" onSubmit={submit}>
        <ErrorBanner message={error} onClose={() => setError(null)} />
        <Field label="Tên">
          <input value={name} onChange={(e) => setName(e.target.value)} required />
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
          Người cố định (đóng phí tháng, không thu phí buổi)
        </label>
        {regular && (
          <Field label="Phí tháng mặc định (đ)">
            <MoneyInput value={monthlyFee} onChange={setMonthlyFee} />
          </Field>
        )}
        <Field label="Ghi chú">
          <input value={note} onChange={(e) => setNote(e.target.value)} />
        </Field>
        <button className="btn btn-primary" disabled={busy}>
          {busy ? 'Đang lưu…' : 'Lưu'}
        </button>
        {existing &&
          (confirmDelete ? (
            <button
              type="button"
              className="btn btn-danger"
              disabled={busy}
              onClick={async () => {
                if ((await run(() => api.deletePlayer(existing.id))) !== undefined) onSaved()
              }}
            >
              Xác nhận xóa {existing.name}
            </button>
          ) : (
            <button type="button" className="btn btn-danger-quiet" onClick={() => setConfirmDelete(true)}>
              Xóa khỏi danh bạ
            </button>
          ))}
        {confirmDelete && <p className="hint">Lịch sử các buổi cũ của người này vẫn được giữ.</p>}
      </form>
    </Sheet>
  )
}
