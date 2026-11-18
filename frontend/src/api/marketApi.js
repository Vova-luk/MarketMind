import { apiClient } from './client';
import { getTelegramUser } from '../utils/telegram';



// профиль и инфа
export const initAppData = async () => {
  const tgUser = getTelegramUser();

  const userRes = await apiClient.post('/users/me');

  const [exchangesRes, subsRes] = await Promise.all([
    apiClient.get('/exchanges'),
    apiClient.get('/subscriptions'),
  ]);

  return {
    user: {
      ...tgUser,
      userId: userRes.data?.telegram_user_id || tgUser.userId,
    },
    exchanges: exchangesRes.data.items || [],
    subscriptions: subsRes.data.items || [],
  };
};

// поиск рынков
export const searchMarketsApi = async (exchangeId, query = '', page = 1, limit = 20) => {
 const params = { exchange_id: Number(exchangeId), page, limit };
  const trimmed = query.trim();
  if (trimmed) {
    params.q = trimmed;
  }
  const response = await apiClient.get('/markets', { params });
  return response.data;
};

// добавить рынок в отслеживаемое
export const addSubscriptionApi = async (marketOption) => {
  const payload = {
    exchange_id: Number(marketOption.exchange_id),
    external_id: marketOption.external_id,
    base_asset: marketOption.base_asset,
    quote_asset: marketOption.quote_asset,
    display_name: marketOption.display_name,
    price: marketOption.price_exact ?? marketOption.price,
  };
  const response = await apiClient.post('/subscriptions', payload);
  return response.data;
};

// удалить рынок из отслеживаемого
export const deleteSubscriptionApi = async (subscriptionId) => {
  await apiClient.delete(`/subscriptions/${subscriptionId}`);
};

// получение карточки монеты
export const getSubscriptionDetailApi = async (subscriptionId) => {
  const response = await apiClient.get(`/subscriptions/${subscriptionId}`);
  return response.data;
};

// новый алерт
export const createAlertApi = async (subscriptionId, { type, value, direction }) => {
  const cleanValue = String(value).replace(/^\+/, '').trim();
  const response = await apiClient.post(`/subscriptions/${subscriptionId}/alerts`, {
    type,
    value: cleanValue,
    direction,
  });
  return response.data;
};

// колокольчик
export const toggleAlertApi = async (subscriptionId, alertId, isActive) => {
  const response = await apiClient.patch(
    `/subscriptions/${subscriptionId}/alerts/${alertId}`,
    { is_active: Boolean(isActive) }
  );
  return response.data;
};

// удаление
export const deleteAlertApi = async (subscriptionId, alertId) => {
  await apiClient.delete(`/subscriptions/${subscriptionId}/alerts/${alertId}`);
};