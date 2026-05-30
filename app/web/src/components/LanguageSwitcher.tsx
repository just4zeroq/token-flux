import { useTranslation } from 'react-i18next'

const FLAGS: Record<string, string> = {
  en: '🇬🇧',
  zh: '🇨🇳',
  ja: '🇯🇵',
  ko: '🇰🇷',
  vi: '🇻🇳',
}

export function LanguageSwitcher() {
  const { i18n } = useTranslation()

  return (
    <select
      value={i18n.language}
      onChange={(e) => i18n.changeLanguage(e.target.value)}
      className="h-7 rounded-md border border-input bg-card px-1.5 text-xs focus:outline-none focus:ring-2 focus:ring-ring"
      title="Language"
    >
      {Object.entries(FLAGS).map(([code, flag]) => (
        <option key={code} value={code}>{flag}</option>
      ))}
    </select>
  )
}
