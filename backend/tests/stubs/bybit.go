package main

import "net/http"

func serveBybitTickers(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("category") != "spot" {
		http.Error(w, "expected category=spot", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	const tickers = `{"retCode":0,"retMsg":"OK","result":{"category":"spot","list":[{"symbol":"IT06USDT","lastPrice":"100"}]}}`
	_, _ = w.Write([]byte(tickers))
}
