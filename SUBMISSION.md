# Openlane THE BLOCK
## How to Run

Requires **Docker Desktop** (WSL2 backend, on Windows). 

#### Accessing the application
> Navigate to https://localhost (no port)

#### Starting the application
```bash
cd server
./deploy/docker/setup-ssl-windows.sh   # once per machine
docker compose up --build
```

## Time Spent

Day 1 - I spent a few (3-4) hours to research some auction basics and set up the frontend portion of this project as recommended.  

Day 2 - I had some time so I came back and designed the backend portion a few days later. I spent much more time this day simply because I had a working frontend so I could refine the full-system design decisions. I thoroughly iterated on a plan for an hour or two but the AI agent took most of the time implementing it. 

Both days, I went back after the agent(s) were done and fixed any bugs or issues I found.

## Assumptions and Scope

- **24-hour auction duration.** The dataset's `auction_start` has no matching end time, and the source requirements only specify a start. 24 hours is an explicit, unconfirmed assumption — now stored as data (`listings.auction_duration_sec`, default 86400), not hardcoded, so changing it later is an `UPDATE`, not a migration (`guidelines/06-backend-architecture.md`, "Data layer").
- **Once you place a bid, you can't lower it or take it back** I went with the greedy option for fun but this would be something hashed out in business logic
- **Rival bidding is real, but nothing bids on its own.** Any two sessions (two browsers, or a normal tab + incognito) are independent bidders. There's still no automated opponent placing bids for you, so seeing `Outbid` in a single session means opening a second one and bidding there.
- **Sessions are anonymous.** An opaque cookie-backed session id, not a real account — no login, no password, no identity that survives switching browsers/devices. This is intentional because we don't have user auth.
- **`reserve_price` is in the dataset but intentionally not shown** - I wasn't sure what this is for since we have a buy_now price.
- **Watchlist starts empty.** No hand-picked seed data.
- **Money is a whole number everywhere** (the `server/` backend and its Postgres schema included) — every cost in this domain is flat, with no decimal-prone fees or taxes, so there are no cents anywhere: `starting_bid`, `current_bid`, bid increments, all whole integers. If fractional currency is ever needed, the two migration paths are (a) switch the column/application type to a decimal type, or (b) keep integers and scale by 100, treating the last two digits as cents — either way, deliberately not a silent default.

## Stack

- **Frontend:** Vite, Typescript, VueJS (framework I know best)
- **Backend:** Golang (highly performant, minimal binary, strictly typed)
- **Database:** Redis (atomic bidding) + PostgresQL (non-volatile audit trail)

## What I Built


#### Frontend UI/UX
I asked a few questions about who I'm designing this for, what information they would want to see at a glance, what the data looks like (answers questions about features we can support), and then researched the basic auction process. This all culminated in the following:
- card grid with infinite scroll on results
- filter, sort, offset-based pagination
- preview modal for quick access to listings without losing cursor position
- watchlist for listings that we're interested in or bid on
- compare feature for side-by-side easy vehicle comparisons
- bid card with consistent access on both the inventory page and the listing page
- fuzzy search with support for make/model/vin/seller (db query with conditions)

#### Server + Storage
Bidding was my highest priority here followed by security. I wanted to make sure the application was easily runnable so I Dockerized it.
Bidding needs to be highly auditable and atomic so we can refute any issues a bidder may have. Redis is perfect for this as the single-threaded nature guarantees atomic operations and it runs in memory so it's super fast. I wanted to use this as my primary database originally but decided to use PostgresQL instead because it's both non-volatile AND much easier to use with filter/sort/paginate.
- Redis for atomic operations and fast access
- PostgresQL for non-volatile audit trail and filter/sort/pagination support
- Offset-based pagination to prevent issues with duplicate results as the filter set grows
- GoLang because it's super fast, strictly typed, and I like the syntax
- Caddy because it's an easy to run reverse proxy with native SSL cert support through LetsEncrypt
- SSL required because I wanted to use secure WebSockets and session cookies
- Websockets for broadcasting bidding updates in mass (this is one-way)
- REST API for placing new bids and basic resource access (search listings/filters, etc.)

## Testing

Test coverage is available with nearly every commit. Every commit has a well-defined message for what was accomplished, how and why. The tests range from individual functions up to full integration.

## What I'd Do With More Time

- Fix the bug where you can increase your bid but you receive a "You have been outbid" error.
- Add anti-snipe measures (extend time on last-minute bid)
- Sanitize data for things like VIN (used in an external link)
- True proxy/second-price bidding: a real bidder pool exists now, but a bid still becomes the new price outright rather than an auto-raised maximum competing against other bidders' stored maximums. (you place your max but the max becomes the winning bid if it's high enough)
- Add auto-bidding functionality with a pre-bid phase so users can budget ahead of time and not worry about auction timing
- User authentication & access control (historical data and legal requirements for placing a bid, auditing, data persistence on watchlist, etc.)
- Gap-free reconnect on WebSockets - right now we just resubscribe to the same ids so we can miss events in between. If I had time, i'd replay any events that occurred in between as a true reconnect
- "My Auctions" page for viewing any auctions that the user bid on and enforce any SOPs that must occur after an auction is over
- Printer-friendly styling for listing details page
