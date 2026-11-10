import { useEffect, useState, type FormEvent } from 'react'
import { api, type Settings } from '../api'
import { parseMoney } from '../format'
import { ErrorBanner, Field, MoneyInput, useAction } from '../ui'

const fmt = new Intl.NumberFormat('vi-VN')

export function SettingsPage({ onLogout }: { onLogout: () => void }) {
  const [settings, setSettings] = useState<Settings | null>(null)
  const [feeMale, setFeeMale] = useState('')
  const [feeFemale, setFeeFemale] = useState('')
  const [courtFee, setCourtFee] = useState('')
  const [saved, setSaved] = useState(false)
  const { busy, error, setError, run } = useAction()

  useEffect(() => {
    run(() => api.settings()).then((s) => {
      if (!s) return
      setSettings(s)
      setFeeMale(fmt.format(s.fee_male))
      setFeeFemale(fmt.format(s.fee_female))
      setCourtFee(fmt.format(s.court_fee))
    })
  }, [run])

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    if (!settings) return
    const next = { ...settings, fee_male: parseMoney(feeMale), fee_female: parseMoney(feeFemale), court_fee: parseMoney(courtFee || '0') }
    if ([next.fee_male, next.fee_female, next.court_fee].some(Number.isNaN)) {
      setError('Nhập đủ các mức phí trước khi lưu.')
      return
    }
    setSaved(false)
    if (await run(() => api.saveSettings(next))) {
      setSettings(next)
      setSaved(true)
    }
  }

  const setRule = (key: 'gender_gap_threshold' | 'balance_threshold' | 'repeat_lookback', value: string) =>
    settings && setSettings({ ...settings, [key]: Math.max(0, Number(value) || 0) })

  return (
    <>
      <header className="banner banner-plain">
        <div className="banner-main">
          <h1>Cài đặt</h1>
          <p>Mức phí điền sẵn và quy tắc ghép trận.</p>
        </div>
      </header>
      <ErrorBanner message={error} onClose={() => setError(null)} />
      {settings === null ? (
        <p className="empty">Đang tải cài đặt…</p>
      ) : (
        <form className="pad stack" onSubmit={submit}>
          <h2>Phí điền sẵn khi mở buổi</h2>
          <div className="row-2">
            <Field label="Phí nam (đ)">
              <MoneyInput value={feeMale} onChange={setFeeMale} />
            </Field>
            <Field label="Phí nữ (đ)">
              <MoneyInput value={feeFemale} onChange={setFeeFemale} />
            </Field>
          </div>
          <Field label="Tiền thuê mỗi sân mỗi buổi (đ)">
            <MoneyInput value={courtFee} onChange={setCourtFee} />
          </Field>

          <h2>Quy tắc ghép trận</h2>
          <Field label="Hai đội được lệch tối đa (điểm level)">
            <input type="number" min={0} max={10} value={settings.balance_threshold} onChange={(e) => setRule('balance_threshold', e.target.value)} />
          </Field>
          <Field label="Đội hình 3-1: ba người phải hơn người lẻ ít nhất (điểm level trung bình)">
            <input type="number" min={0} max={5} value={settings.gender_gap_threshold} onChange={(e) => setRule('gender_gap_threshold', e.target.value)} />
          </Field>
          <Field label="Tránh lặp đồng đội, đối thủ trong số trận gần nhất">
            <input type="number" min={0} max={5} value={settings.repeat_lookback} onChange={(e) => setRule('repeat_lookback', e.target.value)} />
          </Field>

          <button className="btn btn-primary" disabled={busy}>
            {busy ? 'Đang lưu…' : 'Lưu cài đặt'}
          </button>
          {saved && <p className="hint pos">Đã lưu cài đặt.</p>}
        </form>
      )}
      <div className="pad">
        <button
          className="btn"
          onClick={async () => {
            await run(() => api.logout())
            onLogout()
          }}
        >
          Đăng xuất
        </button>
      </div>
    </>
  )
}
