ALTER TABLE uploads ADD COLUMN skip_identical INTEGER NOT NULL DEFAULT 0;
ALTER TABLE uploads ADD COLUMN existing_only INTEGER NOT NULL DEFAULT 0;
UPDATE uploads SET received = 0 WHERE ready = 0 AND size > 0 AND received = size;
