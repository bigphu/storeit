-- name: CreateAsset :one
INSERT INTO inventory.assets
    (id, tag, name, description, asset_type_id, status_id, location_id, holder_member_id, purchase_date)
VALUES (@id, @tag, @name, @description, @asset_type_id, @status_id,
        sqlc.narg('location_id'), sqlc.narg('holder_member_id'), sqlc.narg('purchase_date'))
RETURNING *;

-- name: GetAsset :one
SELECT * FROM inventory.assets WHERE id = @id;

-- NO KEY UPDATE: không chặn insert tham chiếu tài sản (giá trị thuộc tính, sau này là phiếu mượn)
-- name: GetAssetForUpdate :one
SELECT * FROM inventory.assets WHERE id = @id FOR NO KEY UPDATE;

-- name: UpdateAsset :one
UPDATE inventory.assets
SET name = @name, description = @description, asset_type_id = @asset_type_id, status_id = @status_id,
    location_id = sqlc.narg('location_id'), holder_member_id = sqlc.narg('holder_member_id'),
    purchase_date = sqlc.narg('purchase_date'), version = version + 1, updated_at = now()
WHERE id = @id
RETURNING *;

-- name: RetireAsset :one
UPDATE inventory.assets
SET retired_at = now(), retired_reason = sqlc.narg('reason'), status_id = @status_id,
    version = version + 1, updated_at = now()
WHERE id = @id
RETURNING *;

-- name: RestoreAsset :one
UPDATE inventory.assets
SET retired_at = NULL, retired_reason = NULL, status_id = @status_id,
    version = version + 1, updated_at = now()
WHERE id = @id
RETURNING *;

-- value_number đọc dạng text (giữ đúng số thập phân, không qua float); "" là không có số
-- name: ListAssetValues :many
SELECT attribute_id, data_type, value_text, coalesce(value_number::text, '')::text AS value_number,
       value_date, value_bool, value_option_id
FROM inventory.asset_attribute_values
WHERE asset_id = @asset_id;

-- Giá trị của nhiều tài sản một lần (các dòng của một trang danh sách)
-- name: ListValuesForAssets :many
SELECT asset_id, attribute_id, data_type, value_text, coalesce(value_number::text, '')::text AS value_number,
       value_date, value_bool, value_option_id
FROM inventory.asset_attribute_values
WHERE asset_id = ANY(@asset_ids::uuid[]);

-- name: DeleteAssetValues :exec
DELETE FROM inventory.asset_attribute_values WHERE asset_id = @asset_id;

-- name: InsertAssetValue :exec
INSERT INTO inventory.asset_attribute_values
    (asset_id, attribute_id, asset_type_id, data_type, value_text, value_number, value_date, value_bool, value_option_id)
VALUES (@asset_id, @attribute_id, @asset_type_id, @data_type, sqlc.narg('value_text'),
        CAST(sqlc.narg('value_number')::text AS numeric), sqlc.narg('value_date'),
        sqlc.narg('value_bool'), sqlc.narg('value_option_id'));

