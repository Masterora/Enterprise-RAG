export function truncateText(text: string, maxLength: number) {
  const normalized = text.trim()
  const runes = [...normalized]
  if (runes.length <= maxLength) {
    return normalized
  }
  return `${runes.slice(0, maxLength).join('')}...`
}

export function buildSessionTitle(question: string) {
  return truncateText(question, 18)
}
