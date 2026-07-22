export function currency(amount: number): string {
  return '$' + Math.round(amount).toLocaleString('en-US')
}

export function formatKm(odometerKm: number): string {
  return `${Math.round(odometerKm).toLocaleString('en-US')} km`
}

export function formatGrade(grade: number): string {
  return grade.toFixed(1)
}

export function capitalize(value: string): string {
  return value.charAt(0).toUpperCase() + value.slice(1)
}
