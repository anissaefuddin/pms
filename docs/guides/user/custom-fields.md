# Custom fields

## Purpose
Add your own fields (e.g. Impact, Budget) to items in a Space.

## Steps
1. Open the Space and choose **Custom fields** (“Manage custom fields available in this Space.”).
2. **Add custom field**: enter **Field name** and **Field type**.
3. Types: **Text**, **Number**, **Relation to items** (optionally limited to one item type key), **Formula**, **Currency**, **Percent**, **Single select**, **Multiple select**, **People**, **Date**, **Duration**, **Checkbox**.
4. Type options: select types take **Options** (label and color; **Add option**); Currency has **Currency code** and **Decimal places**; Percent has **Allow values over 100%**; others may offer a **Default aggregation** (None, Sum, Average, Minimum, Maximum, Count) and **Conditional markers** (comparison + value + color; the first matching rule sets the color; **Add marker**).
5. **Formula**: use numbers, `[Field name]`, `+ - * /`, comparisons, `min`, `max`, `round`, `abs`, `if`. The result is calculated; if an input is missing you see “No result”.
6. Reorder with **Move {name} up/down**; **Edit**; **Archive** (archived fields are hidden but kept).
7. To set a value, open an item → **Custom fields** → **Edit {name}**; **Clear value** removes it. Show a field as a column via **Columns → Custom fields**.

## Validation
Number: up to the allowed decimals; Percent 0–max; Duration: whole days; Text: max 5,000 characters. A relation field that also links items you cannot access cannot be edited by you.

## Permissions
Managing fields needs edit rights on the Space; editing values needs edit rights on the item.

## Limitations
Fields are scoped per Space. Totals/aggregation display depends on the view.
