import axios from 'axios';
import { getTelegramUser } from '../utils/telegram';

//VITE_API_URL=http://localhost:8080/api/v1
const API_BASE_URL = import.meta.env.VITE_API_URL || 'http://localhost:8080/api/v1';

export const apiClient = axios.create({
  baseURL: API_BASE_URL,
  timeout: 5000,
  headers: {
    'Content-Type': 'application/json',
  },
});

apiClient.interceptors.request.use((config) => {
  const { initData } = getTelegramUser();
  if (initData) {
    config.headers.Authorization = `tma ${initData}`;
  }
  return config;
});