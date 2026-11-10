package domain

type Alert struct {
	AlertID             int64
	Type                string
	MarketID            int64
	TelegramUserID      int64
	Direction           string
	ReferencePrice      string
	Value               string
	IsNotified          bool
	NotificationEnabled bool
}
