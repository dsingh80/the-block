CREATE TABLE listings (
  id uuid PRIMARY KEY,
  vin text NOT NULL,
  year int NOT NULL,
  make text NOT NULL,
  model text NOT NULL,
  trim text NOT NULL,
  body_style text NOT NULL,
  exterior_color text NOT NULL,
  interior_color text NOT NULL,
  engine text NOT NULL,
  transmission text NOT NULL,
  drivetrain text NOT NULL,
  odometer_km int NOT NULL,
  fuel_type text NOT NULL CHECK (fuel_type IN ('gasoline', 'hybrid', 'electric', 'diesel')),
  condition_grade numeric(2, 1) NOT NULL,
  condition_report text NOT NULL,
  damage_notes text[] NOT NULL DEFAULT '{}',
  title_status text NOT NULL CHECK (title_status IN ('clean', 'rebuilt', 'salvage')),
  province text NOT NULL,
  city text NOT NULL,

  auction_start timestamptz NOT NULL,
  -- Data, not schema: see guidelines/06-backend-architecture.md's "auction_duration_sec"
  -- entry -- changing the assumed duration is an UPDATE, not a migration.
  auction_duration_sec int NOT NULL DEFAULT 86400,
  -- Kept in sync by the trigger below, not a GENERATED column: Postgres requires a
  -- generated column's expression to be IMMUTABLE, and timestamptz + interval is only
  -- STABLE (interval addition can, in general, be calendar/timezone-dependent) --
  -- confirmed by trying it (see the integration test this migration ships with). A
  -- BEFORE INSERT/UPDATE trigger has no such restriction and gives the same
  -- can't-be-forgotten-on-insert guarantee, still as a plain indexable column.
  auction_end timestamptz NOT NULL,

  starting_bid bigint NOT NULL,
  -- Stored and queryable; excluded only from the buyer-facing DTO layer, not the schema.
  reserve_price bigint,
  buy_now_price bigint,

  images text[] NOT NULL DEFAULT '{}',
  selling_dealership text NOT NULL,
  lot text NOT NULL,

  -- Denormalized hot fields, written ONLY by the Redis stream-tailer drain (a later commit).
  current_price bigint NOT NULL,
  bid_count int NOT NULL DEFAULT 0,
  high_bidder_session_id text,
  purchased_at timestamptz,
  last_event_stream_id text,

  updated_at timestamptz NOT NULL DEFAULT now(),
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE FUNCTION set_auction_end() RETURNS trigger AS $$
BEGIN
  NEW.auction_end := NEW.auction_start + make_interval(secs => NEW.auction_duration_sec);
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_set_auction_end
  BEFORE INSERT OR UPDATE OF auction_start, auction_duration_sec ON listings
  FOR EACH ROW EXECUTE FUNCTION set_auction_end();

-- Each pair below does double duty: it backs both a sort order and the corresponding
-- status-filter range predicate (guidelines/06-backend-architecture.md).
CREATE INDEX idx_listings_auction_end ON listings (auction_end, id);
CREATE INDEX idx_listings_auction_start ON listings (auction_start, id);
CREATE INDEX idx_listings_price ON listings (current_price, id);
CREATE INDEX idx_listings_year ON listings (year DESC, id DESC);
CREATE INDEX idx_listings_make ON listings (make);
