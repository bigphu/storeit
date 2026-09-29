// Package migrations nhúng các file goose vào binary, để cmd/migrate chạy được
// mà không cần file .sql trên đĩa. sqlc cũng đọc thư mục này làm schema.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
