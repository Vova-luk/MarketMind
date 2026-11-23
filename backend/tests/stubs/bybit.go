package main

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

type bybitStub struct {
	mu     sync.RWMutex
	prices map[string]string
	fault  string
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
	mux.HandleFunc("GET /v5/market/instruments-info", stub.listInstruments)
	mux.HandleFunc("PUT /ticker-fault", stub.setTickerFault)
	mux.HandleFunc("PUT /prices/{symbol}", stub.setPrice)
	mux.HandleFunc("DELETE /prices/{symbol}", stub.deletePrice)
}

func (stub *bybitStub) listTickers(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("category") != "spot" {
		http.Error(w, "expected category=spot", http.StatusBadRequest)
		return
	}
	stub.mu.RLock()
	fault := stub.fault
	tickers := make([]bybitTicker, 0, len(stub.prices))
	for symbol, price := range stub.prices {
		tickers = append(tickers, bybitTicker{Symbol: symbol, LastPrice: price})
	}
	stub.mu.RUnlock()
	switch fault {
	case "unavailable":
		http.Error(w, "Bybit unavailable", http.StatusServiceUnavailable)
		return
	case "timeout":
		timer := time.NewTimer(2 * time.Second)
		defer timer.Stop()
		select {
		case <-r.Context().Done():
		case <-timer.C:
			http.Error(w, "Bybit timed out", http.StatusGatewayTimeout)
		}
		return
	}
	writeJSON(w, struct {
		RetCode int             `json:"retCode"`
		RetMsg  string          `json:"retMsg"`
		Result  bybitTickerList `json:"result"`
	}{RetCode: 0, RetMsg: "OK", Result: bybitTickerList{Category: "spot", List: tickers}})
}

func (stub *bybitStub) listInstruments(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("category") != "spot" || r.URL.Query().Get("status") != "Trading" {
		http.Error(w, "expected spot trading instruments", http.StatusBadRequest)
		return
	}
	type instrument struct {
		Symbol    string `json:"symbol"`
		BaseCoin  string `json:"baseCoin"`
		QuoteCoin string `json:"quoteCoin"`
		Status    string `json:"status"`
	}
	stub.mu.RLock()
	instruments := make([]instrument, 0, len(stub.prices))
	for symbol := range stub.prices {
		if base, ok := strings.CutSuffix(symbol, "USDT"); ok && base != "" {
			instruments = append(instruments, instrument{Symbol: symbol, BaseCoin: base, QuoteCoin: "USDT", Status: "Trading"})
		}
	}
	stub.mu.RUnlock()
	writeJSON(w, struct {
		RetCode int `json:"retCode"`
		Result  struct {
			List []instrument `json:"list"`
		} `json:"result"`
	}{Result: struct {
		List []instrument `json:"list"`
	}{List: instruments}})
}

func (stub *bybitStub) setTickerFault(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Mode string `json:"mode"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4*1024))
	if err := decoder.Decode(&body); err != nil || decoder.Decode(new(json.RawMessage)) != io.EOF {
		http.Error(w, "expected a fault mode", http.StatusBadRequest)
		return
	}
	switch body.Mode {
	case "healthy", "unavailable", "timeout":
	default:
		http.Error(w, "unknown fault mode", http.StatusBadRequest)
		return
	}
	stub.mu.Lock()
	stub.fault = body.Mode
	stub.mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
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
