import { useState, useEffect, useRef } from 'react';
import { Plus, X, AlertCircle } from 'lucide-react';
import Header from './components/Header';
import CoinCard from './components/CoinCard';
import SearchSheet from './components/SearchSheet';
import CoinSheet from './components/CoinSheet';
import { initTelegram, getTelegramUser, triggerHaptic } from './utils/telegram';
import {
  initAppData,
  addSubscriptionApi,
  deleteSubscriptionApi,
  getSubscriptionDetailApi,
  createAlertApi,
  toggleAlertApi,
  deleteAlertApi,
} from './api/marketApi';

const MAX_SUBSCRIPTIONS = 10;
const MAX_ALERTS_PER_SUBSCRIPTION = 10;

export default function App() {
  const [userName, setUserName] = useState('Пользователь');
  const [exchanges, setExchanges] = useState([]);
  const [subscriptions, setSubscriptions] = useState([]);
  const [isLoading, setIsLoading] = useState(true);
  const [serverError, setServerError] = useState(false);

  const [activeSheet, setActiveSheet] = useState(null);
  const [selectedDetail, setSelectedDetail] = useState(null);
  const [subToDelete, setSubToDelete] = useState(null);

  const [toastMessage, setToastMessage] = useState('');
  const toastTimerRef = useRef(null);

  const showToast = (message) => {
    setToastMessage(message);
    if (toastTimerRef.current) {
      clearTimeout(toastTimerRef.current);
    }
    toastTimerRef.current = setTimeout(() => {
      setToastMessage('');
    }, 3000);
  };

  useEffect(() => {
    return () => {
      if (toastTimerRef.current) clearTimeout(toastTimerRef.current);
    };
  }, []);

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
    setServerError(false);
    try {
      const data = await initAppData();
      if (data.user?.username) {
        setUserName(data.user.username);
      }
      setExchanges(data.exchanges);
      setSubscriptions(data.subscriptions);
    } catch (e) {
      setServerError(true);
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    initTelegram();
    loadInitial();
  }, []);

  const updateSubscriptionAlerts = (subscriptionId, updateFn) => {
    setSelectedDetail((prev) => {
      if (!prev || prev.subscription_id !== subscriptionId) return prev;
      return { ...prev, alerts: updateFn(prev.alerts || []) };
    });

    setSubscriptions((prev) =>
      prev.map((sub) =>
        sub.subscription_id === subscriptionId
          ? { ...sub, alerts: updateFn(sub.alerts || []) }
          : sub
      )
    );
  };

  // Карточка монеты
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
    } catch (e) {}
  };

  // Добавить / удалить подписку
  const handleToggleSubscription = async (market, existingSub) => {
    triggerHaptic('light');

    if (existingSub) {
      setSubToDelete(existingSub);
    } else {
      if (subscriptions.length >= MAX_SUBSCRIPTIONS) {
        showToast(`Можно отслеживать не более ${MAX_SUBSCRIPTIONS} монет`);
        return;
      }
      // добавить
      try {
        const created = await addSubscriptionApi(market);
        setSubscriptions((prev) => [...prev, created]);
      } catch (e) {}
    }
  };

  // Создание уведомления
  const handleAddAlert = async (alertPayload) => {
    if (!selectedDetail) return;

    if ((selectedDetail.alerts?.length || 0) >= MAX_ALERTS_PER_SUBSCRIPTION) {
      showToast(
        `Не более ${MAX_ALERTS_PER_SUBSCRIPTION} уведомлений для одной монеты`
      );
      return;
    }

    triggerHaptic('medium');
    const subId = selectedDetail.subscription_id;

    try {
      const createdAlert = await createAlertApi(subId, alertPayload);
      setSelectedDetail((prev) => ({
        ...prev,
        alerts: [...(prev.alerts || []), createdAlert],
      }));
    } catch (e) {}
  };

  //включение-выключение алерта
  const handleToggleAlert = async (alertId, nextActive) => {
    if (!selectedDetail) return;
    triggerHaptic('light');
    const subId = selectedDetail.subscription_id;

    try {
      const updated = await toggleAlertApi(subId, alertId, nextActive);
      updateSubscriptionAlerts(subId, (alerts) =>
        alerts.map((a) => (a.alert_id === alertId ? updated : a))
      );
    } catch (e) {
      console.error('Ошибка переключения уведомления:', e);
    }
  };

  //удаление алерта
  const handleDeleteAlert = async (alertId) => {
    if (!selectedDetail) return;
    triggerHaptic('medium');
    const subId = selectedDetail.subscription_id;

    try {
      await deleteAlertApi(subId, alertId);
      updateSubscriptionAlerts(subId, (alerts) =>
        alerts.filter((a) => a.alert_id !== alertId)
      );
    } catch (e) {
      console.error('Ошибка удаления уведомления:', e);
    }
  };

  return (
    <div className="max-w-md mx-auto min-h-screen bg-white relative flex flex-col font-sans shadow-xl overflow-hidden">
      <Header userName={userName} />

      <main
        className={`flex-1 p-4 space-y-3 transition-all duration-200 ${
          activeSheet || subToDelete
            ? 'blur-xs pointer-events-none select-none opacity-60'
            : ''
        }`}
      >
        {isLoading ? (
          [1, 2, 3].map((n) => (
            <div
              key={n}
              className="h-16 bg-gray-100 animate-pulse rounded-2xl"
            />
          ))
        ) : serverError ? (
          <div className="text-center py-20 text-red-500 text-sm">
            Нет связи с сервером
          </div>
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

      {!activeSheet && !subToDelete && (
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
            className="w-full max-w-xs bg-white rounded-2xl p-5 shadow-xl space-y-4 text-center"
          >
            <h3 className="text-base font-medium text-black">
              Удалить {subToDelete.base_asset}/{subToDelete.quote_asset} (
              {subToDelete.exchange_name})?
            </h3>

            <div className="grid grid-cols-2 gap-2.5 pt-1">
              <button
                type="button"
                onClick={() => setSubToDelete(null)}
                className="py-2 px-4 rounded-xl bg-gray-100 hover:bg-gray-200 text-sm font-medium text-gray-700 cursor-pointer transition-colors"
              >
                Отмена
              </button>
              <button
                type="button"
                onClick={handleConfirmDeleteSubscription}
                className="py-2 px-4 rounded-xl bg-red-500 hover:bg-red-600 text-sm font-medium text-white cursor-pointer transition-colors"
              >
                Удалить
              </button>
            </div>
          </div>
        </div>
      )}

      {toastMessage && (
        <div className="fixed top-4 left-1/2 -translate-x-1/2 z-[60] w-[calc(100%-2rem)] max-w-sm">
          <div className="flex items-center gap-2.5 bg-gray-900/95 text-white px-4 py-3 rounded-2xl shadow-lg text-sm">
            <AlertCircle className="w-5 h-5 text-orange-400 shrink-0" />
            <span className="flex-1">{toastMessage}</span>
          </div>
        </div>
      )}
    </div>
  );
}