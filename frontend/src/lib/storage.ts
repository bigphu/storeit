// Đọc/ghi JSON trong localStorage/sessionStorage. Trình duyệt có thể chặn storage
// (cửa sổ riêng tư, chính sách), nên mọi lỗi đều bỏ qua và dùng giá trị mặc định.

type Area = 'local' | 'session'

function area(a: Area): Storage {
  return a === 'local' ? window.localStorage : window.sessionStorage
}

export function readJSON<T>(a: Area, key: string, fallback: T): T {
  try {
    const raw = area(a).getItem(key)
    return raw ? (JSON.parse(raw) as T) : fallback
  } catch {
    return fallback
  }
}

export function writeJSON(a: Area, key: string, value: unknown): void {
  try {
    area(a).setItem(key, JSON.stringify(value))
  } catch {
    // không lưu được thì thôi: chỉ là tiện ích của người dùng
  }
}
