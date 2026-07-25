---
name: erp-analysis
description: How to answer questions about the legacy ERP database accurately.
when_to_use: Any question about customers, orders, invoices, or revenue in the ERP.
---

# Analyzing the legacy ERP

Follow this procedure so every answer is grounded in the actual data, never
guessed.

1. **Inspect before you query.** Call `list_tables`, then `describe_table` on the
   tables you plan to use. Never assume a column name — the legacy schema is not
   what you would design today.

2. **Filter on real status values.** Order and invoice status is stored as text
   (e.g. `pending`, `paid`, `overdue`). Check the distinct values with a small
   query before filtering on one.

3. **Aggregate explicitly.** For "who owes the most" or "highest revenue",
   `JOIN` orders to customers, `GROUP BY` the customer, `SUM` the amount, and
   `ORDER BY` it — do not eyeball rows.

4. **Read only.** These tools are read-only by construction. If a task seems to
   need a write, stop and say so; do not attempt it.

5. **Cite your sources.** End every answer by naming the tables the figures came
   from, so a human can verify. Never state a number that did not come from a
   tool result.
