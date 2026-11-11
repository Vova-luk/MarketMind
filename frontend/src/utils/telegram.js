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
  const user = tg?.initDataUnsafe?.user;

  return {
    userId: user?.id ? String(user.id) : null,
    username: user?.first_name || 'Пользователь',
    initData: tg?.initData || '',
  };
};

export const triggerHaptic = (style = 'light') => {
  try {
    tg?.HapticFeedback?.impactOccurred(style);
  } catch (e) {
    console.warn('Запущено вне среды Telegram');}
};