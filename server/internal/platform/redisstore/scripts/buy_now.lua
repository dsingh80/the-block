-- Atomic buy-now accept path -- a structural sibling of place_bid.lua (same
-- existence/idempotency/lifecycle checks), with a different minimum-price rule
-- and a different write (guidelines/06-backend-architecture.md).
--
-- KEYS[1] = {listing:<id>}:state hash
-- KEYS[2] = {listing:<id>}:stream
-- KEYS[3] = idempotency cache key, built server-side from (session, listing) only
--           -- no amount component, since a listing has exactly one buy-now price.
-- KEYS[4] = session:<token>:bids set
--
-- ARGV[1] = session_id
-- ARGV[2] = bid_id
-- ARGV[3] = idempotency cache TTL, seconds
-- ARGV[4] = listing_id

local cached = redis.call('GET', KEYS[3])
if cached then
  return cached
end

if redis.call('EXISTS', KEYS[1]) == 0 then
  return cjson.encode({ ok = false, error = 'not_found' })
end

local d = redis.call('HMGET', KEYS[1], 'current_price', 'bid_count', 'auction_start_ms', 'auction_end_ms', 'version', 'buy_now_price')
local current_price, bid_count, start_ms, end_ms, version, buy_now_price =
  tonumber(d[1]), tonumber(d[2]), tonumber(d[3]), tonumber(d[4]), tonumber(d[5]), tonumber(d[6])

local t = redis.call('TIME')
local now_ms = (tonumber(t[1]) * 1000) + math.floor(tonumber(t[2]) / 1000)

if now_ms < start_ms then
  return cjson.encode({ ok = false, error = 'auction_not_started' })
end
if now_ms >= end_ms then
  return cjson.encode({ ok = false, error = 'auction_ended' })
end

-- A missing buy_now_price field decodes as Lua nil here (HMGET returns false for
-- an absent field, and tonumber(false) is nil) -- that and "someone already
-- bid past it" are both "unavailable," a new rule today's single-bidder client
-- never needed (guidelines/06-backend-architecture.md).
if not buy_now_price or current_price >= buy_now_price then
  return cjson.encode({ ok = false, error = 'buy_now_unavailable' })
end

local new_count = bid_count + 1
local new_version = version + 1
redis.call('HMSET', KEYS[1],
  'current_price', buy_now_price,
  'bid_count', new_count,
  'high_bidder_session', ARGV[1],
  'version', new_version,
  'purchased_at_ms', now_ms)

redis.call('XADD', KEYS[2], '*',
  'bid_id', ARGV[2],
  'type', 'buy_now',
  'session_id', ARGV[1],
  'amount', buy_now_price,
  'bid_count', new_count,
  'accepted_at_ms', now_ms)

redis.call('SADD', KEYS[4], ARGV[4])

local result = cjson.encode({
  ok = true,
  bid_id = ARGV[2],
  current_price = buy_now_price,
  bid_count = new_count,
  accepted_at_ms = now_ms,
})
redis.call('SET', KEYS[3], result, 'EX', tonumber(ARGV[3]))
return result
