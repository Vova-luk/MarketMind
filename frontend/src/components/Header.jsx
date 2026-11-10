export default function Header({ userName }) {
  return (
    <header className="flex items-center justify-between px-4 py-3 border-b border-gray-200 bg-white sticky top-0 z-10">
      <div className="flex items-center gap-2">
        <span className="font-bold text-xl italic tracking-tight text-black">
          <span className="font-bold">Market</span>Mind
        </span>
      </div>
      <div className="text-right text-xs italic text-gray-500 leading-tight">
        Добрый день,
        <br />
        <span className="font-medium text-gray-600">{userName}</span>
      </div>
    </header>
  );
}