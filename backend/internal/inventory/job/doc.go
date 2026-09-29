// Package job khai báo args của các job nền của inventory: kiểu dữ liệu chung
// giữa chỗ enqueue (repository, trong transaction; hoặc service) và chỗ chạy
// (worker). Chỉ chứa dữ liệu: struct args, Kind(), InsertOpts().
//
// Module khác không import package này; muốn inventory làm gì thì gọi qua
// contract hoặc phát event.
package job
