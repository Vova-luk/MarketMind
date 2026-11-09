import { useState } from 'react';
import { ArrowUp, ArrowDown, Bell, BellOff, Trash2 } from 'lucide-react';

export default function CoinSheet({
  coin,
  onAddAlert,
  onToggleAlert,
  onDeleteAlert,
}) {
  const [targetPriceInput, setTargetPriceInput] = useState('');
  const [targetPercentInput, setTargetPercentInput] = useState('');
  const [inputError, setInputError] = useState('');

  const handlePriceSubmit = () => {
    const val = parseFloat(targetPriceInput);
    if (isNaN(val) || val <= 0 || val > 500000) {
      setInputError('Введите значение в диапазоне (0; 500 000]');
      return;
    }
    setInputError('');
    const diffDollar = Math.round(val - coin.price);
    const diffSign = diffDollar >= 0 ? '+' : '';
    onAddAlert(coin.id, val, `${diffSign}${diffDollar}$`);
    setTargetPriceInput('');
  };

  const handlePercentSubmit = () => {
    const pct = parseFloat(targetPercentInput);
    if (isNaN(pct) || pct === 0 || Math.abs(pct) > 1000) {
      setInputError('Введите корректный процент');
      return;
    }
    const calculatedTarget = Math.round(coin.price * (1 + pct / 100));
    if (calculatedTarget <= 0 || calculatedTarget > 500000) {
      setInputError('Итоговая цена выходит за пределы (0; 500 000]');
      return;
    }
    setInputError('');
    const sign = pct > 0 ? '+' : '';
    onAddAlert(coin.id, calculatedTarget, `${sign}${pct}%`);
    setTargetPercentInput('');
  };

  return (
    <div className="mt-4 space-y-5">
      <div>
        <h2 className="text-3xl font-normal text-black">
          {coin.name} ({coin.ticker})
        </h2>
        <div className="flex items-center gap-3 mt-2">
          <span className="text-2xl font-normal text-black">
            {coin.price.toLocaleString('ru-RU')}$
          </span>
          <div
            className={`flex items-center text-xs italic leading-tight ${
              coin.change24hDollar >= 0 ? 'text-green-600' : 'text-red-500'
            }`}
          >
            {coin.change24hDollar >= 0 ? (
              <ArrowUp className="w-4 h-4 mr-0.5" />
            ) : (
              <ArrowDown className="w-4 h-4 mr-0.5" />
            )}
            <div className="flex flex-col">
              <span>{Math.abs(coin.change24hDollar)}$</span>
              <span>{Math.abs(coin.change24hPercent)}%</span>
            </div>
          </div>
        </div>
      </div>

      {/* Добавление алерта */}
      <div>
        <h3 className="text-xl font-normal text-black mb-3">Добавить алерт</h3>
        <div className="grid grid-cols-2 gap-3">
          <div className="p-3 rounded-2xl border border-gray-200 shadow-xs flex flex-col justify-between">
            <label className="text-xs text-gray-700 mb-2">
              Достижение стоимости
            </label>
            <input
              type="number"
              value={targetPriceInput}
              onChange={(e) => setTargetPriceInput(e.target.value)}
              className={`w-full px-2 py-1 text-sm border rounded-md mb-2 focus:outline-none ${
                inputError ? 'border-red-500' : 'border-gray-300'
              }`}
            />
            <button
              onClick={handlePriceSubmit}
              className="self-end px-4 py-1 bg-blue-500 text-white text-xs rounded-lg cursor-pointer"
            >
              ОК
            </button>
          </div>

          <div className="p-3 rounded-2xl border border-gray-200 shadow-xs flex flex-col justify-between">
            <label className="text-xs text-gray-700 mb-2">
              Изменение на (%)
            </label>
            <input
              type="number"
              value={targetPercentInput}
              onChange={(e) => setTargetPercentInput(e.target.value)}
              className={`w-full px-2 py-1 text-sm border rounded-md mb-2 focus:outline-none ${
                inputError ? 'border-red-500' : 'border-gray-300'
              }`}
            />
            <button
              onClick={handlePercentSubmit}
              className="self-end px-4 py-1 bg-blue-500 text-white text-xs rounded-lg cursor-pointer"
            >
              ОК
            </button>
          </div>
        </div>
        {inputError && (
          <p className="text-xs text-red-500 mt-1.5">{inputError}</p>
        )}
      </div>

      {/* Список активных алертов */}
      <div>
        <h3 className="text-xl font-normal text-black mb-3">Активные алерты</h3>
        {coin.alerts.length === 0 ? (
          <p className="text-center text-gray-500 text-sm py-6">
            У Вас нет активных алертов
          </p>
        ) : (
          <div className="space-y-3">
            {coin.alerts.map((alert) => (
              <div
                key={alert.id}
                className="flex items-center justify-between px-4 py-3.5 bg-white rounded-2xl shadow-[0_2px_10px_rgba(0,0,0,0.08)] border border-gray-100"
              >
                <span className="text-sm font-normal text-black">
                  Цель: {alert.targetPrice.toLocaleString('ru-RU')}$ (
                  {alert.diffLabel})
                </span>

                <div className="flex items-center gap-3">
                  <button
                    onClick={() => onToggleAlert(coin.id, alert.id)}
                    className="w-12 h-7 bg-gray-200 rounded-full p-0.5 flex items-center transition-all cursor-pointer"
                  >
                    <div
                      className={`w-6 h-6 rounded-full flex items-center justify-center text-white transition-transform ${
                        alert.isActive
                          ? 'bg-blue-500 translate-x-0'
                          : 'bg-red-500 translate-x-5'
                      }`}
                    >
                      {alert.isActive ? (
                        <Bell className="w-3.5 h-3.5" />
                      ) : (
                        <BellOff className="w-3.5 h-3.5" />
                      )}
                    </div>
                  </button>

                  <button
                    onClick={() => onDeleteAlert(coin.id, alert.id)}
                    className="p-1 text-gray-700 hover:text-red-500 cursor-pointer"
                  >
                    <Trash2 className="w-5 h-5" />
                  </button>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}