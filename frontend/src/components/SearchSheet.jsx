import { useState, useEffect, useRef } from 'react';
import { Search, Trash2, Bookmark, ChevronDown, Check } from 'lucide-react';
import { searchMarketsApi } from '../api/marketApi';
import { triggerHaptic } from '../utils/telegram';

export default function SearchSheet({
  exchanges,
  subscriptions,
  onToggleSubscription,
}) {
  const [searchQuery, setSearchQuery] = useState('');
  const [sortAsc, setSortAsc] = useState(false);

  const [selectedExchangeId, setSelectedExchangeId] = useState(() =>
    exchanges.length > 0 ? exchanges[0].id : 1
  );
  const [isDropdownOpen, setIsDropdownOpen] = useState(false);
  const [markets, setMarkets] = useState([]);
  const [loading, setLoading] = useState(false);

  const dropdownRef = useRef(null);

  useEffect(() => {
    if (
      exchanges.length > 0 &&
      !exchanges.some((e) => e.id === selectedExchangeId)
    ) {
      setSelectedExchangeId(exchanges[0].id);
    }
  }, [exchanges, selectedExchangeId]);

  useEffect(() => {
    const handleClickOutside = (e) => {
      if (dropdownRef.current && !dropdownRef.current.contains(e.target)) {
        setIsDropdownOpen(false);
      }
    };
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  useEffect(() => {
    if (!selectedExchangeId) return;

    let active = true;
    const loadMarkets = async () => {
      setLoading(true);
      try {
        const res = await searchMarketsApi(selectedExchangeId, searchQuery);
        if (active) {
          setMarkets(res.items || []);
          setLoading(false);
        }
      } catch (e) {
        if (active) setLoading(false);
      }
    };

    const timer = setTimeout(loadMarkets, 250);
    return () => {
      active = false;
      clearTimeout(timer);
    };
  }, [selectedExchangeIds, searchQuery]);

  const handleToggleExchange = (exchangeId) => {
    triggerHaptic('light');
    setSelectedExchangeIds((prev) => {
      const isSelected = prev.includes(exchangeId);
      if (isSelected) {
        if (prev.length === 1) {
          return prev;
        }
        return prev.filter((id) => id !== exchangeId);
      } else {
        return [...prev, exchangeId];
      }
    });
  };

  const getExchangeName = (exchangeId) => {
    return exchanges.find((e) => e.id === exchangeId)?.name || 'Биржа';
  };

  const handleSelectExchange = (exchangeId) => {
    triggerHaptic('light');
    setSelectedExchangeId(exchangeId);
    setIsDropdownOpen(false);
  };

  const getExchangeName = (exchangeId) => {
    return exchanges.find((e) => e.id === exchangeId)?.name || 'Биржа';
  };

  const findExistingSubscription = (market) => {
    const exName = getExchangeName(market.exchange_id);
    return subscriptions.find(
      (s) =>
        s.base_asset === market.base_asset &&
        s.quote_asset === market.quote_asset &&
        s.exchange_name === exName
    );
  };

  const sortedMarkets = [...markets].sort((a, b) => {
    const pA = Number(a.price || 0);
    const pB = Number(b.price || 0);
    return sortAsc ? pA - pB : pB - pA;
  });

  return (
    <div className="mt-6 space-y-4">
      {/* поле поиска */}
      <div className="relative">
        <input
          type="text"
          value={searchQuery}
          onChange={(e) => setSearchQuery(e.target.value)}
          placeholder="Поиск..."
          className="w-full pl-4 pr-10 py-2.5 rounded-xl border border-gray-300 text-base focus:outline-none focus:border-blue-500"
        />
        <Search className="w-5 h-5 text-gray-400 absolute right-3 top-3" />
      </div>

      {/* фильтра */}
      <div className="flex items-center justify-between text-xs relative">
        <div className="flex items-center gap-2">
          <button
            onClick={() => setSortAsc(!sortAsc)}
            className="px-3 py-1.5 bg-gray-200 hover:bg-gray-300 rounded-lg font-medium text-gray-800 cursor-pointer transition-colors"
          >
            Цена {sortAsc ? '↑' : '↓'}
          </button>

          <div className="relative" ref={dropdownRef}>
            <button
              onClick={() => setIsDropdownOpen((prev) => !prev)}
              className="px-3 py-1.5 bg-gray-200 hover:bg-gray-300 rounded-lg font-medium text-gray-800 flex items-center gap-1 cursor-pointer transition-colors"
            >
              <span>{getExchangeName(selectedExchangeId)}</span>
              <ChevronDown
                className={`w-3.5 h-3.5 transition-transform duration-150 ${
                  isDropdownOpen ? 'rotate-180' : ''
                }`}
              />
            </button>

            {isDropdownOpen && (
              <div className="absolute left-0 mt-1.5 w-44 bg-white border border-gray-200 rounded-xl shadow-lg py-1 z-40 overflow-hidden">
                {exchanges.map((exchange) => {
                  const isSelected = exchange.id === selectedExchangeId;

                  return (
                    <button
                      key={exchange.id}
                      type="button"
                      onClick={() => handleSelectExchange(exchange.id)}
                      className={`w-full flex items-center justify-between px-3 py-2 text-sm text-left cursor-pointer transition-colors ${
                        isSelected
                          ? 'bg-blue-50 text-blue-600 font-medium'
                          : 'text-gray-800 hover:bg-gray-50'
                      }`}
                    >
                      <span>{exchange.name}</span>
                      {isSelected && <Check className="w-4 h-4 text-blue-600" />}
                    </button>
                  );
                })}
              </div>
            )}
          </div>
        </div>

        {/* сброс фильтров */}
        <button
          onClick={() => {
            setSearchQuery('');
            setSortAsc(false);
            if (exchanges.length > 0) {
              setSelectedExchangeId(exchanges[0].id);
            }
            setIsDropdownOpen(false);
          }}
          className="p-1.5 bg-gray-200 hover:bg-gray-300 rounded-lg text-gray-700 cursor-pointer transition-colors"
          title="Сбросить фильтры"
        >
          <Trash2 className="w-4 h-4" />
        </button>
      </div>

      {/* список монет */}
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
                <div className="flex flex-col">
                  <span className="text-base text-black">
                    {market.display_name}
                  </span>
                  <span className="text-xs text-gray-400">
                    {getExchangeName(market.exchange_id)}
                  </span>
                </div>

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