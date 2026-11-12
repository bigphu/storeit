---
title: Importing assets
description: Create and update assets from an Excel workbook, with a preview before anything is saved.
---

Import reads the same layout as the Excel data export, so a workbook can be exported, edited and imported back. You
need the `inventory.asset.manage` permission. Open **Assets → Import**.

## Steps

1. **Download the template** (one sheet per asset type, with drop-down lists), or export the assets you want to edit.
2. **Upload** the `.xlsx` (up to 5 MB and 5000 rows). The page checks the file and shows a preview. Nothing is saved yet.
3. **Review** each row: will create, will update, no change, or failed with the reason and the Excel row number.
   Download the failed rows as a CSV to fix them.
4. **Import**. Rows are saved one by one; failed rows are left out and listed in the summary.

## The workbook

- One sheet per asset type, named by the type code (for example `LAPTOP`).
- Columns: `tag`, `name` (both required), `description`, `status`, `purchase_date`, and `attr:<key>` for each attribute.
  A column you leave out is not changed.
- An empty `tag` creates a new asset with a generated tag. A tag that already exists updates that asset.

## Rules for existing assets

- A blank cell keeps the current value; it never clears it.
- Tag, type and retired state are never changed by an import. The status cannot be changed while the asset is
  reserved or on loan.
- A tag that belongs to another type fails the row. A retired asset fails with "Restore it before importing changes".
- A tag that appears twice in the file fails every row that uses it.

## Safety

- A file with the wrong sheets or columns, too many rows or the wrong format is rejected as a whole.
- A file that creates assets can be imported only once, so uploading it twice cannot create duplicates. A file that
  only updates can be run again.
- Each saved row appears in the asset's activity history like an edit made in the form.
