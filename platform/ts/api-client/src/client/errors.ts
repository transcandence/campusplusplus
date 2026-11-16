export class ApiError extends Error {
  constructor(
    public status: number,
    public data: unknown,
  ) {
    super(`API request failed with status ${status}`);
    this.name = "ApiError";
  }
}