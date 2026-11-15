// Retained across modal and library remounts in this browser tab's JS lifetime.
// No automatic rotation on failure/success: unchanged parameters refer to one job.
const keys = new Map<string, string>()
export function slicerSubmissionKey(payload: Record<string, unknown>): string {
  const fingerprint = JSON.stringify(payload)
  let key = keys.get(fingerprint)
  if (!key) { key = crypto.randomUUID(); keys.set(fingerprint, key) }
  return key
}
export function forgetSlicerSubmission(key: string): void {
  for (const [payload, retained] of keys) if (retained === key) keys.delete(payload)
}
