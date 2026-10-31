-- Tag tự sinh: mỗi loại có tiền tố tag (LAP, MON...), tài sản mới không nhập tag thì lấy
-- <tiền tố>-<số kế tiếp> (LAP-0091). Loại đã có lấy tiền tố mà tag hiện có của nó dùng nhiều
-- nhất (dạng PREFIX-123); loại chưa có tag như vậy lấy 3 ký tự đầu của code. Tiền tố trùng
-- nhau thì loại tạo sau thêm số (LAP2) vì mỗi tiền tố chỉ thuộc một loại.

-- +goose Up
ALTER TABLE inventory.asset_types ADD COLUMN tag_prefix text;

UPDATE inventory.asset_types t
SET tag_prefix = p.prefix
FROM (
    SELECT DISTINCT ON (asset_type_id) asset_type_id, substring(tag FROM '^([A-Z0-9]{1,10})-[0-9]+$') AS prefix
    FROM inventory.assets
    WHERE tag ~ '^[A-Z0-9]{1,10}-[0-9]+$'
    GROUP BY asset_type_id, substring(tag FROM '^([A-Z0-9]{1,10})-[0-9]+$')
    ORDER BY asset_type_id, count(*) DESC, 2
) p
WHERE p.asset_type_id = t.id;

UPDATE inventory.asset_types
SET tag_prefix = coalesce(nullif(left(regexp_replace(code, '[^A-Z0-9]', '', 'g'), 3), ''), 'AST')
WHERE tag_prefix IS NULL;

UPDATE inventory.asset_types t
SET tag_prefix = left(d.tag_prefix, 8) || d.n
FROM (
    SELECT id, tag_prefix, row_number() OVER (PARTITION BY tag_prefix ORDER BY created_at, id) AS n
    FROM inventory.asset_types
) d
WHERE d.id = t.id AND d.n > 1;

ALTER TABLE inventory.asset_types
    ALTER COLUMN tag_prefix SET NOT NULL,
    ADD CONSTRAINT asset_types_tag_prefix_check CHECK (tag_prefix ~ '^[A-Z0-9]{1,10}$'),
    ADD CONSTRAINT asset_types_tag_prefix_key UNIQUE (tag_prefix);

-- +goose Down
ALTER TABLE inventory.asset_types DROP COLUMN tag_prefix;
