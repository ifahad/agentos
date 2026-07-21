-- Seeds the demo "legacy ERP" database the SQL connector exposes to agents.
-- Runs once via the postgres image entrypoint as the agentos superuser.

CREATE DATABASE legacy_erp;

\connect legacy_erp

CREATE ROLE erp_reader LOGIN PASSWORD 'erp_reader';

CREATE TABLE customers (
    id          serial PRIMARY KEY,
    name        text        NOT NULL,
    city        text        NOT NULL,
    segment     text        NOT NULL, -- enterprise | smb | government
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE orders (
    id           serial PRIMARY KEY,
    customer_id  int         NOT NULL REFERENCES customers (id),
    ordered_at   date        NOT NULL,
    status       text        NOT NULL, -- fulfilled | pending | cancelled
    total_sar    numeric(12, 2) NOT NULL
);

CREATE TABLE invoices (
    id          serial PRIMARY KEY,
    order_id    int  NOT NULL REFERENCES orders (id),
    issued_at   date NOT NULL,
    paid        boolean NOT NULL DEFAULT false,
    amount_sar  numeric(12, 2) NOT NULL
);

INSERT INTO customers (name, city, segment) VALUES
    ('Al-Faisal Trading Co.',      'Riyadh',  'enterprise'),
    ('Najd Industrial Supplies',   'Riyadh',  'smb'),
    ('Red Sea Logistics',          'Jeddah',  'enterprise'),
    ('Hejaz Retail Group',         'Jeddah',  'smb'),
    ('Eastern Petro Services',     'Dammam',  'enterprise'),
    ('Qassim Agri Traders',        'Buraidah','smb'),
    ('Ministry of Municipal Works','Riyadh',  'government'),
    ('Tabuk Construction Partners','Tabuk',   'smb');

INSERT INTO orders (customer_id, ordered_at, status, total_sar) VALUES
    (1, '2026-01-14', 'fulfilled', 482000.00),
    (1, '2026-02-02', 'fulfilled', 315500.00),
    (1, '2026-03-19', 'fulfilled', 268750.00),
    (1, '2026-05-30', 'pending',   199000.00),
    (2, '2026-01-25', 'fulfilled',  42300.00),
    (2, '2026-04-11', 'cancelled',  18950.00),
    (3, '2026-01-08', 'fulfilled', 236400.00),
    (3, '2026-02-27', 'fulfilled', 189900.00),
    (3, '2026-06-15', 'pending',   244000.00),
    (4, '2026-02-14', 'fulfilled',  67800.00),
    (4, '2026-05-03', 'fulfilled',  58200.00),
    (5, '2026-01-30', 'fulfilled', 391000.00),
    (5, '2026-03-22', 'fulfilled', 176500.00),
    (5, '2026-06-28', 'pending',   210000.00),
    (6, '2026-03-05', 'fulfilled',  23400.00),
    (6, '2026-06-01', 'fulfilled',  31750.00),
    (7, '2026-02-09', 'fulfilled', 154000.00),
    (7, '2026-04-20', 'pending',   162500.00),
    (8, '2026-01-17', 'fulfilled',  88600.00),
    (8, '2026-03-29', 'cancelled',  45200.00),
    (8, '2026-05-25', 'fulfilled',  91300.00);

INSERT INTO invoices (order_id, issued_at, paid, amount_sar)
SELECT id, ordered_at + 7, status = 'fulfilled', total_sar
FROM orders
WHERE status <> 'cancelled';

GRANT CONNECT ON DATABASE legacy_erp TO erp_reader;
GRANT USAGE ON SCHEMA public TO erp_reader;
GRANT SELECT ON ALL TABLES IN SCHEMA public TO erp_reader;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT ON TABLES TO erp_reader;
