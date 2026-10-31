// checkPath: chặn request mà đường dẫn còn chỗ {tham số} chưa điền (openapi-fetch để nguyên
// '{assetID}' khi id là undefined). Gửi đi thì server trả 400 "invalid parameters" và người dùng
// thấy một lỗi không do họ; chặn ở đây thì không gọi API, NoQueryId không toast, không thử lại
import { NoQueryId } from '@/lib/queryKeyId'

export function checkPath(url: string) {
  const path = decodeURIComponent(new URL(url).pathname)
  if (/\{[^}]*\}/.test(path)) throw new NoQueryId(`request path has an unfilled parameter: ${path}`)
}
