-- Atomic bid-accept path (guidelines/06-backend-architecture.md). Runs entirely
-- inside Redis's single-threaded script execution, so the check-then-write below
-- is genuinely atomic -- no app-level locking needed for correctness under
-- concurrent bids on the same listing.
--
-- KEYS[1] = {listing:<id>}:state hash
-- KEYS[2] = {listing:<id>}:stream
-- KEYS[3] = idempotency cache key, built server-side from (session, listing, amount)
--           -- never client-supplied, see the "Idempotency" section of
--           guidelines/06-backend-architecture.md for why this is safe and why it
--           exists at all (it's not what makes concurrent bids safe -- this
--           script already does that on its own).
-- KEYS[4] = session:<token>:bids set
--
-- ARGV[1] = amount (int, as string)
-- ARGV[2] = session_id
-- ARGV[3] = bid_id
-- ARGV[4] = idempotency cache TTL, seconds
-- ARGV[5..9] = tier1_ceiling, tier1_incr, tier2_ceiling, tier2_incr, tier3_incr
--   (domain.Tier*Ceiling/domain.Tier*Increment, passed in rather than hardcoded
--   here, so the two can never silently drift without showing up as a
--   values-only diff in review)
-- ARGV[10] = listing_id

local cached = redis.call('GET', KEYS[3])
if cached then
  return cached
end

if redis.call('EXISTS', KEYS[1]) == 0 then
  return cjson.encode({ ok = false, error = 'not_found' })
end

local d = redis.call('HMGET', KEYS[1], 'current_price', 'bid_count', 'auction_start_ms', 'auction_end_ms', 'version')
local current_price, bid_count, start_ms, end_ms, version =
  tonumber(d[1]), tonumber(d[2]), tonumber(d[3]), tonumber(d[4]), tonumber(d[5])

-- Server-authoritative clock, not an app-supplied timestamp -- avoids any clock
-- skew between multiple app instances ever disagreeing about a lifecycle
-- boundary. Safe to call inside a script: Redis replicates the script's
-- *effects*, not the script body, so this isn't a determinism hazard for
-- replication/AOF the way it would be for, say, a Lua math.random() call.
local t = redis.call('TIME')
local now_ms = (tonumber(t[1]) * 1000) + math.floor(tonumber(t[2]) / 1000)

if now_ms < start_ms then
  return cjson.encode({ ok = false, error = 'auction_not_started' })
end
if now_ms >= end_ms then
  return cjson.encode({ ok = false, error = 'auction_ended' })
end

local amount = tonumber(ARGV[1])
local increment
if current_price < tonumber(ARGV[5]) then
  increment = tonumber(ARGV[6])
elseif current_price < tonumber(ARGV[7]) then
  increment = tonumber(ARGV[8])
else
  increment = tonumber(ARGV[9])
end

local minimum = current_price + increment
if amount < minimum then
  return cjson.encode({ ok = false, error = 'bid_too_low', minimum = minimum })
end

local new_count = bid_count + 1
local new_version = version + 1
redis.call('HMSET', KEYS[1],
  'current_price', amount,
  'bid_count', new_count,
  'high_bidder_session', ARGV[2],
  'version', new_version)

redis.call('XADD', KEYS[2], '*',
  'bid_id', ARGV[3],
  'type', 'bid',
  'session_id', ARGV[2],
  'amount', amount,
  'bid_count', new_count,
  'accepted_at_ms', now_ms)

-- Backs the join-free viewer.has_bid computation (guidelines/06-backend-architecture.md).
redis.call('SADD', KEYS[4], ARGV[10])

local result = cjson.encode({
  ok = true,
  bid_id = ARGV[3],
  current_price = amount,
  bid_count = new_count,
  accepted_at_ms = now_ms,
})
redis.call('SET', KEYS[3], result, 'EX', tonumber(ARGV[4]))
return result
