package command

type Category string

const (
	General    Category = "ℹ️ General"
	Tickets    Category = "📩 Tickets"
	Settings   Category = "🔧 Settings"
	Tags       Category = "✍️ Tags"
	Statistics Category = "📈 Statistics"
)

var Categories = []Category{
	General,
	Tickets,
	Settings,
	Tags,
	Statistics,
}

func (c Category) ToRawString() string {
	switch c {
	case General:
		return "general"
	case Tickets:
		return "tickets"
	case Settings:
		return "settings"
	case Tags:
		return "tags"
	case Statistics:
		return "statistics"
	default:
		return string(c)
	}
}
