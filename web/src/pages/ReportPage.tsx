import { useCallback, useEffect, useState } from 'react'
import { api, type MonthReport, type SessionPlayer, type SessionReport, type SessionRow, type Summary, type Totals } from '../api'
import { categoryLabel, dateVN, money, monthVN, signed } from '../format'
import { ErrorBanner, GenderMark, LevelTag, useAction } from '../ui'
import { FeeSheet } from './SessionPage'

/** Drill-down report: total → month → session → player. */
export function ReportPage() {
  const [summary, setSummary] = useState<Summary | null>(null)
  const [openMonth, setOpenMonth] = useState<string | null>(null)
  const { error, setError, run } = useAction()

  const load = useCallback(() => run(() => api.summary()).then((s) => s && setSummary(s)), [run])
  useEffect(() => {
    load()
  }, [load])

  return (
    <>
      <header className="banner banner-plain">
        <div className="banner-main">
          <h1>Báo cáo</h1>
          <p>Bấm vào tháng, rồi vào buổi để xem chi tiết.</p>
        </div>
      </header>
      <ErrorBanner message={error} onClose={() => setError(null)} />
      <div className="pad stack">
        {summary === null ? (
          <p className="empty">Đang tải báo cáo…</p>
        ) : (
          <>
            <section className="total-card">
              <h2>Toàn bộ thời gian</h2>
              <TotalsLine totals={summary} />
            </section>
            {summary.months.length === 0 && <p className="empty">Chưa có dữ liệu. Báo cáo xuất hiện sau buổi chơi hoặc khoản thu chi đầu tiên.</p>}
            {summary.months.map((m) => (
              <section key={m.month} className="tree">
                <button className="tree-head" aria-expanded={openMonth === m.month} onClick={() => setOpenMonth(openMonth === m.month ? null : m.month)}>
                  <span className="tree-title">
                    {monthVN(m.month)}
                    <small>{m.session_count} buổi</small>
                  </span>
                  <span className={m.profit < 0 ? 'tree-value neg' : 'tree-value pos'}>{money(m.profit)}</span>
                </button>
                {openMonth === m.month && <MonthDetail month={m.month} onChanged={load} />}
              </section>
            ))}
          </>
        )}
      </div>
    </>
  )
}

function TotalsLine({ totals }: { totals: Totals }) {
  return (
    <ul className="facts facts-row">
      <li>
        <span>Thu</span>
        <b>{money(totals.income)}</b>
      </li>
      <li>
        <span>Chi</span>
        <b>{money(totals.expense)}</b>
      </li>
      <li>
        <span>Lợi nhuận</span>
        <b className={totals.profit < 0 ? 'neg' : 'pos'}>{money(totals.profit)}</b>
      </li>
    </ul>
  )
}

