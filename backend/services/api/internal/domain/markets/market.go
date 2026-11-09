package markets

type Instrument struct {
	ExternalID string
	BaseAsset  string
	QuoteAsset string
	Price      *string
}
