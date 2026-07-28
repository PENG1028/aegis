export interface CertificateMetadata {
  domains: string;
  not_before?: string;
  not_after: string;
}

export function parseCertificateDomains(cert: Pick<CertificateMetadata, 'domains'>): string[] {
  try {
    const value = JSON.parse(cert.domains);
    return Array.isArray(value) ? value.map(String) : [String(value)];
  } catch {
    return [cert.domains];
  }
}

export function certificateCoversDomain(cert: Pick<CertificateMetadata, 'domains'>, domain: string): boolean {
  const normalized = domain.trim().toLowerCase().replace(/\.$/, '');
  return normalized !== '' && parseCertificateDomains(cert).some((name) => {
    const pattern = name.trim().toLowerCase().replace(/\.$/, '');
    if (!pattern.startsWith('*.')) return pattern === normalized;
    const suffix = pattern.slice(2);
    return normalized.endsWith(`.${suffix}`)
      && normalized.split('.').length === suffix.split('.').length + 1;
  });
}

export function certificateIsCurrentlyValid(cert: CertificateMetadata, now = Date.now()): boolean {
  const notBefore = cert.not_before ? new Date(cert.not_before).getTime() : Number.NaN;
  const notAfter = new Date(cert.not_after).getTime();
  return Number.isFinite(notBefore) && Number.isFinite(notAfter) && notBefore <= now && now < notAfter;
}
