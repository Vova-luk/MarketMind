import { useState, useEffect } from 'react';
import { Search, Trash2, Bookmark } from 'lucide-react';
import { searchMarketsApi } from '../api/marketApi';

export default function SearchSheet({
  exchanges,
  subscriptions,
  onToggleSubscription,
}) {
  const [searchQuery, setSearchQuery] = useState('');
  const [sortAsc, setSortAsc] = useState(false);
  const [selectedExchangeId, setSelectedExchangeId] = useState(
    exchanges[0]?.id || 1
  );
  const [markets, setMarkets] = useState([]);
  const [loading, setLoading] = useState(false);

  const currentExchange = exchanges.find((e) => e.id === selectedExchangeId);

  useEffect(() => {
    let active = true;
    const loadMarkets = async () => {
      setLoading(true);
      const data = await searchMarketsApi(selectedExchangeId, searchQuery);
      if (active) {
        setMarkets(data.items || []);
        setLoading(false);
      }
    };
    const timer = setTimeout(loadMarkets, 250);
    return () => {
      active = false;
      clearTimeout(timer);
    };
  }, [selectedExchangeId, searchQuery]);

  const cycleExchange = () => {
    if (exchanges.length === 0) return;
    const idx = exchanges.findIndex((e) => e.id === selectedExchangeId);
    const next = exchanges[(idx + 1) % exchanges.length];
    setSelectedExchangeId(next.id);
  };

  
  const findExistingSubscription = (market) => {
    return subscriptions.find(
      (s) =>
        s.base_asset === market.base_asset &&
        s.quote_asset === market.quote_asset &&
        s.exchange_name === currentExchange?.name
    );
  };

  const sortedMarkets = [...markets].sort((a, b) => {
    const pA = Number(a.price || 0);
    const pB = Number(b.price || 0);
    return sortAsc ? pA - pB : pB - pA;
  });

  return (
    <div className="mt-6 space-y-4">
      <div className="relative">
        <input
          type="text"
          value={searchQuery}
          onChange={(e) => setSearchQuery(e.target.value)}
          placeholder="Поиск (например, BTC)..."
          className="w-full pl-4 pr-10 py-2.5 rounded-xl border border-gray-300 text-base focus:outline-none focus:border-blue-500"
        />
        <Search className="w-5 h-5 text-gray-400 absolute right-3 top-3" />
      </div>

      <div className="flex items-center justify-between text-xs">
        <div className="flex items-center gap-2">
          <button
            onClick={() => setSortAsc(!sortAsc)}
            className="px-3 py-1.5 bg-gray-200 rounded-lg font-medium text-gray-800 cursor-pointer"
          >
            Цена {sortAsc ? '↑' : '↓'}
          </button>
          <button
            onClick={cycleExchange}
            className="px-3 py-1.5 bg-gray-200 rounded-lg font-medium text-gray-800 cursor-pointer"
          >
            {currentExchange ? currentExchange.name : 'Биржа'}
          </button>
        </div>

        <button
          onClick={() => {
            setSearchQuery('');
            setSortAsc(false);
          }}
          className="p-1.5 bg-gray-200 rounded-lg text-gray-700 cursor-pointer"
          title="Сбросить фильтры"
        >
          <Trash2 className="w-4 h-4" />
        </button>
      </div>

      <div className="space-y-3 pt-2">
        {loading ? (
          <div className="text-center py-12 text-gray-400 text-sm">
            Загрузка рынков...
          </div>
        ) : sortedMarkets.length === 0 ? (
          <div className="text-center py-16 text-gray-400 text-sm">
            Ничего не найдено
          </div>
        ) : (
          sortedMarkets.map((market) => {
            const existingSub = findExistingSubscription(market);
            const isWatched = Boolean(existingSub);
            const priceLabel =
              market.price !== null
                ? `${Number(market.price).toLocaleString('ru-RU')} ${market.quote_asset}`
                : 'Н/Д';

            return (
              <div
                key={`${market.exchange_id}-${market.external_id}`}
                className="flex items-center justify-between px-4 py-3.5 bg-white rounded-2xl shadow-[0_2px_10px_rgba(0,0,0,0.08)] border border-gray-100"
              >
                <span className="text-base text-black">
                  {market.display_name}
                </span>

                <div className="flex items-center gap-4">
                  <span className="text-base font-medium text-black">
                    {priceLabel}
                  </span>
                  <button
                    onClick={() => onToggleSubscription(market, existingSub)}
                    className="p-1 cursor-pointer"
                  >
                    <Bookmark
                      className={`w-6 h-6 ${
                        isWatched
                          ? 'fill-orange-500 text-orange-500'
                          : 'text-gray-700'
                      }`}
                    />
                  </button>
                </div>
              </div>
            );
          })
        )}
      </div>
    </div>
  );
}