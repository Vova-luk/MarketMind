const tg = window.Telegram?.WebApp;

export const initTelegram = () => {
  try {
    tg?.ready();
    tg?.expand();
  } catch (e) {
    console.warn('Запущено вне среды Telegram');
  }
};

export const getTelegramUser = () => {
  return {
    id: tg?.initDataUnsafe?.user?.id || null,
    firstName: tg?.initDataUnsafe?.user?.first_name || 'Пользователь',
    username: tg?.initDataUnsafe?.user?.username || '',
    initData: tg?.initData || '',
  };
};

export const triggerHaptic = (style = 'light') => {
  try {
    tg?.HapticFeedback?.impactOccurred(style);
  } catch (e) {
    console.warn('Запущено вне среды Telegram');}
};