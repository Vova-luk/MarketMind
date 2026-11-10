import { apiClient } from './client';
import { getTelegramUser } from '../utils/telegram';


//заглушечки
const FALLBACK_COINS = [
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

//профиль и монеты
export const fetchInitialData = async () => {
  try {
    const response = await apiClient.get('/coins');
    return {
      user: response.data.user || getTelegramUser(),
      coins: response.data.coins,
      isOffline: false,
    };
  } catch (error) {
    console.warn('local data');
    return {
      user: getTelegramUser(),
      coins: FALLBACK_COINS,
      isOffline: true,
    };
  }
};

//добавить-удалить монету
export const toggleWatchlistApi = async (coinId, isCurrentlyWatched) => {
  if (isCurrentlyWatched) {
    return apiClient.delete(`/watchlist/${coinId}`);
  } else {
    return apiClient.post('/watchlist', { coinId });
  }
};

//новый алерт
export const createAlertApi = async (coinId, targetPrice, diffLabel) => {
  const response = await apiClient.post('/alerts', {
    coinId,
    targetPrice,
    diffLabel,
  });
  return response.data;
};

//колокольчик
export const toggleAlertApi = async (alertId, isActive) => {
  return apiClient.patch(`/alerts/${alertId}`, { isActive });
};

//удаление
export const deleteAlertApi = async (alertId) => {
  return apiClient.delete(`/alerts/${alertId}`);
};