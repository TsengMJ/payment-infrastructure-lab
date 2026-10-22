export class ApiNotReadyError extends Error {
  readonly path: string;

  constructor(path: string) {
    super(`介面尚未接上：${path}`);
    this.name = "ApiNotReadyError";
    this.path = path;
  }
}

export class CheckoutError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "CheckoutError";
  }
}
