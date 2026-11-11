package domain

import "marketmind/api/pkg/alerts"

type Alert struct {
	AlertID             int64
	Type                alerts.Kind
	MarketID            int64
	TelegramUserID      int64
	Direction           alerts.Direction
	ReferencePrice      string
	Value               string
	IsNotified          bool
	NotificationEnabled bool
}
