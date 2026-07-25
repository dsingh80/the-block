import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { RealtimeConnection, type WsServerMessage } from './ws'

type FakeListener = (event: { data?: string }) => void

/** Stands in for the browser's WebSocket -- exposes open()/close()/message() so a test can drive the connection lifecycle directly instead of needing a real socket. */
class FakeWebSocket {
  static instances: FakeWebSocket[] = []
  static readonly CONNECTING = 0
  static readonly OPEN = 1
  static readonly CLOSING = 2
  static readonly CLOSED = 3

  readonly url: string
  readyState = FakeWebSocket.CONNECTING
  readonly sent: string[] = []
  private readonly listeners: Record<string, FakeListener[]> = {}

  constructor(url: string) {
    this.url = url
    FakeWebSocket.instances.push(this)
  }

  addEventListener(type: string, listener: FakeListener): void {
    ;(this.listeners[type] ??= []).push(listener)
  }

  send(data: string): void {
    this.sent.push(data)
  }

  close(): void {
    this.readyState = FakeWebSocket.CLOSED
    this.emit('close', {})
  }

  open(): void {
    this.readyState = FakeWebSocket.OPEN
    this.emit('open', {})
  }

  message(payload: unknown): void {
    this.emit('message', { data: JSON.stringify(payload) })
  }

  messageRaw(data: string): void {
    this.emit('message', { data })
  }

  sentMessages(): unknown[] {
    return this.sent.map((raw) => JSON.parse(raw))
  }

  private emit(type: string, event: { data?: string }): void {
    for (const listener of this.listeners[type] ?? []) listener(event)
  }
}

describe('RealtimeConnection', () => {
  beforeEach(() => {
    FakeWebSocket.instances = []
    vi.stubGlobal('WebSocket', FakeWebSocket)
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    vi.useRealTimers()
  })

  it('opens a socket to /v1/ws on the current origin', () => {
    new RealtimeConnection().connect()

    expect(FakeWebSocket.instances).toHaveLength(1)
    expect(FakeWebSocket.instances[0]!.url).toBe(`ws://${location.host}/v1/ws`)
  })

  it('does not open a second socket while one is already connecting or open', () => {
    const conn = new RealtimeConnection()
    conn.connect()
    conn.connect()

    expect(FakeWebSocket.instances).toHaveLength(1)
  })

  it('subscribe sends only ids not already subscribed', () => {
    const conn = new RealtimeConnection()
    conn.connect()
    FakeWebSocket.instances[0]!.open()

    conn.subscribe(['a', 'b'])
    conn.subscribe(['b', 'c'])

    expect(FakeWebSocket.instances[0]!.sentMessages()).toEqual([
      { type: 'subscribe', listing_ids: ['a', 'b'] },
      { type: 'subscribe', listing_ids: ['c'] },
    ])
  })

  it('only tells the server to unsubscribe once every caller interested in an id has let go', () => {
    // Two independent callers can both want the same listing live (e.g. it's
    // in the inventory grid AND currently open in the Preview Modal) -- a
    // ref count, not a Set, is what stops one caller's unsubscribe from
    // silently killing the other's subscription.
    const conn = new RealtimeConnection()
    conn.connect()
    const socket = FakeWebSocket.instances[0]!
    socket.open()

    conn.subscribe(['shared'])
    conn.subscribe(['shared'])
    expect(socket.sentMessages()).toEqual([{ type: 'subscribe', listing_ids: ['shared'] }])

    conn.unsubscribe(['shared'])
    expect(socket.sentMessages()).toEqual([{ type: 'subscribe', listing_ids: ['shared'] }])

    conn.unsubscribe(['shared'])
    expect(socket.sentMessages()).toEqual([
      { type: 'subscribe', listing_ids: ['shared'] },
      { type: 'unsubscribe', listing_ids: ['shared'] },
    ])
  })

  it('routes a parsed message to every registered listener until it unsubscribes', () => {
    const conn = new RealtimeConnection()
    conn.connect()
    const socket = FakeWebSocket.instances[0]!
    socket.open()

    const received: WsServerMessage[] = []
    const stop = conn.onMessage((message) => received.push(message))

    const bidAccepted: WsServerMessage = {
      type: 'bid_accepted',
      listing_id: 'abc',
      bid_id: 'bid-1',
      current_bid: 21500,
      bid_count: 3,
      high_bidder_is_you: false,
      accepted_at: '2026-01-01T00:00:00Z',
    }
    socket.message(bidAccepted)
    expect(received).toEqual([bidAccepted])

    stop()
    socket.message(bidAccepted)
    expect(received).toHaveLength(1)
  })

  it('silently drops an unparseable message frame instead of throwing', () => {
    const conn = new RealtimeConnection()
    conn.connect()
    const socket = FakeWebSocket.instances[0]!
    socket.open()

    const listener = vi.fn()
    conn.onMessage(listener)

    expect(() => socket.messageRaw('not json')).not.toThrow()
    expect(listener).not.toHaveBeenCalled()
  })

  it('reconnects after an unexpected close, backing off exponentially', () => {
    vi.useFakeTimers()
    const conn = new RealtimeConnection()
    conn.connect()
    FakeWebSocket.instances[0]!.close()

    vi.advanceTimersByTime(999)
    expect(FakeWebSocket.instances).toHaveLength(1)
    vi.advanceTimersByTime(1)
    expect(FakeWebSocket.instances).toHaveLength(2)

    FakeWebSocket.instances[1]!.close()
    vi.advanceTimersByTime(1999)
    expect(FakeWebSocket.instances).toHaveLength(2)
    vi.advanceTimersByTime(1)
    expect(FakeWebSocket.instances).toHaveLength(3)
  })

  it('re-subscribes to every previously-wanted id once a reconnected socket opens', () => {
    vi.useFakeTimers()
    const conn = new RealtimeConnection()
    conn.connect()
    FakeWebSocket.instances[0]!.open()
    conn.subscribe(['a', 'b'])

    FakeWebSocket.instances[0]!.close()
    vi.advanceTimersByTime(1000)
    const reconnected = FakeWebSocket.instances[1]!
    reconnected.open()

    expect(reconnected.sentMessages()).toEqual([{ type: 'subscribe', listing_ids: ['a', 'b'] }])
  })

  it('does not schedule a reconnect after an explicit disconnect', () => {
    vi.useFakeTimers()
    const conn = new RealtimeConnection()
    conn.connect()
    FakeWebSocket.instances[0]!.open()

    conn.disconnect()

    vi.advanceTimersByTime(60_000)
    expect(FakeWebSocket.instances).toHaveLength(1)
  })
})
