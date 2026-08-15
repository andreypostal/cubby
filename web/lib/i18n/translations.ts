import en from "@/lib/i18n/locales/en.json"
import pt from "@/lib/i18n/locales/pt.json"

export const locales = ["en", "pt"] as const

export type Locale = (typeof locales)[number]

export const defaultLocale: Locale = "en"

export const localeNames: Record<Locale, string> = {
  en: "English",
  pt: "Português",
}

export type TranslationKey = keyof typeof en

const translations = { en, pt } satisfies Record<
  Locale,
  Record<TranslationKey, string>
>

export default translations
