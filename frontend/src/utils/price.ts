export function formatPrice(value: number, currency = 'USD') {
  return new Intl.NumberFormat('en-US', {
    style: 'currency',
    currency,
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }).format(value);
}

export function formatPriceDelta(value: number, currency = 'USD') {
  const sign = value > 0 ? '+' : '';
  return `${sign}${formatPrice(value, currency)}`;
}