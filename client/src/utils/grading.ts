import type { TitleStatus } from '@/types/vehicle'
import type { GradeVariant } from '@/types/listing'

/** Bucketed, not continuous — resolves the design-handoff doc's own open question in favor of what the approved mock already decided. */
export function gradeVariant(grade: number): GradeVariant {
  if (grade < 3) return 'poor'
  if (grade < 4) return 'fair'
  return 'good'
}

export function gradeLabel(grade: number): string {
  if (grade >= 4.5) return 'Excellent'
  if (grade >= 3.5) return 'Good'
  if (grade >= 2.5) return 'Fair'
  if (grade >= 1.5) return 'Poor'
  return 'Very Poor'
}

export function gradeTooltip(grade: number): string {
  return `Condition Grade ${grade.toFixed(1)} / 5.0 — ${gradeLabel(grade)}. Scale runs 1.0 (worst) to 5.0 (perfect).`
}

export function titleLabel(status: TitleStatus): string {
  if (status === 'clean') return 'Clean Title'
  if (status === 'rebuilt') return 'Rebuilt Title'
  return 'Salvage Title'
}
