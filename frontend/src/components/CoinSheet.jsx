import { useState } from 'react';
import { Bell, BellOff, Trash2 } from 'lucide-react';

export default function CoinSheet({
  detail,
  onAddAlert,
  onToggleAlert,
  onDeleteAlert,
}) {
  const [targetPriceInput, setTargetPriceInput] = useState('');
  const [targetPercentInput, setTargetPercentInput] = useState('');
  const [priceDirection, setPriceDirection] = useState('gte'); // 'gte' | 'lte'
  const [inputError, setInputError] = useState('');

  const currentPrice = Number(detail.price || 0);

  const handlePriceChange = (e) => {
    const val = e.target.value;
    setTargetPriceInput(val);
    const num = Number(val.replace(',', '.'));
    if (!isNaN(num) && num > 0 && currentPrice > 0) {
      setPriceDirection(num >= currentPrice ? 'gte' : 'lte');
    }
  };

  const handlePriceSubmit = () => {
    const raw = targetPriceInput.trim().replace(',', '.');
    const val = Number(raw);

    if (!raw || isNaN(val) || val <= 0 || val > 500000) {
      setInputError('Цена должна быть в диапазоне (0; 500 000]');
      return;
    }
    if (val === currentPrice) {
      setInputError('Целевая цена не должна совпадать с текущей');
      return;
    }

    setInputError('');
    const direction = val > currentPrice ? 'gte' : 'lte';
    onAddAlert({
      type: 'price',
      value: String(val),
      direction,
    });
    setTargetPriceInput('');
  };

  const handlePercentSubmit = () => {
    const raw = targetPercentInput.trim().replace(',', '.').replace(/^\+/, '');
    const pct = Number(raw);

    if (!raw || isNaN(pct) || pct === 0 || pct <= -100 || pct > 500000) {
      setInputError('Процент должен быть от -99.9 до 500 000 (кроме 0)');
      return;
    }

    const calculatedTarget = currentPrice * (1 + pct / 100);
    if (calculatedTarget <= 0 || calculatedTarget > 500000) {
      setInputError('Итоговая цель выходит за диапазон (0; 500 000]');
      return;
    }

    setInputError('');
    const direction = pct > 0 ? 'gte' : 'lte';
    onAddAlert({
      type: 'percent',
      value: String(pct),
      direction,
    });
    setTargetPercentInput('');
  };

  // Проверяем, выше ли целевая цена, чем цена в момент создания алерта
  const isTargetAboveReference = (alert) => {
    const target = Number(alert.target_price);
    const ref = Number(alert.reference_price);
    if (!isNaN(target) && !isNaN(ref) && ref > 0) {
      return target >= ref;
    }
    const dir = alert.direction || alert.condition || 'gte';
    return dir === 'gte';
  };

  const formatAlertBadge = (alert, isUp) => {
    const sign = isUp ? '+' : '-';
    if (alert.type === 'percent') {
      const numVal = Math.abs(Number(alert.value));
      return `${sign}${String(numVal).replace('.', ',')}%`;
    }
    const diff = Number(alert.target_price) - Number(alert.reference_price);
    return `${sign}${Math.abs(Math.round(diff))} ${detail.quote_asset}`;
  };

  return (
    <div className="mt-4 space-y-5">
      <div>
        <div className="flex items-baseline justify-between pr-8">
          <h2 className="text-3xl font-normal text-black">
            {detail.base_asset}/{detail.quote_asset}
          </h2>
          <span className="text-xs text-gray-400">{detail.exchange_name}</span>
        </div>
        <div className="flex items-center gap-3 mt-2">
          <span className="text-2xl font-normal text-black">
            {detail.price !== null && detail.price !== undefined
              ? `${Number(detail.price).toLocaleString('ru-RU')} ${detail.quote_asset}`
              : 'Цена недоступна'}
          </span>
        </div>
      </div>

      {/* добавление алерта */}
      <div>
        <h3 className="text-xl font-normal text-black mb-3">Добавить алерт</h3>
        <div className="grid grid-cols-2 gap-3">
          {/* достижение стоимости */}
          <div className="p-3 rounded-2xl border border-gray-200 shadow-xs flex flex-col justify-between">
            <div className="flex items-center justify-between mb-2">
              <label className="text-xs text-gray-700">
                Достижение стоимости
              </label>
            </div>

            <input
              type="number"
              value={targetPriceInput}
              onChange={handlePriceChange}
              placeholder="85000"
              className={`w-full px-2 py-1 text-sm border rounded-md mb-2 focus:outline-none ${
                inputError ? 'border-red-500' : 'border-gray-300'
              }`}
            />

            <div className="flex items-center justify-end gap-1">
              <button
                onClick={handlePriceSubmit}
                className="px-3.5 py-1 bg-blue-500 text-white text-xs rounded-lg cursor-pointer"
              >
                ОК
              </button>
            </div>
          </div>

          {/* изменение на % */}
          <div className="p-3 rounded-2xl border border-gray-200 shadow-xs flex flex-col justify-between">
            <label className="text-xs text-gray-700 mb-2">
              Изменение на (%)
            </label>
            <input
              type="number"
              value={targetPercentInput}
              onChange={(e) => setTargetPercentInput(e.target.value)}
              placeholder="5 или -5"
              className={`w-full px-2 py-1 text-sm border rounded-md mb-2 focus:outline-none ${
                inputError ? 'border-red-500' : 'border-gray-300'
              }`}
            />
            <div className="flex items-center justify-end">
              <button
                onClick={handlePercentSubmit}
                className="px-3.5 py-1 bg-blue-500 text-white text-xs rounded-lg cursor-pointer"
              >
                ОК
              </button>
            </div>
          </div>
        </div>
        {inputError && (
          <p className="text-xs text-red-500 mt-1.5">{inputError}</p>
        )}
      </div>

      {/* алерты */}
      <div>
        <h3 className="text-xl font-normal text-black mb-3">Активные алерты</h3>
        {!detail.alerts || detail.alerts.length === 0 ? (
          <p className="text-center text-gray-500 text-sm py-6">
            У Вас нет активных алертов
          </p>
        ) : (
          <div className="space-y-3">
            {detail.alerts.map((alert) => {
              const isUp = isTargetAboveReference(alert);

              return (
                <div
                  key={alert.alert_id}
                  className="flex items-center justify-between px-4 py-3.5 bg-white rounded-2xl shadow-[0_2px_10px_rgba(0,0,0,0.08)] border border-gray-100"
                >
                  <div className="flex items-center gap-2.5">
                    <div className="flex flex-col">
                      <span className="text-sm font-normal text-black">
                        Цель {isUp ? '≥' : '≤'}{' '}
                        {Number(alert.target_price).toLocaleString('ru-RU')}{' '}
                        {detail.quote_asset}{' '}
                        <span
                          className={`font-medium ${
                            isUp ? 'text-green-600' : 'text-red-500'
                          }`}
                        >
                          ({formatAlertBadge(alert, isUp)})
                        </span>
                      </span>
                      
                    </div>
                  </div>

                  <div className="flex items-center gap-3">
                    <button
                      onClick={() =>
                        onToggleAlert(alert.alert_id, !alert.is_active)
                      }
                      className="w-12 h-7 bg-gray-200 rounded-full p-0.5 flex items-center transition-all cursor-pointer"
                    >
                      <div
                        className={`w-6 h-6 rounded-full flex items-center justify-center text-white transition-transform ${
                          alert.is_active
                            ? 'bg-blue-500 translate-x-0'
                            : 'bg-red-500 translate-x-5'
                        }`}
                      >
                        {alert.is_active ? (
                          <Bell className="w-3.5 h-3.5" />
                        ) : (
                          <BellOff className="w-3.5 h-3.5" />
                        )}
                      </div>
                    </button>

                    <button
                      onClick={() => onDeleteAlert(alert.alert_id)}
                      className="p-1 text-gray-700 hover:text-red-500 cursor-pointer"
                    >
                      <Trash2 className="w-5 h-5" />
                    </button>
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
}