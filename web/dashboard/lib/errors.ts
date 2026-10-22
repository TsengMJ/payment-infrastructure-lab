export class ApiNotReadyError extends Error {
  readonly path: string;

  constructor(path: string) {
    super(`介面尚未接上：${path}`);
    this.name = "ApiNotReadyError";
    this.path = path;
  }
}

export class AuthError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "AuthError";
  }
}

export class PermissionError extends Error {
  constructor(message = "沒有權限") {
    super(message);
    this.name = "PermissionError";
  }
}

export class NotFoundError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "NotFoundError";
  }
}
