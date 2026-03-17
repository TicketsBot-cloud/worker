package i18n

type Locale struct {
	IsoShortCode       string
	IsoLongCode        string
	FlagEmoji          string
	EnglishName        string
	LocalName          string
	DiscordLocale      *string
	ParentIsoShortCode *string
	Messages           map[MessageId]string
	Coverage           int
}

var LocaleEnglish = &Locale{
	IsoShortCode:  "en",
	IsoLongCode:   "en-GB",
	FlagEmoji:     "🏴󠁧󠁢󠁥󠁮󠁧󠁿",
	EnglishName:   "English",
	LocalName:     "English",
	DiscordLocale: ptr("en-US"),
}

var Locales = []*Locale{
	{
		IsoShortCode:  "ar",
		IsoLongCode:   "ar-SA",
		FlagEmoji:     "🇸🇦",
		EnglishName:   "Arabic",
		LocalName:     "اَلْعَرَبِيَّةُ",
		DiscordLocale: nil,
	},
	{
		IsoShortCode:  "bg",
		IsoLongCode:   "bg-BG",
		FlagEmoji:     "🇧🇬",
		EnglishName:   "Bulgarian",
		LocalName:     "български",
		DiscordLocale: ptr("bg"),
	},
	{
		IsoShortCode:  "ca",
		IsoLongCode:   "ca-ES",
		FlagEmoji:     "🇪🇸",
		EnglishName:   "Catalan",
		LocalName:     "Català",
		DiscordLocale: nil,
	},
	{
		IsoShortCode:       "ch",
		IsoLongCode:        "de-CH",
		FlagEmoji:          "🇨🇭",
		EnglishName:        "Swiss German",
		LocalName:          "Schwiizerdütsch",
		DiscordLocale:      nil,
		ParentIsoShortCode: ptr("de"),
	},
	{
		IsoShortCode:  "cz",
		IsoLongCode:   "cs-CZ",
		FlagEmoji:     "🇨🇿",
		EnglishName:   "Czech",
		LocalName:     "Čeština",
		DiscordLocale: ptr("cs"),
	},
	{
		IsoShortCode:  "dk",
		IsoLongCode:   "da-DK",
		FlagEmoji:     "🇩🇰",
		EnglishName:   "Danish",
		LocalName:     "Dansk",
		DiscordLocale: ptr("da"),
	},
	{
		IsoShortCode:  "de",
		IsoLongCode:   "de-DE",
		FlagEmoji:     "🇩🇪",
		EnglishName:   "German",
		LocalName:     "Deutsch",
		DiscordLocale: ptr("de"),
	},
	{
		IsoShortCode:  "el",
		IsoLongCode:   "el-GR",
		FlagEmoji:     "🇬🇷",
		EnglishName:   "Greek",
		LocalName:     "Ελληνικά",
		DiscordLocale: ptr("el"),
	},
	LocaleEnglish,
	{
		IsoShortCode:  "eo",
		IsoLongCode:   "eo-UY",
		FlagEmoji:     "🌎",
		EnglishName:   "Esperanto",
		LocalName:     "Esperanto",
		DiscordLocale: nil,
	},
	{
		IsoShortCode:  "es",
		IsoLongCode:   "es-ES",
		FlagEmoji:     "🇪🇸",
		EnglishName:   "Spanish",
		LocalName:     "Español",
		DiscordLocale: ptr("es-ES"),
	},
	{
		IsoShortCode:  "fi",
		IsoLongCode:   "fi-FI",
		FlagEmoji:     "🇫🇮",
		EnglishName:   "Finnish",
		LocalName:     "Suomi",
		DiscordLocale: ptr("fi"),
	},
	{
		IsoShortCode:  "fr",
		IsoLongCode:   "fr-FR",
		FlagEmoji:     "🇫🇷",
		EnglishName:   "French",
		LocalName:     "Français",
		DiscordLocale: ptr("fr"),
	},
	{
		IsoShortCode:  "he",
		IsoLongCode:   "he-IL",
		FlagEmoji:     "🇮🇱",
		EnglishName:   "Hebrew",
		LocalName:     "עברית",
		DiscordLocale: nil,
	},
	{
		IsoShortCode:  "hi",
		IsoLongCode:   "hi-IN",
		FlagEmoji:     "🇮🇳",
		EnglishName:   "Hindi",
		LocalName:     "हिन्दी",
		DiscordLocale: ptr("hi"),
	},
	{
		IsoShortCode:  "hu",
		IsoLongCode:   "hu-HU",
		FlagEmoji:     "🇭🇺",
		EnglishName:   "Hungarian",
		LocalName:     "Magyar",
		DiscordLocale: ptr("hu"),
	},
	{
		IsoShortCode:  "hr",
		IsoLongCode:   "hr-HR",
		FlagEmoji:     "🇭🇷",
		EnglishName:   "Croatian",
		LocalName:     "Hrvatski",
		DiscordLocale: ptr("hr"),
	},
	{
		IsoShortCode:  "id",
		IsoLongCode:   "id-ID",
		FlagEmoji:     "🇮🇩",
		EnglishName:   "Indonesian",
		LocalName:     "Bahasa Indonesia",
		DiscordLocale: ptr("id"),
	},
	{
		IsoShortCode:  "it",
		IsoLongCode:   "it-IT",
		FlagEmoji:     "🇮🇹",
		EnglishName:   "Italian",
		LocalName:     "Italiano",
		DiscordLocale: ptr("it"),
	},
	{
		IsoShortCode:  "jp",
		IsoLongCode:   "ja-JP",
		FlagEmoji:     "🇯🇵",
		EnglishName:   "Japanese",
		LocalName:     "日本語",
		DiscordLocale: ptr("ja"),
	},
	{
		IsoShortCode:  "kr",
		IsoLongCode:   "ko-KR",
		FlagEmoji:     "🇰🇷",
		EnglishName:   "Korean",
		LocalName:     "한국어",
		DiscordLocale: ptr("ko"),
	},
	{
		IsoShortCode:  "lt",
		IsoLongCode:   "lt-LT",
		FlagEmoji:     "🇱🇹",
		EnglishName:   "Lithuanian",
		LocalName:     "Lietuviškai",
		DiscordLocale: ptr("lt"),
	},
	{
		IsoShortCode:  "nl",
		IsoLongCode:   "nl-NL",
		FlagEmoji:     "🇳🇱",
		EnglishName:   "Dutch",
		LocalName:     "Nederlands",
		DiscordLocale: ptr("nl"),
	},
	{
		IsoShortCode:  "no",
		IsoLongCode:   "no-NO",
		FlagEmoji:     "🇳🇴",
		EnglishName:   "Norwegian",
		LocalName:     "Norsk",
		DiscordLocale: ptr("no"),
	},
	{
		IsoShortCode:  "pl",
		IsoLongCode:   "pl-PL",
		FlagEmoji:     "🇵🇱",
		EnglishName:   "Polish",
		LocalName:     "Polski",
		DiscordLocale: ptr("pl"),
	},
	{
		IsoShortCode:       "br",
		IsoLongCode:        "pt-BR",
		FlagEmoji:          "🇧🇷",
		EnglishName:        "Portuguese (Brazil)",
		LocalName:          "Português do Brasil",
		DiscordLocale:      ptr("pt-BR"),
		ParentIsoShortCode: ptr("pt"),
	},
	{
		IsoShortCode:  "pt",
		IsoLongCode:   "pt-PT",
		FlagEmoji:     "🇵🇹",
		EnglishName:   "Portuguese",
		LocalName:     "Português",
		DiscordLocale: nil,
	},
	{
		IsoShortCode:  "ro",
		IsoLongCode:   "ro-RO",
		FlagEmoji:     "🇷🇴",
		EnglishName:   "Romanian",
		LocalName:     "Română",
		DiscordLocale: ptr("ro"),
	},
	{
		IsoShortCode:  "ru",
		IsoLongCode:   "ru-RU",
		FlagEmoji:     "🇷🇺",
		EnglishName:   "Russian",
		LocalName:     "Pусский",
		DiscordLocale: ptr("ru"),
	},
	{
		IsoShortCode:  "sk",
		IsoLongCode:   "sk-SK",
		FlagEmoji:     "🇸🇰",
		EnglishName:   "Slovak",
		LocalName:     "Slovenský",
		DiscordLocale: nil,
	},
	{
		IsoShortCode:  "sl",
		IsoLongCode:   "sl-SI",
		FlagEmoji:     "🇸🇮",
		EnglishName:   "Slovenian",
		LocalName:     "Slovenščina",
		DiscordLocale: nil,
	},
	{
		IsoShortCode:  "sr",
		IsoLongCode:   "sr-SP",
		FlagEmoji:     "🇷🇸",
		EnglishName:   "Serbian",
		LocalName:     "Српски",
		DiscordLocale: nil,
	},
	{
		IsoShortCode:  "sv",
		IsoLongCode:   "sv-SE",
		FlagEmoji:     "🇸🇪",
		EnglishName:   "Swedish",
		LocalName:     "Svenska",
		DiscordLocale: ptr("sv-SE"),
	},
	{
		IsoShortCode:  "th",
		IsoLongCode:   "th-TH",
		FlagEmoji:     "🇹🇭",
		EnglishName:   "Thai",
		LocalName:     "ไทย",
		DiscordLocale: ptr("th"),
	},
	{
		IsoShortCode:  "tr",
		IsoLongCode:   "tr-TR",
		FlagEmoji:     "🇹🇷",
		EnglishName:   "Turkish",
		LocalName:     "Türkçe",
		DiscordLocale: ptr("tr"),
	},
	{
		IsoShortCode:  "ua",
		IsoLongCode:   "uk-UA",
		FlagEmoji:     "🇺🇦",
		EnglishName:   "Ukrainian",
		LocalName:     "Українська",
		DiscordLocale: ptr("uk"),
	},
	{
		IsoShortCode:  "vn",
		IsoLongCode:   "vi-VN",
		FlagEmoji:     "🇻🇳",
		EnglishName:   "Vietnamese",
		LocalName:     "Tiếng Việt",
		DiscordLocale: nil,
	},
	{
		IsoShortCode:  "cy",
		IsoLongCode:   "cy-GB",
		FlagEmoji:     "🏴󠁧󠁢󠁷󠁬󠁳󠁿",
		EnglishName:   "Welsh",
		LocalName:     "Cymraeg",
		DiscordLocale: nil,
	},
	{
		IsoShortCode:  "cn",
		IsoLongCode:   "zh-CN",
		FlagEmoji:     "🇨🇳",
		EnglishName:   "Chinese",
		LocalName:     "中文",
		DiscordLocale: ptr("zh-CN"),
	},
	{
		IsoShortCode:       "tw",
		IsoLongCode:        "zh-TW",
		FlagEmoji:          "🇹🇼",
		EnglishName:        "Chinese (Taiwan)",
		LocalName:          "繁體中文",
		DiscordLocale:      ptr("zh-TW"),
		ParentIsoShortCode: ptr("cn"),
	},
}

var (
	MappedByIsoShortCode = make(map[string]*Locale)

	// DiscordLocales https://discord.com/developers/docs/reference#locales
	// Discord locale (e.g. bg) -> Locale
	DiscordLocales = make(map[string]*Locale)
)

func SeedIndices() {
	for _, locale := range Locales {
		MappedByIsoShortCode[locale.IsoShortCode] = locale

		if locale.DiscordLocale != nil {
			DiscordLocales[*locale.DiscordLocale] = locale
		}
	}
}

func Init() {
	SeedIndices()
	LoadMessages()
	SeedCoverage()
}

func ptr[T any](t T) *T {
	return &t
}
