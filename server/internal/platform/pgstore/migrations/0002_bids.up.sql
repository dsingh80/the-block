CREATE TABLE bids (
  id uuid PRIMARY KEY,
  listing_id uuid NOT NULL REFERENCES listings (id),
  -- Intentionally not an FK to a sessions table: this audit row must outlive a
  -- 30-minute session TTL (guidelines/06-backend-architecture.md).
  session_id text NOT NULL,
  request_id text,
  type text NOT NULL CHECK (type IN ('bid', 'buy_now')),
  amount bigint NOT NULL,
  bid_count_after int NOT NULL,
  -- From Redis TIME at accept, not insert time -- preserves true order under drain lag.
  accepted_at timestamptz NOT NULL,
  source_stream_id text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

-- Drain idempotency: the stream-tailer inserts with ON CONFLICT (listing_id, source_stream_id) DO NOTHING.
CREATE UNIQUE INDEX idx_bids_stream_dedup ON bids (listing_id, source_stream_id);
CREATE INDEX idx_bids_history ON bids (listing_id, accepted_at DESC, id DESC);
