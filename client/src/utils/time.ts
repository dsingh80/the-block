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
