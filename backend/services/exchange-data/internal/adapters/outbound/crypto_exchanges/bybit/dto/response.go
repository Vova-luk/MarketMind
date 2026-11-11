package dto

type ExchangePrice struct {
	Symbol string `json:"symbol"`
	Price  string `json:"lastPrice"`
}

type TickersResponse struct {
	RetCode int    `json:"retCode"`
	RetMsg  string `json:"retMsg"`
	Result  struct {
		List []ExchangePrice `json:"list"`
	} `json:"result"`
}
