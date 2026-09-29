// Package apicommon là type Go sinh từ api/common.yaml (lỗi, phân trang, ID),
// dùng chung cho code sinh của mọi module qua import-mapping. Không sửa tay.
package apicommon

//go:generate go tool oapi-codegen -config oapi.yaml ../../../../api/common.yaml
