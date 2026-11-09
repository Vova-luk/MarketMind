import { ArrowUp, ArrowDown } from 'lucide-react';

export default function CoinCard({ coin, onClick }) {
  const isPositive = coin.change24hDollar >= 0;

  return (
    <div
      onClick={() => onClick(coin.id)}
      className="flex items-center justify-between px-4 py-3.5 min-h-[64px] bg-white rounded-2xl shadow-[0_2px_10px_rgba(0,0,0,0.08)] border border-gray-100 active:scale-[0.99] transition-transform cursor-pointer"
    >
      <span className="text-base font-normal text-black">
        {coin.name} ({coin.ticker})
      </span>

      <div className="flex items-center gap-3">
        <span className="text-base font-medium text-black">
          {coin.price.toLocaleString('ru-RU')}$
        </span>

        <div
          className={`flex items-center text-xs italic leading-tight ${
            isPositive ? 'text-green-600' : 'text-red-500'
          }`}
        >
          {isPositive ? (
            <ArrowUp className="w-4 h-4 mr-0.5 stroke-[2.5]" />
          ) : (
            <ArrowDown className="w-4 h-4 mr-0.5 stroke-[2.5]" />
          )}
          <div className="flex flex-col">
            <span>{Math.abs(coin.change24hDollar)}$</span>
            <span>{Math.abs(coin.change24hPercent)}%</span>
          </div>
        </div>
      </div>
    </div>
  );
}