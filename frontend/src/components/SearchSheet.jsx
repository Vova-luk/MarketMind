import { useState } from 'react';
import { Search, Trash2, Bookmark } from 'lucide-react';

export default function SearchSheet({ coins, onToggleWatch }) {
  const [searchQuery, setSearchQuery] = useState('');
  const [sortAsc, setSortAsc] = useState(false);
  const [selectedExchange, setSelectedExchange] = useState('Все');

  const filteredCoins = coins
    .filter((c) => {
      const q = searchQuery.trim().toLowerCase();
      const matchesSearch =
        c.name.toLowerCase().includes(q) || c.ticker.toLowerCase().includes(q);
      const matchesExchange =
        selectedExchange === 'Все' || c.exchange === selectedExchange;
      return matchesSearch && matchesExchange;
    })
    .sort((a, b) => (sortAsc ? a.price - b.price : b.price - a.price));

  return (
    <div className="mt-6 space-y-4">
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

      <div className="flex items-center justify-between text-xs">
        <div className="flex items-center gap-2">
          <button
            onClick={() => setSortAsc(!sortAsc)}
            className="px-3 py-1.5 bg-gray-200 rounded-lg font-medium text-gray-800 cursor-pointer"
          >
            Цена {sortAsc ? '↑' : '↓'}
          </button>
          <button
            onClick={() =>
              setSelectedExchange((prev) =>
                prev === 'Все' ? 'Binance' : prev === 'Binance' ? 'Bybit' : 'Все'
              )
            }
            className="px-3 py-1.5 bg-gray-200 rounded-lg font-medium text-gray-800 cursor-pointer"
          >
            {selectedExchange === 'Все' ? 'Биржа' : selectedExchange}
          </button>
        </div>

        <button
          onClick={() => {
            setSearchQuery('');
            setSelectedExchange('Все');
            setSortAsc(false);
          }}
          className="p-1.5 bg-gray-200 rounded-lg text-gray-700 cursor-pointer"
          title="Сбросить фильтры"
        >
          <Trash2 className="w-4 h-4" />
        </button>
      </div>

      <div className="space-y-3 pt-2">
        {filteredCoins.length === 0 ? (
          <div className="text-center py-16 text-gray-400 text-sm">
            Ничего не найдено
          </div>
        ) : (
          filteredCoins.map((coin) => (
            <div
              key={coin.id}
              className="flex items-center justify-between px-4 py-3.5 bg-white rounded-2xl shadow-[0_2px_10px_rgba(0,0,0,0.08)] border border-gray-100"
            >
              <span className="text-base text-black">
                {coin.name} ({coin.ticker})
              </span>

              <div className="flex items-center gap-4">
                <span className="text-base font-medium text-black">
                  {coin.price.toLocaleString('ru-RU')}$
                </span>
                <button
                  onClick={() => onToggleWatch(coin.id)}
                  className="p-1 cursor-pointer"
                >
                  <Bookmark
                    className={`w-6 h-6 ${
                      coin.isWatched
                        ? 'fill-orange-500 text-orange-500'
                        : 'text-gray-700'
                    }`}
                  />
                </button>
              </div>
            </div>
          ))
        )}
      </div>
    </div>
  );
}