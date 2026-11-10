import type { Gender, Level, TransactionType } from './api'

const TZ = 'Asia/Ho_Chi_Minh'
const number = new Intl.NumberFormat('vi-VN')

export const money = (n: number) => `${number.format(n)} đ`

/** Today's calendar day in Vietnam as YYYY-MM-DD. */
export const todayVN = () => new Intl.DateTimeFormat('en-CA', { timeZone: TZ }).format(new Date())

/** 2026-11-09 → 09/11/2026 */
export const dateVN = (iso: string) => iso.split('-').reverse().join('/')

/** 2026-11 → Tháng 11/2026 */
export const monthVN = (month: string) => {
  const [y, m] = month.split('-')
  return `Tháng ${Number(m)}/${y}`
}

export const timeVN = (iso: string | null) =>
  iso ? new Date(iso).toLocaleTimeString('vi-VN', { timeZone: TZ, hour: '2-digit', minute: '2-digit' }) : ''

export const LEVELS: Level[] = ['Y', 'Y+', 'TBY', 'TB-', 'TB', 'TB+']

export const genderLabel = (g: Gender) => (g === 'M' ? 'Nam' : 'Nữ')

export const INCOME_CATEGORIES: [string, string][] = [
  ['session_fee', 'Phí buổi'],
  ['monthly_fee', 'Phí tháng'],
  ['shuttle_sale', 'Bán cầu'],
  ['drinks', 'Nước'],
  ['other_income', 'Thu khác'],
]

export const EXPENSE_CATEGORIES: [string, string][] = [
  ['court_rent', 'Tiền thuê sân'],
  ['shuttles', 'Cầu'],
  ['utilities', 'Điện nước'],
  ['staff', 'Nhân viên'],
  ['other_expense', 'Chi khác'],
]

const categoryNames = new Map([...INCOME_CATEGORIES, ...EXPENSE_CATEGORIES])

export const categoryLabel = (c: string) => categoryNames.get(c) ?? c

export const signed = (type: TransactionType, amount: number) => (type === 'income' ? '+' : '−') + money(amount)

/** Parses what the host typed ("70.000", "70000") into whole VND. */
export const parseMoney = (text: string) => {
  const digits = text.replace(/[^\d]/g, '')
  return digits === '' ? NaN : Number(digits)
}