-- name: ListAssets :many
SELECT a.*, t.name AS type_name, s.name AS status_name, s.kind AS status_kind
FROM inventory.assets a
JOIN inventory.asset_types t ON t.id = a.asset_type_id
JOIN inventory.asset_statuses s ON s.id = a.status_id
-- giá trị của thuộc tính để sắp (sort_attr NULL thì không khớp dòng nào)
LEFT JOIN inventory.asset_attribute_values sv ON sv.asset_id = a.id AND sv.attribute_id = sqlc.narg('sort_attr')::uuid
LEFT JOIN inventory.asset_attribute_options so ON so.id = sv.value_option_id
WHERE (sqlc.narg('q')::text IS NULL
       OR a.tag ILIKE '%' || sqlc.narg('q')::text || '%'
       OR a.name ILIKE '%' || sqlc.narg('q')::text || '%')
  AND (sqlc.narg('type_id')::uuid IS NULL OR a.asset_type_id = sqlc.narg('type_id')::uuid)
  AND (sqlc.narg('status_id')::uuid IS NULL OR a.status_id = sqlc.narg('status_id')::uuid)
  AND (sqlc.narg('status_kind')::text IS NULL OR s.kind = sqlc.narg('status_kind')::text)
  AND (sqlc.narg('location_id')::uuid IS NULL OR a.location_id = sqlc.narg('location_id')::uuid)
  AND (sqlc.narg('holder_member_id')::uuid IS NULL OR a.holder_member_id = sqlc.narg('holder_member_id')::uuid)
  AND (@include_retired::boolean OR a.retired_at IS NULL)
  AND (sqlc.narg('ids')::uuid[] IS NULL OR a.id = ANY(sqlc.narg('ids')::uuid[]))
  AND NOT EXISTS (
    -- mọi điều kiện i (f_attrs[i], f_ops[i], f_vals[i]) phải có giá trị khớp; giá trị đã kiểm
    -- ở Go nên ép kiểu trong nhánh CASE không lỗi
    SELECT 1 FROM generate_subscripts(@f_ops::text[], 1) AS f(i)
    WHERE NOT EXISTS (
      SELECT 1 FROM inventory.asset_attribute_values v
      WHERE v.asset_id = a.id AND v.attribute_id = (@f_attrs::uuid[])[f.i] AND CASE (@f_ops::text[])[f.i]
        WHEN 'text_eq' THEN lower(v.value_text) = lower((@f_vals::text[])[f.i])
        WHEN 'text_contains' THEN v.value_text ILIKE '%' || (@f_vals::text[])[f.i] || '%'
        WHEN 'number_eq' THEN v.value_number = (@f_vals::text[])[f.i]::numeric
        WHEN 'number_gt' THEN v.value_number > (@f_vals::text[])[f.i]::numeric
        WHEN 'number_gte' THEN v.value_number >= (@f_vals::text[])[f.i]::numeric
        WHEN 'number_lt' THEN v.value_number < (@f_vals::text[])[f.i]::numeric
        WHEN 'number_lte' THEN v.value_number <= (@f_vals::text[])[f.i]::numeric
        WHEN 'date_eq' THEN v.value_date = (@f_vals::text[])[f.i]::date
        WHEN 'date_gt' THEN v.value_date > (@f_vals::text[])[f.i]::date
        WHEN 'date_gte' THEN v.value_date >= (@f_vals::text[])[f.i]::date
        WHEN 'date_lt' THEN v.value_date < (@f_vals::text[])[f.i]::date
        WHEN 'date_lte' THEN v.value_date <= (@f_vals::text[])[f.i]::date
        WHEN 'boolean_eq' THEN v.value_bool = (@f_vals::text[])[f.i]::boolean
        WHEN 'select_eq' THEN v.value_option_id = (@f_vals::text[])[f.i]::uuid
        WHEN 'select_in' THEN v.value_option_id = ANY(string_to_array((@f_vals::text[])[f.i], ',')::uuid[])
      END))
ORDER BY
  CASE WHEN @sort::text = 'tag' THEN a.tag END ASC,
  CASE WHEN @sort::text = '-tag' THEN a.tag END DESC,
  CASE WHEN @sort::text = 'name' THEN lower(a.name) END ASC,
  CASE WHEN @sort::text = '-name' THEN lower(a.name) END DESC,
  CASE WHEN @sort::text = 'purchase_date' THEN a.purchase_date END ASC NULLS LAST,
  CASE WHEN @sort::text = '-purchase_date' THEN a.purchase_date END DESC NULLS LAST,
  CASE WHEN @sort::text = 'updated_at' THEN a.updated_at END ASC,
  CASE WHEN @sort::text = '-updated_at' THEN a.updated_at END DESC,
  CASE WHEN @sort::text = 'asset_type' THEN lower(t.name) END ASC,
  CASE WHEN @sort::text = '-asset_type' THEN lower(t.name) END DESC,
  CASE WHEN @sort::text = 'status' THEN s.position END ASC,
  CASE WHEN @sort::text = 'status' THEN lower(s.name) END ASC,
  CASE WHEN @sort::text = '-status' THEN s.position END DESC,
  CASE WHEN @sort::text = '-status' THEN lower(s.name) END DESC,
  -- theo thuộc tính: không có giá trị luôn ở cuối
  CASE WHEN @sort::text = 'attr_text' THEN lower(sv.value_text) END ASC NULLS LAST,
  CASE WHEN @sort::text = '-attr_text' THEN lower(sv.value_text) END DESC NULLS LAST,
  CASE WHEN @sort::text = 'attr_number' THEN sv.value_number END ASC NULLS LAST,
  CASE WHEN @sort::text = '-attr_number' THEN sv.value_number END DESC NULLS LAST,
  CASE WHEN @sort::text = 'attr_date' THEN sv.value_date END ASC NULLS LAST,
  CASE WHEN @sort::text = '-attr_date' THEN sv.value_date END DESC NULLS LAST,
  CASE WHEN @sort::text = 'attr_boolean' THEN sv.value_bool END ASC NULLS LAST,
  CASE WHEN @sort::text = '-attr_boolean' THEN sv.value_bool END DESC NULLS LAST,
  CASE WHEN @sort::text = 'attr_select' THEN so.position END ASC NULLS LAST,
  CASE WHEN @sort::text = '-attr_select' THEN so.position END DESC NULLS LAST,
  a.tag
