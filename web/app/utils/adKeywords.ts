const SUBJECT_FAMILIES = new Set(["Topic", "Framework"]);
const MAX_KEYWORDS = 5;

/**
 * Convert PyPI trove classifiers into keywords for EthicalAds contextual
 * targeting. Only Topic and Framework families describe subject matter; the
 * most specific (last) segment of each is used.
 */
export function deriveAdKeywords(classifiers: string[] | null | undefined): string[] {
  if (!classifiers?.length) return [];

  const keywords: string[] = [];
  const seen = new Set<string>();

  for (const classifier of classifiers) {
    const segments = classifier.split("::").map((s) => s.trim());
    if (segments.length < 2) continue;
    if (!SUBJECT_FAMILIES.has(segments[0]!)) continue;

    const keyword = segments[segments.length - 1]!.toLowerCase();
    if (!keyword || seen.has(keyword)) continue;

    seen.add(keyword);
    keywords.push(keyword);
    if (keywords.length === MAX_KEYWORDS) break;
  }

  return keywords;
}
