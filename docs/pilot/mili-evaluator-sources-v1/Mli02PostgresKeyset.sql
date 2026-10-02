-- MLI-02 database acceptance probe. Reproduces the pinned repository's
-- tuple predicate, DESC ordering, LIMIT 50, and API millisecond cursor encoding.
-- Run against a disposable PostgreSQL 18 database with psql -X -f this file.
\set ON_ERROR_STOP on
CREATE TEMP TABLE inbound_event (id text PRIMARY KEY, tenant_id uuid NOT NULL, status text NOT NULL, event_type text NOT NULL, received_at timestamptz NOT NULL);
CREATE TEMP TABLE pilot_results (scenario text, rows_returned integer, unique_ids integer, expected integer);
CREATE OR REPLACE PROCEDURE run_pagination_scenario(label text) LANGUAGE plpgsql AS $$
DECLARE cursor_time timestamptz := NULL; cursor_id text := NULL; row_data record; page_count int; seen text[] := ARRAY[]::text[];
BEGIN
  LOOP
    page_count := 0;
    FOR row_data IN
      SELECT id, tenant_id, event_type, status, received_at
      FROM inbound_event
      WHERE tenant_id = '11111111-1111-1111-1111-111111111111'
        AND (cursor_time IS NULL OR (received_at, id) < (cursor_time, cursor_id))
      ORDER BY received_at DESC, id DESC
      LIMIT 50
    LOOP
      page_count := page_count + 1;
      seen := array_append(seen, row_data.id);
      cursor_time := to_timestamp(floor(extract(epoch FROM row_data.received_at) * 1000) / 1000);
      cursor_id := row_data.id;
    END LOOP;
    EXIT WHEN page_count < 50;
  END LOOP;
  INSERT INTO pilot_results VALUES(label, cardinality(seen), (SELECT count(DISTINCT value)::int FROM unnest(seen) value), (SELECT count(*)::int FROM inbound_event));
END $$;
INSERT INTO inbound_event
SELECT 'evt-' || lpad(g::text, 3, '0'), '11111111-1111-1111-1111-111111111111'::uuid, 'PROCESSED', 'pilot',
       timestamptz '2026-09-27 12:00:00+00' - (g / 2) * interval '1 millisecond'
FROM generate_series(0,119) AS g;
CALL run_pagination_scenario('exact_millisecond_ties');
TRUNCATE inbound_event;
INSERT INTO inbound_event
SELECT 'micro-' || lpad(g::text, 3, '0'), '11111111-1111-1111-1111-111111111111'::uuid, 'PROCESSED', 'pilot',
       timestamptz '2026-09-27 12:00:00+00' + g * interval '1 microsecond'
FROM generate_series(1,120) AS g;
CALL run_pagination_scenario('submillisecond_precision');
SELECT scenario, rows_returned, unique_ids, expected, expected - rows_returned AS missing FROM pilot_results ORDER BY scenario;