LIMIT @lim OFFSET @off;

-- Số tài sản chưa retire của mỗi loại, tách theo kind của status (sidebar, trang loại)
-- name: CountActiveAssetsByTypeAndKind :many
SELECT a.asset_type_id, s.kind, count(*)::bigint AS n
FROM inventory.assets a
JOIN inventory.asset_statuses s ON s.id = a.status_id
WHERE a.retired_at IS NULL
GROUP BY a.asset_type_id, s.kind;

-- Số tài sản (kể cả đã retire) đang dùng mỗi status
-- name: CountAssetsByStatus :many
SELECT status_id, count(*)::bigint AS n
FROM inventory.assets
GROUP BY status_id;

-- name: CountAssets :one
SELECT count(*)
FROM inventory.assets a
JOIN inventory.asset_statuses s ON s.id = a.status_id
WHERE (sqlc.narg('q')::text IS NULL
       OR a.tag ILIKE '%' || sqlc.narg('q')::text || '%'
       OR a.name ILIKE '%' || sqlc.narg('q')::text || '%')
  AND (sqlc.narg('type_id')::uuid IS NULL OR a.asset_type_id = sqlc.narg('type_id')::uuid)
  AND (sqlc.narg('status_id')::uuid IS NULL OR a.status_id = sqlc.narg('status_id')::uuid)
  AND (sqlc.narg('status_kind')::text IS NULL OR s.kind = sqlc.narg('status_kind')::text)
  AND (sqlc.narg('location_id')::uuid IS NULL OR a.location_id = sqlc.narg('location_id')::uuid)
  AND (sqlc.narg('holder_member_id')::uuid IS NULL OR a.holder_member_id = sqlc.narg('holder_member_id')::uuid)
  AND (@include_retired::boolean OR a.retired_at IS NULL)
  AND (sqlc.narg('ids')::uuid[] IS NULL OR a.id = ANY(sqlc.narg('ids')::uuid[]))
  AND NOT EXISTS (
    -- mọi điều kiện i (f_attrs[i], f_ops[i], f_vals[i]) phải có giá trị khớp; giá trị đã kiểm
    -- ở Go nên ép kiểu trong nhánh CASE không lỗi
    SELECT 1 FROM generate_subscripts(@f_ops::text[], 1) AS f(i)
    WHERE NOT EXISTS (
      SELECT 1 FROM inventory.asset_attribute_values v
      WHERE v.asset_id = a.id AND v.attribute_id = (@f_attrs::uuid[])[f.i] AND CASE (@f_ops::text[])[f.i]
        WHEN 'text_eq' THEN lower(v.value_text) = lower((@f_vals::text[])[f.i])
        WHEN 'text_contains' THEN v.value_text ILIKE '%' || (@f_vals::text[])[f.i] || '%'
        WHEN 'number_eq' THEN v.value_number = (@f_vals::text[])[f.i]::numeric
        WHEN 'number_gt' THEN v.value_number > (@f_vals::text[])[f.i]::numeric
        WHEN 'number_gte' THEN v.value_number >= (@f_vals::text[])[f.i]::numeric
        WHEN 'number_lt' THEN v.value_number < (@f_vals::text[])[f.i]::numeric
        WHEN 'number_lte' THEN v.value_number <= (@f_vals::text[])[f.i]::numeric
        WHEN 'date_eq' THEN v.value_date = (@f_vals::text[])[f.i]::date
        WHEN 'date_gt' THEN v.value_date > (@f_vals::text[])[f.i]::date
        WHEN 'date_gte' THEN v.value_date >= (@f_vals::text[])[f.i]::date
        WHEN 'date_lt' THEN v.value_date < (@f_vals::text[])[f.i]::date
        WHEN 'date_lte' THEN v.value_date <= (@f_vals::text[])[f.i]::date
        WHEN 'boolean_eq' THEN v.value_bool = (@f_vals::text[])[f.i]::boolean
        WHEN 'select_eq' THEN v.value_option_id = (@f_vals::text[])[f.i]::uuid
        WHEN 'select_in' THEN v.value_option_id = ANY(string_to_array((@f_vals::text[])[f.i], ',')::uuid[])
      END));
