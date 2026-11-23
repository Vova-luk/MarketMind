package main

import (
	"encoding/json"
	"io"
	"net/http"
	"sync"
)

type bybitStub struct {
	mu     sync.RWMutex
	prices map[string]string
}

type bybitTicker struct {
	Symbol    string `json:"symbol"`
	LastPrice string `json:"lastPrice"`
}

type bybitTickerList struct {
	Category string        `json:"category"`
	List     []bybitTicker `json:"list"`
}

func registerBybit(mux *http.ServeMux) {
	stub := &bybitStub{prices: map[string]string{"IT06USDT": "100"}}
	mux.HandleFunc("GET /v5/market/tickers", stub.listTickers)
	mux.HandleFunc("PUT /prices/{symbol}", stub.setPrice)
	mux.HandleFunc("DELETE /prices/{symbol}", stub.deletePrice)
}

func (stub *bybitStub) listTickers(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("category") != "spot" {
		http.Error(w, "expected category=spot", http.StatusBadRequest)
		return
	}
	stub.mu.RLock()
	tickers := make([]bybitTicker, 0, len(stub.prices))
	for symbol, price := range stub.prices {
		tickers = append(tickers, bybitTicker{Symbol: symbol, LastPrice: price})
	}
	stub.mu.RUnlock()
	writeJSON(w, struct {
		RetCode int             `json:"retCode"`
		RetMsg  string          `json:"retMsg"`
		Result  bybitTickerList `json:"result"`
	}{RetCode: 0, RetMsg: "OK", Result: bybitTickerList{Category: "spot", List: tickers}})
}

func (stub *bybitStub) setPrice(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Price string `json:"price"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4*1024))
	if err := decoder.Decode(&body); err != nil || decoder.Decode(new(json.RawMessage)) != io.EOF || body.Price == "" {
		http.Error(w, "expected a price string", http.StatusBadRequest)
		return
	}
	stub.mu.Lock()
	stub.prices[r.PathValue("symbol")] = body.Price
	stub.mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

func (stub *bybitStub) deletePrice(w http.ResponseWriter, r *http.Request) {
	stub.mu.Lock()
	delete(stub.prices, r.PathValue("symbol"))
	stub.mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}
