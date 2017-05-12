ALTER TABLE notebooks
    ADD COLUMN query_end text,
    ADD COLUMN query_range text,
    ADD COLUMN trailing_now boolean NOT NULL;
