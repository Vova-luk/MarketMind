import { useState, useEffect } from 'react';
import { Plus, X } from 'lucide-react';
import Header from './components/Header';
import CoinCard from './components/CoinCard';
import SearchSheet from './components/SearchSheet';
import CoinSheet from './components/CoinSheet';
import { initTelegram, triggerHaptic } from './utils/telegram';
import {
  initAppData,
  addSubscriptionApi,
  deleteSubscriptionApi,
  getSubscriptionDetailApi,
  createAlertApi,
  toggleAlertApi,
  deleteAlertApi,
} from './api/marketApi';

export default function App() {
  const [userName, setUserName] = useState('Пользователь');
  const [exchanges, setExchanges] = useState([]);
  const [subscriptions, setSubscriptions] = useState([]);
  const [isLoading, setIsLoading] = useState(true);

  const [activeSheet, setActiveSheet] = useState(null); 
  const [selectedDetail, setSelectedDetail] = useState(null);

  const [subToDelete, setSubToDelete] = useState(null);

  const handleConfirmDeleteSubscription = async () => {
    if (!subToDelete) return;
    triggerHaptic('medium');

    const idToRemove = subToDelete.subscription_id;
    setSubscriptions((prev) =>
      prev.filter((s) => s.subscription_id !== idToRemove)
    );
    setSubToDelete(null);

    try {
      await deleteSubscriptionApi(idToRemove);
    } catch (e) {}
  };

  const loadInitial = async () => {
    setIsLoading(true);
    const data = await initAppData();
    if (data.user?.username) {
      setUserName(data.user.username);
    }
    setExchanges(data.exchanges);
    setSubscriptions(data.subscriptions);
    setIsLoading(false);
  };

  useEffect(() => {
    initTelegram();
    loadInitial();
  }, []);

  //карточка монеты
  const handleOpenSubscription = async (subscriptionId) => {
    triggerHaptic('light');
    const baseSub = subscriptions.find(
      (s) => s.subscription_id === subscriptionId
    );
    setSelectedDetail({ ...baseSub, alerts: baseSub?.alerts || [] });
    setActiveSheet('coin');

    try {
      const detail = await getSubscriptionDetailApi(subscriptionId);
      setSelectedDetail(detail);
    } catch (e) {
    }
  };

  //добавить-удалить подписку
  const handleToggleSubscription = async (market, existingSub) => {
    triggerHaptic('light');

    if (existingSub) {
      //удалить
      setSubToDelete(existingSub);
    } else {
      // добавить
      try {
        const created = await addSubscriptionApi(market);
        setSubscriptions((prev) => [...prev, created]);
      } catch (e) {
        const exchangeObj = exchanges.find((ex) => ex.id === market.exchange_id);
        const mockSub = {
          subscription_id: String(Date.now()),
          exchange_name: exchangeObj?.name || 'Binance',
          base_asset: market.base_asset,
          quote_asset: market.quote_asset,
          price: market.price,
          alerts: [],
        };
        setSubscriptions((prev) => [...prev, mockSub]);
      }
    }
  };

  //создание алерта
  const handleAddAlert = async (alertPayload) => {
    if (!selectedDetail) return;
    triggerHaptic('medium');
    const subId = selectedDetail.subscription_id;

    try {
      const createdAlert = await createAlertApi(subId, alertPayload);
      setSelectedDetail((prev) => ({
        ...prev,
        alerts: [...(prev.alerts || []), createdAlert],
      }));
    } catch (e) {
      const refPrice = Number(selectedDetail.price || 0);
      const targetPrice =
        alertPayload.type === 'price'
          ? alertPayload.value
          : String(
              Math.round(refPrice * (1 + Number(alertPayload.value) / 100) * 100) / 100
            );

      const mockAlert = {
        alert_id: String(Date.now()),
        type: alertPayload.type,
        value: alertPayload.value,
        condition: alertPayload.condition,
        reference_price: String(refPrice),
        target_price: targetPrice,
        is_active: true,
      };
      setSelectedDetail((prev) => ({
        ...prev,
        alerts: [...(prev.alerts || []), mockAlert],
      }));
    }
  };

  //уведомление о включении-выключении алерта
  const handleToggleAlert = async (alertId, nextActive) => {
    if (!selectedDetail) return;
    triggerHaptic('light');
    const subId = selectedDetail.subscription_id;

    setSelectedDetail((prev) => ({
      ...prev,
      alerts: prev.alerts.map((a) =>
        a.alert_id === alertId ? { ...a, is_active: nextActive } : a
      ),
    }));

    try {
      await toggleAlertApi(subId, alertId, nextActive);
    } catch (e) {}
  };

  //удалениеалерта
  const handleDeleteAlert = async (alertId) => {
    if (!selectedDetail) return;
    triggerHaptic('medium');
    const subId = selectedDetail.subscription_id;

    setSelectedDetail((prev) => ({
      ...prev,
      alerts: prev.alerts.filter((a) => a.alert_id !== alertId),
    }));

    try {
      await deleteAlertApi(subId, alertId);
    } catch (e) {}
  };

  return (
    <div className="max-w-md mx-auto min-h-screen bg-white relative flex flex-col font-sans shadow-xl overflow-hidden">
      <Header userName={userName} />

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
        ) : subscriptions.length === 0 ? (
          <div className="text-center py-20 text-gray-400 text-sm">
            Список отслеживания пуст. Нажмите «+», чтобы добавить рынок.
          </div>
        ) : (
          subscriptions.map((sub) => (
            <CoinCard
              key={sub.subscription_id}
              subscription={sub}
              onClick={handleOpenSubscription}
              onRequestDelete={(targetSub) => {
                triggerHaptic('light');
                setSubToDelete(targetSub);
              }}
            />
          ))
        )}
        
      </main>

      {!activeSheet && (
        <button
          onClick={() => {
            triggerHaptic('medium');
            setActiveSheet('search');
          }}
          className="fixed bottom-6 right-6 w-14 h-14 bg-blue-500 hover:bg-blue-600 active:scale-95 text-white rounded-full shadow-lg flex items-center justify-center transition-transform z-20 cursor-pointer"
        >
          <Plus className="w-8 h-8 stroke-[2]" />
        </button>
      )}

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
              <SearchSheet
                exchanges={exchanges}
                subscriptions={subscriptions}
                onToggleSubscription={handleToggleSubscription}
              />
            )}

            {activeSheet === 'coin' && selectedDetail && (
              <CoinSheet
                detail={selectedDetail}
                onAddAlert={handleAddAlert}
                onToggleAlert={handleToggleAlert}
                onDeleteAlert={handleDeleteAlert}
              />
            )}
          </div>
        </div>
      )}
      {/* Модальное окно подтверждения удаления подписки */}
      {subToDelete && (
        <div
          onClick={() => setSubToDelete(null)}
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 px-4"
        >
          <div
            onClick={(e) => e.stopPropagation()}
            className="w-full max-w-xs bg-white rounded-2xl p-5 shadow-xl space-y-4 text-center animate-in fade-in zoom-in-95 duration-150"
          >
            <h3 className="text-base font-medium text-black">
              Газ удалять {subToDelete.base_asset}/{subToDelete.quote_asset} (
              {subToDelete.exchange_name})?
            </h3>
            {/* {<p className="text-xs text-gray-500 leading-relaxed">
              Газ удалять {subToDelete.base_asset}/{subToDelete.quote_asset} (
              {subToDelete.exchange_name})?
            </p>} */}
           
            <div className="grid grid-cols-2 gap-2.5 pt-1">
              <button
                type="button"
                onClick={() => setSubToDelete(null)}
                className="py-2 px-4 rounded-xl bg-gray-100 hover:bg-gray-200 text-sm font-medium text-gray-700 cursor-pointer transition-colors"
              >
                не
              </button>
              <button
                type="button"
                onClick={handleConfirmDeleteSubscription}
                className="py-2 px-4 rounded-xl bg-red-500 hover:bg-red-600 text-sm font-medium text-white cursor-pointer transition-colors"
              >
                газ
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}