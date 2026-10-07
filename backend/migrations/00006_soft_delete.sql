-- Xoá mềm cho role và profile export: hàng giữ lại để khôi phục (Undo). Tên chỉ cần không
-- trùng giữa các hàng chưa xoá, nên ràng buộc tên thành index một phần cùng tên cũ (ánh xạ
-- lỗi trong repository không đổi).

-- +goose Up
ALTER TABLE identity.roles ADD COLUMN deleted_at timestamptz;
ALTER TABLE identity.roles DROP CONSTRAINT roles_name_key;
CREATE UNIQUE INDEX roles_name_key ON identity.roles (name) WHERE deleted_at IS NULL;

ALTER TABLE inventory.export_profiles ADD COLUMN deleted_at timestamptz;
DROP INDEX inventory.export_profiles_owner_name;
CREATE UNIQUE INDEX export_profiles_owner_name ON inventory.export_profiles (owner_id, lower(name)) WHERE deleted_at IS NULL;

-- +goose Down
DELETE FROM inventory.export_profiles WHERE deleted_at IS NOT NULL;
DROP INDEX inventory.export_profiles_owner_name;
CREATE UNIQUE INDEX export_profiles_owner_name ON inventory.export_profiles (owner_id, lower(name));
ALTER TABLE inventory.export_profiles DROP COLUMN deleted_at;

DELETE FROM identity.roles WHERE deleted_at IS NOT NULL;
DROP INDEX identity.roles_name_key;
ALTER TABLE identity.roles ADD CONSTRAINT roles_name_key UNIQUE (name);
ALTER TABLE identity.roles DROP COLUMN deleted_at;
