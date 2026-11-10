export default function CoinCard({ subscription, onClick }) {
  const { subscription_id, base_asset, quote_asset, exchange_name, price } = subscription;

  return (
    <button
      type="button"
      onClick={() => onClick(subscription_id)}
      className="flex w-full items-center justify-between gap-3 px-4 py-3.5 min-h-[64px] bg-white rounded-2xl shadow-[0_2px_10px_rgba(0,0,0,0.08)] border border-gray-100 active:scale-[0.99] transition-transform cursor-pointer text-left"
    >
      <span className="flex flex-col">
        <span className="text-base font-normal text-black">
          {base_asset}/{quote_asset}
        </span>
        <span className="text-xs text-gray-500">{exchange_name}</span>
      </span>
      <span className="text-right text-base font-medium text-black">
        {price === null ? 'Цена недоступна' : `${price.replace('.', ',')} ${quote_asset}`}
      </span>
    </button>
  );
}