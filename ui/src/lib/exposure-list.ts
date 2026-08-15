// normalizeExposureList accepts the two response shapes the exposure list
// endpoint has used over time — a bare JSON array (current) and an object
// with {data|exposures} — and returns a stable array. The list page reads
// through this so a shape mismatch can never silently render an empty list.
export function normalizeExposureList(data: unknown): any[] {
  if (Array.isArray(data)) return data;
  if (data && typeof data === 'object') {
    const obj = data as Record<string, unknown>;
    if (Array.isArray(obj.data)) return obj.data;
    if (Array.isArray(obj.exposures)) return obj.exposures;
  }
  return [];
}
