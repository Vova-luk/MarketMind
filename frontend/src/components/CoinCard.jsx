import { Bookmark } from 'lucide-react';

export default function CoinCard({ subscription, onClick, onRequestDelete }) {
  const { subscription_id, base_asset, quote_asset, exchange_name, price } =
    subscription;

  return (
    <div
      onClick={() => onClick(subscription_id)}
      className="flex w-full items-center justify-between gap-3 px-4 py-3.5 min-h-[64px] bg-white rounded-2xl shadow-[0_2px_10px_rgba(0,0,0,0.08)] border border-gray-100 active:scale-[0.99] transition-transform cursor-pointer text-left"
    >
      <span className="flex flex-col">
        <span className="text-base font-normal text-black">
          {base_asset}/{quote_asset}
        </span>
        <span className="text-xs text-gray-500">{exchange_name}</span>
      </span>

      <div className="flex items-center gap-3">
        <span className="text-right text-base font-medium text-black">
          {price === null || price === undefined
            ? 'Цена недоступна'
            : `${String(price).replace('.', ',')} ${quote_asset}`}
        </span>

        <button
          type="button"
          onClick={(e) => {
            e.stopPropagation();
            onRequestDelete(subscription);
          }}
          className="p-1 cursor-pointer active:scale-90 transition-transform"
          title="Удалить из отслеживаемого"
        >
          <Bookmark className="w-6 h-6 fill-orange-500 text-orange-500" />
        </button>
      </div>
    </div>
  );
}