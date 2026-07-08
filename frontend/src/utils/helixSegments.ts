/** Expand `all` selections into concrete version lists for the DNA helix. */
export function resolveHelixVersions(
  selected: string[] | undefined,
  available?: string[]
): string[] {
  if (!selected?.length) return []
  if (selected.includes('all')) {
    return available?.length ? [...available] : []
  }
  return selected.filter((v) => v !== 'all')
}
