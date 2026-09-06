-- payments: InnoDB PK-only table for the UUIDv4 vs UUIDv7 insert experiment.
-- No secondary indexes. payment_id is BINARY(16) (16 raw bytes, not a CHAR UUID).
CREATE TABLE IF NOT EXISTS payments (
  payment_id   BINARY(16) PRIMARY KEY,
  merchant_id  VARCHAR(30) NOT NULL,
  customer_id  VARCHAR(30) NOT NULL,
  amount       DECIMAL(12, 2) NOT NULL,
  currency     CHAR(3) NOT NULL,
  reference_id VARCHAR(30)
);
