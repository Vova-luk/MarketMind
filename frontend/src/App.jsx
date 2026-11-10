import { useState, useEffect } from 'react';
import { Plus, X } from 'lucide-react';
import Header from './components/Header';
import CoinCard from './components/CoinCard';
import SearchSheet from './components/SearchSheet';
import CoinSheet from './components/CoinSheet';
import { initTelegram, triggerHaptic } from './utils/telegram';
import {
  fetchInitialData,
  toggleWatchlistApi,
  createAlertApi,
  toggleAlertApi,
  deleteAlertApi,
} from './api/marketApi';

export default function App() {
  const [userName, setUserName] = useState('Пользователь');
  const [coins, setCoins] = useState([]);
  const [isLoading, setIsLoading] = useState(true);
  const [syncError, setSyncError] = useState(false);

  const [activeSheet, setActiveSheet] = useState(null); 
  const [selectedCoinId, setSelectedCoinId] = useState(null);

  const loadData = async () => {
    setIsLoading(true);
    setSyncError(false);
    const data = await fetchInitialData();
    if (data.user?.firstName) {
      setUserName(data.user.firstName);
    }
    setCoins(data.coins || []);
    setIsLoading(false);
  };

  useEffect(() => {
    initTelegram();
    loadData();
  }, []);

  const watchedCoins = coins.filter((c) => c.isWatched);
  const selectedCoin = coins.find((c) => c.id === selectedCoinId);

  const handleOpenCoin = (id) => {
    triggerHaptic('light');
    setSelectedCoinId(id);
    setActiveSheet('coin');
  };

  const handleOpenSearch = () => {
    triggerHaptic('medium');
    setActiveSheet('search');
  };

  const handleToggleWatch = async (coinId) => {
    triggerHaptic('light');
    const target = coins.find((c) => c.id === coinId);
    if (!target) return;

    setCoins((prev) =>
      prev.map((c) => (c.id === coinId ? { ...c, isWatched: !c.isWatched } : c))
    );

    try {
      await toggleWatchlistApi(coinId, target.isWatched);
    } catch (e) {
    }
  };

  const handleAddAlert = async (coinId, targetPrice, diffLabel) => {
    triggerHaptic('medium');
    const tempAlert = {
      id: Date.now(),
      targetPrice,
      diffLabel,
      isActive: true,
    };

    setCoins((prev) =>
      prev.map((c) =>
        c.id === coinId ? { ...c, alerts: [...c.alerts, tempAlert] } : c
      )
    );

    try {
      await createAlertApi(coinId, targetPrice, diffLabel);
    } catch (e) {
    }
  };

  const handleToggleAlert = async (coinId, alertId) => {
    triggerHaptic('light');
    const targetCoin = coins.find((c) => c.id === coinId);
    const targetAlert = targetCoin?.alerts.find((a) => a.id === alertId);

    setCoins((prev) =>
      prev.map((c) =>
        c.id === coinId
          ? {
              ...c,
              alerts: c.alerts.map((a) =>
                a.id === alertId ? { ...a, isActive: !a.isActive } : a
              ),
            }
          : c
      )
    );

    try {
      await toggleAlertApi(alertId, !targetAlert?.isActive);
    } catch (e) {}
  };

  const handleDeleteAlert = async (coinId, alertId) => {
    triggerHaptic('medium');
    setCoins((prev) =>
      prev.map((c) =>
        c.id === coinId
          ? { ...c, alerts: c.alerts.filter((a) => a.id !== alertId) }
          : c
      )
    );

    try {
      await deleteAlertApi(alertId);
    } catch (e) {}
  };

  return (
    <div className="max-w-md mx-auto min-h-screen bg-white relative flex flex-col font-sans shadow-xl overflow-hidden">
      <Header userName={userName} />

      {/*баннер ошибки синхронизации*/}
      {syncError && (
        <div className="mx-4 mt-3 p-3 bg-red-50 border border-red-200 rounded-xl flex items-center justify-between text-xs text-red-600">
          <span>Ошибка синхронизации данных</span>
          <button
            onClick={loadData}
            className="px-2.5 py-1 bg-red-600 text-white rounded-lg cursor-pointer"
          >
            Повторить
          </button>
        </div>
      )}

      {/* главный список + скелетоны при загрузке */}
      <main
        className={`flex-1 p-4 space-y-3 transition-all duration-200 ${
          activeSheet ? 'blur-xs pointer-events-none select-none opacity-60' : ''
        }`}
      >
        {isLoading ? (
          [1, 2, 3].map((n) => (
            <div
              key={n}
              className="h-16 bg-gray-100 animate-pulse rounded-2xl"
            />
          ))
        ) : watchedCoins.length === 0 ? (
          <div className="text-center py-20 text-gray-400 text-sm">
            Список отслеживания пуст. Нажмите «+», чтобы добавить монеты.
          </div>
        ) : (
          watchedCoins.map((coin) => (
            <CoinCard key={coin.id} coin={coin} onClick={handleOpenCoin} />
          ))
        )}
      </main>

      {/*кнопка + */}
      {!activeSheet && (
        <button
          onClick={handleOpenSearch}
          className="fixed bottom-6 right-6 w-14 h-14 bg-blue-500 hover:bg-blue-600 active:scale-95 text-white rounded-full shadow-lg flex items-center justify-center transition-transform z-20 cursor-pointer"
        >
          <Plus className="w-8 h-8 stroke-[2]" />
        </button>
      )}

      {/*модальная шторка */}
      {activeSheet && (
        <div
          onClick={() => setActiveSheet(null)}
          className="fixed inset-0 z-30 flex flex-col justify-end bg-black/15"
        >
          <div
            onClick={(e) => e.stopPropagation()}
            className="max-w-md w-full mx-auto bg-white rounded-t-[32px] shadow-[0_-8px_30px_rgba(0,0,0,0.12)] border-t border-gray-200 p-5 min-h-[72vh] max-h-[85vh] overflow-y-auto relative"
          >
            <button
              onClick={() => setActiveSheet(null)}
              className="absolute top-4 right-5 p-2 text-gray-400 hover:text-gray-600 cursor-pointer"
            >
              <X className="w-6 h-6" />
            </button>

            {activeSheet === 'search' && (
              <SearchSheet coins={coins} onToggleWatch={handleToggleWatch} />
            )}

            {activeSheet === 'coin' && selectedCoin && (
              <CoinSheet
                coin={selectedCoin}
                onAddAlert={handleAddAlert}
                onToggleAlert={handleToggleAlert}
                onDeleteAlert={handleDeleteAlert}
              />
            )}
          </div>
        </div>
      )}
    </div>
  );
}