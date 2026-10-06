// Tải file nhận từ API (blob): tên file lấy từ Content-Disposition, lưu qua một liên kết tạm

// fileNameFrom: ưu tiên filename*=utf-8''… (tên có dấu), rồi filename="…"
export function fileNameFrom(header: string | null, fallback: string): string {
  if (!header) return fallback
  const star = /filename\*\s*=\s*utf-8''([^;]+)/i.exec(header)
  if (star) {
    try {
      return decodeURIComponent(star[1].trim())
    } catch {
      // tên mã hoá hỏng: dùng tên thường bên dưới
    }
  }
  const plain = /filename\s*=\s*"?([^";]+)"?/i.exec(header)
  return plain ? plain[1].trim() : fallback
}

export function saveBlob(blob: Blob, name: string): void {
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = name
  document.body.appendChild(a)
  a.click()
  a.remove()
  setTimeout(() => URL.revokeObjectURL(url), 1000)
}
