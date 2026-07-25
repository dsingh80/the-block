import type { Lifecycle } from '@/types/listing'

/**
 * `totalHours` >= 24 -> "{d}d {h}h". >= 1 -> "{h}h {mm}m" (minutes
 * zero-padded). Otherwise -> "{m}m" (minimum 1, so a sub-minute duration
 * doesn't render as "0m").
 */
export function formatDuration(totalHours: number): string {
  const totalMinutes = Math.round(totalHours * 60)
  const days = Math.floor(totalMinutes / (24 * 60))
  const hours = Math.floor((totalMinutes % (24 * 60)) / 60)
  const minutes = totalMinutes % 60

  if (days >= 1) {
    return `${days}d ${hours}h`
  }
  if (hours >= 1) {
    return `${hours}h ${String(minutes).padStart(2, '0')}m`
  }
  return `${Math.max(minutes, 1)}m`
}

/**
 * `hoursRemaining` follows AugmentedListing's sign convention: positive
 * hours until start (upcoming) or until end (active), negative hours since
 * end (ended).
 */
export function timeLabelFor(lifecycle: Lifecycle, hoursRemaining: number): string {
  if (lifecycle === 'upcoming') {
    return `Starts in ${formatDuration(hoursRemaining)}`
  }
  if (lifecycle === 'active') {
    return `Ends in ${formatDuration(hoursRemaining)}`
  }
  return `Ended ${formatDuration(-hoursRemaining)} ago`
}

/**
 * Short relative-time label for "how long ago", not a countdown --
 * same d/h/m bucketing as formatDuration but no floor-at-1, since an
 * elapsed duration legitimately starts at zero. `elapsedMs` is clamped to
 * >= 0 so a clock read landing a tick before its own timestamp still reads
 * "just now" instead of negative.
 */
export function timeSince(elapsedMs: number): string {
  const minutes = Math.floor(Math.max(elapsedMs, 0) / 60000)
  if (minutes < 1) return 'just now'
  if (minutes < 60) return `${minutes}m ago`
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `${hours}h ago`
  const days = Math.floor(hours / 24)
  return `${days}d ago`
}
