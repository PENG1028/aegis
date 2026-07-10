// ─── TLS Settings → redirects to certificate management ───
import { useEffect } from 'react';
import { useNavigate } from 'react-router-dom';

export default function TlsSettings() {
  const nav = useNavigate();
  useEffect(() => { nav('/access/certificates', { replace: true }); }, [nav]);
  return null;
}
