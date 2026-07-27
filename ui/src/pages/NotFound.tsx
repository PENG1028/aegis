import { Link } from 'react-router-dom';

export default function NotFoundPage() {
  return (
    <div className="flex flex-col items-center justify-center py-20">
      <h2 className="text-2xl font-bold text-a-muted mb-2">404</h2>
      <p className="text-sm text-a-muted mb-6">页面不存在</p>
      <Link to="/" className="inline-flex items-center text-xs px-3 py-1.5 rounded-a-md bg-a-accent text-white no-underline hover:opacity-90">返回总览</Link>
    </div>
  );
}
