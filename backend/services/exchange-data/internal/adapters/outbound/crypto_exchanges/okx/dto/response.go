package dto

type ExchangePrice struct {
	Symbol string `json:"instId"`
	Price  string `json:"last"`
}

type TickersResponse struct {
	Code    string          `json:"code"`
	Message string          `json:"msg"`
	Data    []ExchangePrice `json:"data"`
}
