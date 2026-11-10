import { useCallback, useEffect, useState, type ReactNode } from 'react'
import { ApiError, type Gender, type Level } from './api'
import { genderLabel, parseMoney } from './format'

/**
 * Runs data-changing calls with a loading flag and a Vietnamese error
 * message. Returns undefined when the call failed.
 */
export function useAction() {
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const run = useCallback(async <T,>(fn: () => Promise<T>): Promise<T | undefined> => {
    setBusy(true)
    setError(null)
    try {
      return await fn()
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Có lỗi xảy ra. Vui lòng thử lại.')
      return undefined
    } finally {
      setBusy(false)
    }
  }, [])
  return { busy, error, setError, run }
}

export function ErrorBanner({ message, onClose }: { message: string | null; onClose: () => void }) {
  if (!message) return null
  return (
    <div className="error" role="alert">
      <span>{message}</span>
      <button className="error-close" onClick={onClose} aria-label="Đóng thông báo">
        ×
      </button>
    </div>
  )
}

/** Bottom sheet for one-handed use. */
export function Sheet({ title, onClose, children }: { title: string; onClose: () => void; children: ReactNode }) {
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && onClose()
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [onClose])
  return (
    <div className="sheet-backdrop" onClick={onClose}>
      <div className="sheet" role="dialog" aria-modal="true" aria-label={title} onClick={(e) => e.stopPropagation()}>
        <div className="sheet-head">
          <h2>{title}</h2>
          <button className="btn-ghost" onClick={onClose}>
            Đóng
          </button>
        </div>
        <div className="sheet-body">{children}</div>
      </div>
    </div>
  )
}

export function Shuttle({ size = 28 }: { size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 64 64" aria-hidden="true">
      <g transform="rotate(-32 32 32)">
        <path d="M32 16 18 54h28z" fill="#fff" />
        <path d="M32 16 25 54M32 16v38M32 16l7 38M21 46h22" stroke="currentColor" strokeWidth="1.6" fill="none" />
        <circle cx="32" cy="14" r="8" fill="var(--play)" />
      </g>
    </svg>
  )
}

export function LevelTag({ level }: { level: Level }) {
  return <span className="level">{level}</span>
}

export function GenderMark({ gender }: { gender: Gender }) {
  return (
    <span className={gender === 'M' ? 'gender gender-m' : 'gender gender-f'} title={genderLabel(gender)}>
      {genderLabel(gender)}
    </span>
  )
}

/** Text input for whole VND amounts; shows thousands separators as typed. */
export function MoneyInput({ value, onChange, id }: { value: string; onChange: (v: string) => void; id?: string }) {
  return (
    <input
      id={id}
      inputMode="numeric"
      value={value}
      onChange={(e) => {
        const n = parseMoney(e.target.value)
        onChange(Number.isNaN(n) ? '' : new Intl.NumberFormat('vi-VN').format(n))
      }}
      placeholder="0"
    />
  )
}

export function Field({ label, children }: { label: string; children: ReactNode }) {
  return (
    <label className="field">
      <span>{label}</span>
      {children}
    </label>
  )
}

export function Segmented<T extends string | number>({
  value,
  options,
  onChange,
}: {
  value: T
  options: [T, string][]
  onChange: (v: T) => void
}) {
  return (
    <div className="segmented" role="group">
      {options.map(([v, label]) => (
        <button key={String(v)} type="button" className={v === value ? 'on' : ''} aria-pressed={v === value} onClick={() => onChange(v)}>
          {label}
        </button>
      ))}
    </div>
  )
}
