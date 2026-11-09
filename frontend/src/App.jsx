import { useState, useEffect } from 'react';
import {
  Plus,
  ArrowUp,
  ArrowDown,
  X,
  Search,
  Trash2,
  Bookmark,
  Bell,
  BellOff,
} from 'lucide-react';

const tg = window.Telegram?.WebApp;

const triggerHaptic = (style = 'light') => {
  try {
    tg?.HapticFeedback?.impactOccurred(style);
  } catch (e) {
  }
};

const INITIAL_COINS = [
  {
    id: 'btc',
    name: 'Bitcoin',
    ticker: 'BTC',
    price: 83781,
    change24hDollar: 243,
    change24hPercent: 0.29,
    exchange: 'Binance',
    isWatched: true,
    alerts: [
      { id: 1, targetPrice: 87970, diffLabel: '+5%', isActive: true },
      { id: 2, targetPrice: 83650, diffLabel: '-131$', isActive: false },
    ],
  },
  {
    id: 'eth',
    name: 'Etherium',
    ticker: 'ETH',
    price: 2505,
    change24hDollar: -54,
    change24hPercent: -2.16,
    exchange: 'Bybit',
    isWatched: true,
    alerts: [],
  },
  {
    id: 'zec',
    name: 'Zcach',
    ticker: 'ZEC',
    price: 1252,
    change24hDollar: -123,
    change24hPercent: -9.82,
    exchange: 'OKX',
    isWatched: true,
    alerts: [],
  },
  {
    id: 'btw',
    name: 'Bitway',
    ticker: 'BTW',
    price: 1.92,
    change24hDollar: 0.05,
    change24hPercent: 2.6,
    exchange: 'Binance',
    isWatched: false,
    alerts: [],
  },
];

