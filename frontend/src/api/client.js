import axios from 'axios';
import { getTelegramUser } from '../utils/telegram';

const API_BASE_URL = import.meta.env.VITE_API_URL || '/api/v1';

export const apiClient = axios.create({
  baseURL: API_BASE_URL,
  timeout: 8000,
});

apiClient.interceptors.request.use((config) => {
  const { initData } = getTelegramUser();
  const rawInitData = initData || import.meta.env.VITE_DEV_INIT_DATA || '';

  if (rawInitData) {
    config.headers.Authorization = `tma ${rawInitData}`;
  }

  if (config.data !== undefined) {
    config.headers['Content-Type'] = 'application/json';
  }

  return config;
});