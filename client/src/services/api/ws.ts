/**
 * One multiplexed WebSocket connection to /v1/ws (guidelines/06-backend-architecture.md,
 * "WebSocket protocol") -- matches the existing Watchlist reality (a user
 * watches many listings at once), so callers subscribe/unsubscribe listing
 * ids on this single shared connection rather than opening one socket per
 * listing. Bidding itself stays a REST action (services/api/listings.ts);
 * this is a pure push channel for observing other sessions' activity.
 */

export interface WsBidAccepted {
  type: 'bid_accepted'
  listing_id: string
  bid_id: string
  current_bid: number
  bid_count: number
  high_bidder_is_you: boolean
  accepted_at: string
}

export interface WsListingEnded {
  type: 'listing_ended'
  listing_id: string
  reason: string
  final_price: number
}

export interface WsAck {
  type: 'ack'
  subscribed: string[]
}

export interface WsError {
  type: 'error'
  code: string
  message: string
}

export type WsServerMessage = WsBidAccepted | WsListingEnded | WsAck | WsError

type Listener = (message: WsServerMessage) => void

const RECONNECT_BASE_DELAY_MS = 1000
const RECONNECT_MAX_DELAY_MS = 30_000

/**
 * Reconnect-with-backoff, and re-subscribes to every id the caller had asked
 * for once the new connection opens -- from the caller's perspective,
 * subscriptions just survive a reconnect, they don't need to notice one
 * happened at all.
 */
export class RealtimeConnection {
  private socket: WebSocket | null = null
  private readonly listeners = new Set<Listener>()
  // A ref count per id, not a Set -- more than one caller can independently
  // want the same listing live at once (e.g. it's in the inventory grid AND
  // currently open in the Preview Modal), and unsubscribing must only ever
  // tell the server once the *last* interested caller has let go.
  private readonly refCounts = new Map<string, number>()
  private reconnectAttempt = 0
  private reconnectTimer: ReturnType<typeof setTimeout> | undefined
  private manuallyClosed = false

  connect(): void {
    if (this.socket && this.socket.readyState <= WebSocket.OPEN) return
    this.manuallyClosed = false

    const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:'
    const socket = new WebSocket(`${protocol}//${location.host}/v1/ws`)
    this.socket = socket

    socket.addEventListener('open', () => {
      this.reconnectAttempt = 0
      if (this.refCounts.size > 0) {
        this.sendRaw({ type: 'subscribe', listing_ids: [...this.refCounts.keys()] })
      }
    })

    socket.addEventListener('message', (event: MessageEvent<string>) => {
      let message: WsServerMessage
      try {
        message = JSON.parse(event.data) as WsServerMessage
      } catch {
        return // an unparseable frame has nothing meaningful to recover into
      }
      for (const listener of this.listeners) listener(message)
    })

    socket.addEventListener('close', () => this.scheduleReconnect())
    socket.addEventListener('error', () => socket.close())
  }

  disconnect(): void {
    this.manuallyClosed = true
    clearTimeout(this.reconnectTimer)
    this.socket?.close()
    this.socket = null
  }

  private scheduleReconnect(): void {
    if (this.manuallyClosed) return
    const delay = Math.min(RECONNECT_BASE_DELAY_MS * 2 ** this.reconnectAttempt, RECONNECT_MAX_DELAY_MS)
    this.reconnectAttempt += 1
    this.reconnectTimer = setTimeout(() => this.connect(), delay)
  }

  private sendRaw(payload: unknown): void {
    if (this.socket?.readyState === WebSocket.OPEN) this.socket.send(JSON.stringify(payload))
  }

  subscribe(listingIds: string[]): void {
    const newIds: string[] = []
    for (const id of listingIds) {
      const count = this.refCounts.get(id) ?? 0
      if (count === 0) newIds.push(id)
      this.refCounts.set(id, count + 1)
    }
    if (newIds.length > 0) this.sendRaw({ type: 'subscribe', listing_ids: newIds })
  }

  unsubscribe(listingIds: string[]): void {
    const removedIds: string[] = []
    for (const id of listingIds) {
      const count = this.refCounts.get(id) ?? 0
      if (count <= 1) {
        this.refCounts.delete(id)
        removedIds.push(id)
      } else {
        this.refCounts.set(id, count - 1)
      }
    }
    if (removedIds.length > 0) this.sendRaw({ type: 'unsubscribe', listing_ids: removedIds })
  }

  /** Returns an unsubscribe function for this listener, mirroring the addEventListener/cleanup pattern the rest of the app already uses. */
  onMessage(listener: Listener): () => void {
    this.listeners.add(listener)
    return () => this.listeners.delete(listener)
  }
}

/** One shared connection for the whole app -- see the module doc comment above. */
export const realtimeConnection = new RealtimeConnection()
