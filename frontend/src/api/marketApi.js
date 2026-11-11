import { apiClient } from './client';
import { getTelegramUser } from '../utils/telegram';


//заглушечки
const FALLBACK_EXCHANGES = [
  { id: 1, name: 'Binance' },
  { id: 2, name: 'Bybit' },
  { id: 3, name: 'OKX' },
];

const FALLBACK_SUBSCRIPTIONS = [
  {
    subscription_id: '18',
    exchange_name: 'Binance',
    base_asset: 'BTC',
    quote_asset: 'USDT',
    price: '83781',
  },
  {
    subscription_id: '19',
    exchange_name: 'Bybit',
    base_asset: 'ETH',
    quote_asset: 'USDT',
    price: '2505',
  },
  {
    subscription_id: '20',
    exchange_name: 'OKX',
    base_asset: 'ZEC',
    quote_asset: 'USDT',
    price: '1252',
  },
];

const FALLBACK_MARKETS = [
  {
    exchange_id: 1,
    external_id: 'BTCUSDT',
    base_asset: 'BTC',
    quote_asset: 'USDT',
    display_name: 'BTC/USDT',
    price: '83781',
  },
  {
    exchange_id: 1,
    external_id: 'BTWUSDT',
    base_asset: 'BTW',
    quote_asset: 'USDT',
    display_name: 'BTW/USDT',
    price: '1.92',
  },
  {
    exchange_id: 2,
    external_id: 'ETHUSDT',
    base_asset: 'ETH',
    quote_asset: 'USDT',
    display_name: 'ETH/USDT',
    price: '2505',
  },
  {
    exchange_id: 3,
    external_id: 'ZECUSDT',
    base_asset: 'ZEC',
    quote_asset: 'USDT',
    display_name: 'ZEC/USDT',
    price: '1252',
  },
];

//профиль и инфа
export const initAppData = async () => {
  try {
    await apiClient.post('/users/me');

    const [exchangesRes, subsRes] = await Promise.all([
      apiClient.get('/exchanges'),
      apiClient.get('/subscriptions'),
    ]);

    return {
      user: getTelegramUser(),
      exchanges: exchangesRes.data.items || [],
      subscriptions: subsRes.data.items || [],
      isOffline: false,
    };
  } catch (error) {
    console.warn('API недоступен, демо режим');
    return {
      user: getTelegramUser(),
      exchanges: FALLBACK_EXCHANGES,
      subscriptions: FALLBACK_SUBSCRIPTIONS,
      isOffline: true,
    };
  }
};

//оиск рынков
export const searchMarketsApi = async (exchangeId, query = '', page = 1, limit = 20) => {
  try {
    const params = { exchange_id: exchangeId, page, limit };
    const trimmed = query.trim();
    if (trimmed) {
      params.q = trimmed;
    }
    const response = await apiClient.get('/markets', { params });
    return response.data; 
  } catch (error) {
    const q = query.trim().toLowerCase();
    const filtered = FALLBACK_MARKETS.filter(
      (m) =>
        m.exchange_id === exchangeId &&
        (m.display_name.toLowerCase().includes(q) ||
          m.base_asset.toLowerCase().includes(q))
    );
    return { items: filtered, next_page: null };
  }
};



// обавить рынок в отслеживаемое
export const addSubscriptionApi = async (marketOption) => {
  const payload = {
    exchange_id: marketOption.exchange_id,
    external_id: marketOption.external_id,
    base_asset: marketOption.base_asset,
    quote_asset: marketOption.quote_asset,
    display_name: marketOption.display_name,
    price: marketOption.price,
  };
  const response = await apiClient.post('/subscriptions', payload);
  return response.data; 
};

//удалить рынок из отслеживаемого
export const deleteSubscriptionApi = async (subscriptionId) => {
  await apiClient.delete(`/subscriptions/${subscriptionId}`);
};


//получение карточки монетф
export const getSubscriptionDetailApi = async (subscriptionId) => {
  const response = await apiClient.get(`/subscriptions/${subscriptionId}`);
  return response.data; 
};
//новый алерт
export const createAlertApi = async (subscriptionId, { type, value, direction }) => {
  const cleanValue = String(value).replace(/^\+/, '').trim();
  const response = await apiClient.post(`/subscriptions/${subscriptionId}/alerts`, {
    type,
    value: cleanValue,
    direction,
  });
  return response.data; 
};

//колокольчик
export const toggleAlertApi = async (subscriptionId, alertId, isActive) => {
  const response = await apiClient.patch(
    `/subscriptions/${subscriptionId}/alerts/${alertId}`,
    { is_active: Boolean(isActive) }
  );
  return response.data;
};

//удаление
export const deleteAlertApi = async (subscriptionId, alertId) => {
  await apiClient.delete(`/subscriptions/${subscriptionId}/alerts/${alertId}`);
};