export default function App() {
  const [coins, setCoins] = useState(INITIAL_COINS);
  const [activeSheet, setActiveSheet] = useState(null); 
  const [selectedCoinId, setSelectedCoinId] = useState(null);

  const [searchQuery, setSearchQuery] = useState('');
  const [sortAsc, setSortAsc] = useState(false);
  const [selectedExchange, setSelectedExchange] = useState('Все');

  const [targetPriceInput, setTargetPriceInput] = useState('');
  const [targetPercentInput, setTargetPercentInput] = useState('');
  const [inputError, setInputError] = useState('');

  useEffect(() => {
    try {
      tg?.ready();
      tg?.expand();
    } catch (e) {
      console.log('Запущено вне Telegram');
    }
  }, []);

  const userName = tg?.initDataUnsafe?.user?.first_name || 'Пользователь';
  const watchedCoins = coins.filter((coin) => coin.isWatched);
  const selectedCoin = coins.find((c) => c.id === selectedCoinId);

  const handleOpenCoin = (id) => {
    triggerHaptic('light');
    setSelectedCoinId(id);
    setTargetPriceInput('');
    setTargetPercentInput('');
    setInputError('');
    setActiveSheet('coin');
  };

  const handleOpenSearch = () => {
    triggerHaptic('medium');
    setSearchQuery('');
    setActiveSheet('search');
  };

  const closeSheet = () => {
    setActiveSheet(null);
  };

  const toggleWatchCoin = (id) => {
    triggerHaptic('light');
    setCoins((prev) =>
      prev.map((c) => (c.id === id ? { ...c, isWatched: !c.isWatched } : c))
    );
  };

  const handleAddPriceAlert = () => {
    const val = parseFloat(targetPriceInput);
    if (isNaN(val) || val <= 0 || val > 500000) {
      setInputError('Введите значение от 0 до 500 000');
      return;
    }
    setInputError('');
    triggerHaptic('medium');

    const diffDollar = Math.round(val - selectedCoin.price);
    const diffSign = diffDollar >= 0 ? '+' : '';

    const newAlert = {
      id: Date.now(),
      targetPrice: val,
      diffLabel: `${diffSign}${diffDollar}$`,
      isActive: true,
    };

    setCoins((prev) =>
      prev.map((c) =>
        c.id === selectedCoin.id
          ? { ...c, alerts: [...c.alerts, newAlert] }
          : c
      )
    );
    setTargetPriceInput('');
  };

  const handleAddPercentAlert = () => {
    const pct = parseFloat(targetPercentInput);
    if (isNaN(pct) || pct === 0 || Math.abs(pct) > 1000) {
      setInputError('Введите корректный процент');
      return;
    }
    const calculatedTarget = Math.round(selectedCoin.price * (1 + pct / 100));
    if (calculatedTarget <= 0 || calculatedTarget > 500000) {
      setInputError('Итоговая цена вне диапазона (0; 500 000]');
      return;
    }
    setInputError('');
    triggerHaptic('medium');

    const sign = pct > 0 ? '+' : '';
    const newAlert = {
      id: Date.now(),
      targetPrice: calculatedTarget,
      diffLabel: `${sign}${pct}%`,
      isActive: true,
    };

    setCoins((prev) =>
      prev.map((c) =>
        c.id === selectedCoin.id
          ? { ...c, alerts: [...c.alerts, newAlert] }
          : c
      )
    );
    setTargetPercentInput('');
  };

  const toggleAlertStatus = (alertId) => {
    triggerHaptic('light');
    setCoins((prev) =>
      prev.map((c) =>
        c.id === selectedCoin.id
          ? {
              ...c,
              alerts: c.alerts.map((a) =>
                a.id === alertId ? { ...a, isActive: !a.isActive } : a
              ),
            }
          : c
      )
    );
  };

  const deleteAlert = (alertId) => {
    triggerHaptic('medium');
    setCoins((prev) =>
      prev.map((c) =>
        c.id === selectedCoin.id
          ? { ...c, alerts: c.alerts.filter((a) => a.id !== alertId) }
          : c
      )
    );
  };

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
    <div className="max-w-md mx-auto min-h-screen bg-white relative flex flex-col font-sans shadow-xl overflow-hidden">
      <header className="flex items-center justify-between px-4 py-3 border-b border-gray-200 bg-white sticky top-0 z-10">
        <div className="flex items-center gap-2">
          <span className="font-bold text-xl italic tracking-tight text-black">
            MarketMind
          </span>
        </div>
        <div className="text-right text-xs italic text-gray-500 leading-tight">
          Добрый день,
          <br />
          <span className="font-medium text-gray-600">{userName}</span>
        </div>
      </header>

      <main
        className={`flex-1 p-4 space-y-3 transition-all duration-200 ${
          activeSheet ? 'blur-xs pointer-events-none select-none opacity-60' : ''
        }`}
      >
        {watchedCoins.map((coin) => {
          const isPositive = coin.change24hDollar >= 0;
          return (
            <div
              key={coin.id}
              onClick={() => handleOpenCoin(coin.id)}
              className="flex items-center justify-between px-4 py-3.5 min-h-[64px] bg-white rounded-2xl shadow-[0_2px_10px_rgba(0,0,0,0.08)] border border-gray-100 active:scale-[0.99] transition-transform cursor-pointer"
            >
              <span className="text-base font-normal text-black">
                {coin.name} ({coin.ticker})
              </span>

              <div className="flex items-center gap-3">
                <span className="text-base font-medium text-black">
                  {coin.price.toLocaleString('ru-RU')}$
                </span>

                <div
                  className={`flex items-center text-xs italic leading-tight ${
                    isPositive ? 'text-green-600' : 'text-red-500'
                  }`}
                >
                  {isPositive ? (
                    <ArrowUp className="w-4 h-4 mr-0.5 stroke-[2.5]" />
                  ) : (
                    <ArrowDown className="w-4 h-4 mr-0.5 stroke-[2.5]" />
                  )}
                  <div className="flex flex-col">
                    <span>{Math.abs(coin.change24hDollar)}$</span>
                    <span>{Math.abs(coin.change24hPercent)}%</span>
                  </div>
                </div>
              </div>
            </div>
          );
        })}
      </main>

      {!activeSheet && (
        <button
          onClick={handleOpenSearch}
          className="fixed bottom-6 right-6 w-14 h-14 bg-blue-500 hover:bg-blue-600 active:scale-95 text-white rounded-full shadow-lg flex items-center justify-center transition-transform z-20 cursor-pointer"
        >
          <Plus className="w-8 h-8 stroke-[2]" />
        </button>
      )}

      {activeSheet && (
        <div
          onClick={closeSheet}
          className="fixed inset-0 z-30 flex flex-col justify-end bg-black/15"
        >
          <div
            onClick={(e) => e.stopPropagation()}
            className="max-w-md w-full mx-auto bg-white rounded-t-[32px] shadow-[0_-8px_30px_rgba(0,0,0,0.12)] border-t border-gray-200 p-5 min-h-[72vh] max-h-[85vh] overflow-y-auto relative animate-in slide-in-from-bottom duration-200"
          >
            <button
              onClick={closeSheet}
              className="absolute top-4 right-5 p-2 text-gray-400 hover:text-gray-600 cursor-pointer"
            >
              <X className="w-6 h-6" />
            </button>

            {activeSheet === 'search' && (
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
                          prev === 'Все'
                            ? 'Binance'
                            : prev === 'Binance'
                            ? 'Bybit'
                            : 'Все'
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
                            onClick={() => toggleWatchCoin(coin.id)}
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
            )}

            {activeSheet === 'coin' && selectedCoin && (
              <div className="mt-4 space-y-5">
                <div>
                  <h2 className="text-3xl font-normal text-black">
                    {selectedCoin.name} ({selectedCoin.ticker})
                  </h2>
                  <div className="flex items-center gap-3 mt-2">
                    <span className="text-2xl font-normal text-black">
                      {selectedCoin.price.toLocaleString('ru-RU')}$
                    </span>
                    <div
                      className={`flex items-center text-xs italic leading-tight ${
                        selectedCoin.change24hDollar >= 0
                          ? 'text-green-600'
                          : 'text-red-500'
                      }`}
                    >
                      {selectedCoin.change24hDollar >= 0 ? (
                        <ArrowUp className="w-4 h-4 mr-0.5" />
                      ) : (
                        <ArrowDown className="w-4 h-4 mr-0.5" />
                      )}
                      <div className="flex flex-col">
                        <span>{Math.abs(selectedCoin.change24hDollar)}$</span>
                        <span>{Math.abs(selectedCoin.change24hPercent)}%</span>
                      </div>
                    </div>
                  </div>
                </div>

                <div>
                  <h3 className="text-xl font-normal text-black mb-3">
                    Добавить алерт
                  </h3>
                  <div className="grid grid-cols-2 gap-3">
                    <div className="p-3 rounded-2xl border border-gray-200 shadow-xs flex flex-col justify-between">
                      <label className="text-xs text-gray-700 mb-2">
                        Достижение стоимости
                      </label>
                      <input
                        type="number"
                        value={targetPriceInput}
                        onChange={(e) => setTargetPriceInput(e.target.value)}
                        className={`w-full px-2 py-1 text-sm border rounded-md mb-2 focus:outline-none ${
                          inputError ? 'border-red-500' : 'border-gray-300'
                        }`}
                      />
                      <button
                        onClick={handleAddPriceAlert}
                        className="self-end px-4 py-1 bg-blue-500 text-white text-xs rounded-lg cursor-pointer"
                      >
                        ОК
                      </button>
                    </div>

                    <div className="p-3 rounded-2xl border border-gray-200 shadow-xs flex flex-col justify-between">
                      <label className="text-xs text-gray-700 mb-2">
                        Изменение на (%)
                      </label>
                      <input
                        type="number"
                        value={targetPercentInput}
                        onChange={(e) => setTargetPercentInput(e.target.value)}
                        className={`w-full px-2 py-1 text-sm border rounded-md mb-2 focus:outline-none ${
                          inputError ? 'border-red-500' : 'border-gray-300'
                        }`}
                      />
                      <button
                        onClick={handleAddPercentAlert}
                        className="self-end px-4 py-1 bg-blue-500 text-white text-xs rounded-lg cursor-pointer"
                      >
                        ОК
                      </button>
                    </div>
                  </div>
                  {inputError && (
                    <p className="text-xs text-red-500 mt-1.5">{inputError}</p>
                  )}
                </div>

                <div>
                  <h3 className="text-xl font-normal text-black mb-3">
                    Активные алерты
                  </h3>
                  {selectedCoin.alerts.length === 0 ? (
                    <p className="text-center text-gray-500 text-sm py-6">
                      У Вас нет активных алертов
                    </p>
                  ) : (
                    <div className="space-y-3">
                      {selectedCoin.alerts.map((alert) => (
                        <div
                          key={alert.id}
                          className="flex items-center justify-between px-4 py-3.5 bg-white rounded-2xl shadow-[0_2px_10px_rgba(0,0,0,0.08)] border border-gray-100"
                        >
                          <span className="text-sm font-normal text-black">
                            Цель: {alert.targetPrice.toLocaleString('ru-RU')}$ (
                            {alert.diffLabel})
                          </span>

                          <div className="flex items-center gap-3">
                            <button
                              onClick={() => toggleAlertStatus(alert.id)}
                              className="w-12 h-7 bg-gray-200 rounded-full p-0.5 flex items-center transition-all cursor-pointer"
                            >
                              <div
                                className={`w-6 h-6 rounded-full flex items-center justify-center text-white transition-transform ${
                                  alert.isActive
                                    ? 'bg-blue-500 translate-x-0'
                                    : 'bg-red-500 translate-x-5'
                                }`}
                              >
                                {alert.isActive ? (
                                  <Bell className="w-3.5 h-3.5" />
                                ) : (
                                  <BellOff className="w-3.5 h-3.5" />
                                )}
                              </div>
                            </button>

                            <button
                              onClick={() => deleteAlert(alert.id)}
                              className="p-1 text-gray-700 hover:text-red-500 cursor-pointer"
                            >
                              <Trash2 className="w-5 h-5" />
                            </button>
                          </div>
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );
}