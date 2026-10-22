const EXPONENTS: Record<string, number> = {
  TWD: 2,
  USD: 2,
  JPY: 0,
};

export function formatMoney(amount: number, currency: string): string {
  const exponent = EXPONENTS[currency] ?? 2;
  const negative = amount < 0;
  const minor = Math.abs(amount);
  const factor = 10 ** exponent;
  const whole = Math.trunc(minor / factor);
  const fraction = minor % factor;
  const digits = new Intl.NumberFormat("zh-Hant").format(whole);
  const suffix = exponent === 0 ? "" : `.${String(fraction).padStart(exponent, "0")}`;
  return `${negative ? "-" : ""}${currency} ${digits}${suffix}`;
}
