// Mã loại gợi ý từ tên: bỏ dấu (cả đ), viết hoa, ký tự khác chữ số thành "_".
// Khớp luật của backend: A-Z, 0-9, _ hoặc -, tối đa 32 ký tự.
export function codeFromName(name: string): string {
  return name
    .normalize('NFD')
    .replace(/[̀-ͯ]/g, '')
    .replace(/[đĐ]/g, 'D')
    .toUpperCase()
    .replace(/[^A-Z0-9]+/g, '_')
    .replace(/^_+|_+$/g, '')
    .slice(0, 32)
    .replace(/_+$/, '')
}

// Hai chữ đầu của mã cho ô nhận diện trên thẻ
export function codeMark(code: string): string {
  return code.replace(/[^A-Z0-9]/g, '').slice(0, 2) || code.slice(0, 2)
}