function MonthDetail({ month, onChanged }: { month: string; onChanged: () => void }) {
  const [report, setReport] = useState<MonthReport | null>(null)
  const [openSession, setOpenSession] = useState<number | null>(null)
  const { error, setError, run } = useAction()

  const load = useCallback(() => run(() => api.monthReport(month)).then((r) => r && setReport(r)), [month, run])
  useEffect(() => {
    load()
  }, [load])

  if (!report) return error ? <ErrorBanner message={error} onClose={() => setError(null)} /> : <p className="empty">Đang tải…</p>

  return (
    <div className="tree-body">
      <TotalsLine totals={report} />
      {report.session_count > 0 && (
        <p className="hint">
          Trung bình mỗi buổi: thu {money(report.avg_income)}, lợi nhuận {money(report.avg_profit)}.
        </p>
      )}

      <h3>Nguồn thu</h3>
      <ul className="lines">
        <li>
          <span>Người cố định (phí tháng)</span>
          <b>{money(report.regular_income)}</b>
        </li>
        {report.regular_payments.map((p) => (
          <li key={p.player_id} className="line-sub">
            <span>{p.name}</span>
            <span>{money(p.amount)}</span>
          </li>
        ))}
        <li>
          <span>Vãng lai (phí buổi)</span>
          <b>{money(report.casual_income)}</b>
        </li>
        <li>
          <span>Thu khác</span>
          <b>{money(report.other_income)}</b>
        </li>
      </ul>

      {report.categories.length > 0 && (
        <>
          <h3>Theo danh mục</h3>
          <ul className="lines">
            {report.categories.map((c) => (
              <li key={c.category}>
                <span>{categoryLabel(c.category)}</span>
                <span className={c.type === 'income' ? 'pos' : 'neg'}>{signed(c.type, c.amount)}</span>
              </li>
            ))}
          </ul>
        </>
      )}

      <h3>Các buổi</h3>
      {report.sessions.length === 0 && <p className="empty">Không có buổi nào trong tháng.</p>}
      {report.sessions.map((row) => (
        <div key={row.session.id} className="tree tree-nested">
          <button
            className="tree-head"
            aria-expanded={openSession === row.session.id}
            onClick={() => setOpenSession(openSession === row.session.id ? null : row.session.id)}
          >
            <span className="tree-title">
              {dateVN(row.session.date)}
              <small>
                {row.session.court_count} sân, {row.player_count > 0 ? `${row.player_count} người` : 'không có danh sách'}
                {row.session.status === 'OPEN' && ', đang mở'}
              </small>
            </span>
            <span className={row.profit < 0 ? 'tree-value neg' : 'tree-value pos'}>{money(row.profit)}</span>
          </button>
          {openSession === row.session.id && (
            <SessionDetail
              row={row}
              onChanged={() => {
                load()
                onChanged()
              }}
            />
          )}
        </div>
      ))}

      {report.unlinked.length > 0 && (
        <>
          <h3>Không thuộc buổi</h3>
          <ul className="lines">
            {report.unlinked.map((t) => (
              <li key={t.id}>
                <span>
                  {categoryLabel(t.category)}
                  {t.player_name && `: ${t.player_name}`}
                  <small>{dateVN(t.date)}</small>
                </span>
                <span className={t.type === 'income' ? 'pos' : 'neg'}>{signed(t.type, t.amount)}</span>
              </li>
            ))}
          </ul>
        </>
      )}
    </div>
  )
}

function SessionDetail({ row, onChanged }: { row: SessionRow; onChanged: () => void }) {
  const [detail, setDetail] = useState<SessionReport | null>(null)
  const [editing, setEditing] = useState<SessionPlayer | null>(null)
  const { busy, error, setError, run } = useAction()
  const id = row.session.id

  const load = useCallback(() => run(() => api.sessionReport(id)).then((d) => d && setDetail(d)), [id, run])
  useEffect(() => {
    load()
  }, [load])

  // Payment edits are allowed on ended sessions; reload every level after.
  const act = async (fn: () => Promise<unknown>) => {
    const out = await run(fn)
    if (out !== undefined) {
      await load()
      onChanged()
    }
    return undefined
  }

  return (
    <div className="tree-body">
      <ErrorBanner message={error} onClose={() => setError(null)} />
      <ul className="lines">
        {row.player_count > 0 && (
          <>
            <li>
              <span>Người chơi</span>
              <span>
                {row.male} nam, {row.female} nữ ({row.casual} vãng lai, {row.regular} cố định)
              </span>
            </li>
            <li>
              <span>Phí phải thu</span>
              <span>{money(row.fees.expected)}</span>
            </li>
            <li>
              <span>Chưa thu</span>
              <span className={row.fees.outstanding > 0 ? 'neg' : ''}>{money(row.fees.outstanding)}</span>
            </li>
          </>
        )}
        <li>
          <span>Phí buổi đã thu</span>
          <span className="pos">{money(row.fee_income)}</span>
        </li>
        <li>
          <span>Thu khác</span>
          <span>{money(row.other_income)}</span>
        </li>
        <li>
          <span>Chi</span>
          <span className="neg">{money(row.expense)}</span>
        </li>
        <li>
          <span>Lợi nhuận buổi</span>
          <b className={row.profit < 0 ? 'neg' : 'pos'}>{money(row.profit)}</b>
        </li>
      </ul>

      {detail === null ? (
        <p className="empty">Đang tải…</p>
      ) : detail.players.length === 0 ? (
        <p className="empty">Buổi nhập từ sổ cũ, không có danh sách người chơi.</p>
      ) : (
        <ul className="fee-list">
          {detail.players.map((p) => (
            <li key={p.id}>
              <button className="fee-who" onClick={() => setEditing(p)} aria-label={`Sửa phí của ${p.name}`}>
                <span className="queue-name">
                  {p.name} <GenderMark gender={p.gender} /> <LevelTag level={p.level} />
                </span>
                <span className="fee-amount">
                  {p.is_regular ? 'Cố định' : 'Vãng lai'}, {p.matches_played} trận, phí {money(p.fee)}
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
